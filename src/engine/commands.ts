import { UNITS } from '../data/units.data';
import { COMMAND_BALANCE } from '../data/world.config';
import { CONQUEST_BALANCE } from '../data/conquest.data';
import { MS_PER_MINUTE, UNIT_IDS, villageIdFor, type Command, type GameState, type UnitId, type Village } from './types';
import { distance } from './world';
import { humanVillageIds, isProtected } from './players';

export type UnitCounts = Partial<Record<UnitId, number>>;

function totalUnits(units: UnitCounts): number {
  let n = 0;
  for (const u of UNIT_IDS) n += units[u] ?? 0;
  return n;
}

/** The slowest unit in a command (min/field). */
export function slowestSpeed(units: UnitCounts): number {
  let s = 0;
  for (const u of UNIT_IDS) if ((units[u] ?? 0) > 0) s = Math.max(s, UNITS[u].speed);
  return s;
}

/** March time: distance × min/field of the slowest unit, scaled only by unit speed (not world speed). */
export function travelTimeMs(dist: number, speedMinPerTile: number, unitSpeed: number): number {
  return Math.round((dist * speedMinPerTile * MS_PER_MINUTE) / unitSpeed);
}

export function travelTimeForUnits(state: GameState, origin: Village, tx: number, ty: number, units: UnitCounts): number {
  const d = distance(origin.x, origin.y, tx, ty);
  return travelTimeMs(d, slowestSpeed(units), state.config.unitSpeed);
}

/** A command's home (the village the troops belong to). */
export function commandHome(cmd: Command): string {
  return cmd.type === 'return' ? cmd.targetId : cmd.originId;
}

type AwayByHome = Map<string, Partial<Record<UnitId, number>>>;

/**
 * "Troops on the move by home village" index, derived from `state.commands` and rebuilt when its stamp (command
 * count, revision, first and last) doesn't match.
 */
interface CommandIndex { stamp: string; byHome: AwayByHome; pos: Map<string, number> }
const awayIndex = new WeakMap<Command[], CommandIndex>();
const indexStamp = (state: GameState) => {
  const c = state.commands;
  return `${c.length}|${state.commandsRev}|${c[0]?.id ?? ''}|${c[c.length - 1]?.id ?? ''}`;
};

function indexCommand(byHome: AwayByHome, cmd: Command, sign: 1 | -1): void {
  const home = commandHome(cmd);
  let e = byHome.get(home);
  if (!e) { e = {}; byHome.set(home, e); }
  for (const [u, n] of Object.entries(cmd.units) as [UnitId, number][]) e[u] = (e[u] ?? 0) + sign * n;
}

/** The last used index and the array it belongs to — each index belongs to one array at a time. */
let lastIndexed: { arr: Command[]; e: CommandIndex } | null = null;

function commandIndex(state: GameState): CommandIndex {
  const stamp = indexStamp(state);
  let e = awayIndex.get(state.commands);
  // On every state change the store (immer) hands the engine a new array with the same contents — the index
  // then moves to the new array instead of being rebuilt every tick. The old one loses it, so two states never share it.
  if (!e && lastIndexed && lastIndexed.e.stamp === stamp) {
    e = lastIndexed.e;
    awayIndex.delete(lastIndexed.arr);
    awayIndex.set(state.commands, e);
  }
  if (!e || e.stamp !== stamp) {
    const byHome: AwayByHome = new Map();
    const pos = new Map<string, number>();
    state.commands.forEach((c, i) => { indexCommand(byHome, c, 1); pos.set(c.id, i); });
    e = { stamp, byHome, pos };
    awayIndex.set(state.commands, e);
  }
  lastIndexed = { arr: state.commands, e };
  return e;
}

function awayByHome(state: GameState): AwayByHome {
  return commandIndex(state).byHome;
}

/** A village's units away from home: on the move + stationed as support elsewhere. */
export function unitsAway(state: GameState, villageId: string): Record<UnitId, number> {
  const out = {} as Record<UnitId, number>;
  for (const u of UNIT_IDS) out[u] = 0;
  const moving = awayByHome(state).get(villageId);
  if (moving) for (const [u, n] of Object.entries(moving) as [UnitId, number][]) out[u] += n;
  const ownerId = state.villages[villageId]?.ownerId;
  for (const id of ownerId ? state.players[ownerId]?.villageIds ?? [] : []) {
    const sup = state.villages[id]?.supportFrom[villageId];
    if (!sup) continue;
    for (const u of UNIT_IDS) out[u] += sup[u] ?? 0;
  }
  return out;
}

export type SendBlocker =
  | { kind: 'noUnits' }
  | { kind: 'notEnough'; unit: UnitId; have: number }
  | { kind: 'noTarget' }
  | { kind: 'self' }
  | { kind: 'noAttackPower' }
  | { kind: 'supportTargetNotOwned' }
  | { kind: 'supportTargetBarbarian' }
  | { kind: 'attackOwnVillage' }
  | { kind: 'nobleUnescorted' }
  | { kind: 'nobleTooFar'; max: number }
  | { kind: 'targetProtected'; until: number }
  | { kind: 'selfProtected' };

export interface SendCheck {
  blockers: SendBlocker[];
  warnings: SendBlocker[];
  distance: number;
  travelMs: number;
  arrivesAt: number;
  isScoutMission: boolean;
}

export function checkSend(
  state: GameState, originId: string, tx: number, ty: number, units: UnitCounts, type: 'attack' | 'support',
): SendCheck {
  const origin = state.villages[originId];
  const blockers: SendBlocker[] = [];
  const warnings: SendBlocker[] = [];
  if (!origin) throw new Error(`no village ${originId}`);
  const n = totalUnits(units);
  if (n <= 0) blockers.push({ kind: 'noUnits' });
  for (const u of UNIT_IDS) {
    const want = units[u] ?? 0;
    if (want < 0 || !Number.isInteger(want)) blockers.push({ kind: 'notEnough', unit: u, have: origin.units[u] });
    else if (want > origin.units[u]) blockers.push({ kind: 'notEnough', unit: u, have: origin.units[u] });
  }
  if (origin.x === tx && origin.y === ty) blockers.push({ kind: 'self' });
  // Only own villages are known for sure; the client checks the target against the loaded map (rallyCheck.ts).
  if (!state.villages[villageIdFor(tx, ty)]) blockers.push({ kind: 'noTarget' });

  const scouts = units.scout ?? 0;
  const isScoutMission = n > 0 && scouts === n;
  if (type === 'attack' && !isScoutMission) {
    let power = 0;
    for (const u of UNIT_IDS) power += (units[u] ?? 0) * UNITS[u].attack;
    if (power <= 0) blockers.push({ kind: 'noAttackPower' });
    if ((units.noble ?? 0) > 0 && n - (units.noble ?? 0) === 0) warnings.push({ kind: 'nobleUnescorted' });
  }
  const target = state.villages[villageIdFor(tx, ty)];
  if (type === 'support') {
    if (!target || target.ownerId === null || target.ownerId !== origin.ownerId) blockers.push({ kind: 'supportTargetNotOwned' });
  }
  if (type === 'attack' && target && target.ownerId !== null && target.ownerId === origin.ownerId && target.id !== origin.id) {
    blockers.push({ kind: 'attackOwnVillage' });
  }
  // Beginner protection: protected players cannot be attacked, and a protected player can only attack barbarians.
  if (type === 'attack' && target?.ownerId && target.ownerId !== origin.ownerId) {
    if (isProtected(state, target.ownerId)) blockers.push({ kind: 'targetProtected', until: state.players[target.ownerId]!.protectedUntil });
    if (origin.ownerId && isProtected(state, origin.ownerId)) blockers.push({ kind: 'selfProtected' });
  }
  const dist = distance(origin.x, origin.y, tx, ty);
  if ((units.noble ?? 0) > 0 && dist > CONQUEST_BALANCE.nobleMaxDistance) {
    blockers.push({ kind: 'nobleTooFar', max: CONQUEST_BALANCE.nobleMaxDistance });
  }
  const travelMs = travelTimeMs(dist, slowestSpeed(units), state.config.unitSpeed);
  return { blockers, warnings, distance: dist, travelMs, arrivesAt: state.now + travelMs, isScoutMission };
}

export function canCancelCommand(state: GameState, cmd: Command): boolean {
  if (cmd.type === 'return' || cmd.type === 'intercept') return false;
  // An army hit in the field may turn back at any time before it arrives.
  if (cmd.fieldHit && (cmd.type === 'attack' || cmd.type === 'support')) return state.now < cmd.arrivesAt;
  const elapsed = state.now - cmd.departedAt;
  const window = cmd.type === 'trade' ? COMMAND_BALANCE.tradeCancelWindowMs : COMMAND_BALANCE.cancelWindowMs;
  return elapsed < Math.min(window, cmd.arrivesAt - cmd.departedAt);
}

/** Commands visible to the player: their own movements and everything heading for their villages (AI movements between barbarians are hidden). */
export function playerCommands(state: GameState): Command[] {
  const mine = new Set(humanVillageIds(state));
  return state.commands.filter((c) => mine.has(commandHome(c)) || mine.has(c.targetId));
}
