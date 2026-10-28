import { coordsOfVillageId, MS_PER_MINUTE, type Command, type GameState } from './types';
import { distance } from './world';

/** Label of an incoming command — the slowest unit's class read from the march time. */
export type IncomingLabel = 'scout' | 'cavalry' | 'heavyCav' | 'infantry' | 'sword' | 'siege' | 'noble';

/** Base min/field of each class. */
const LABEL_SPEED: [IncomingLabel, number][] = [
  ['scout', 9], ['cavalry', 10], ['heavyCav', 11], ['infantry', 18], ['sword', 22], ['siege', 30], ['noble', 35],
];

export function labelIncoming(state: GameState, cmd: Command): IncomingLabel {
  const origin = state.villages[cmd.originId] ?? coordsOfVillageId(cmd.originId);
  const target = state.villages[cmd.targetId] ?? coordsOfVillageId(cmd.targetId);
  if (!origin || !target) return 'infantry';
  const d = distance(origin.x, origin.y, target.x, target.y);
  if (d <= 0) return 'infantry';
  const minPerTile = ((cmd.arrivesAt - cmd.departedAt) / MS_PER_MINUTE) * state.config.unitSpeed / d;
  let best: IncomingLabel = 'infantry', bestDiff = Infinity;
  for (const [label, speed] of LABEL_SPEED) {
    const diff = Math.abs(speed - minPerTile);
    if (diff < bestDiff) { bestDiff = diff; best = label; }
  }
  return best;
}

/** Hetman trains: attacks labelled "noble?" on a village, landing at most 1 s apart. */
export function incomingTrains(state: GameState, villageId: string): { commands: Command[]; firstAt: number; lastAt: number }[] {
  const nobles = state.commands
    .filter((c) => c.type === 'attack' && c.targetId === villageId && labelIncoming(state, c) === 'noble')
    .sort((a, b) => a.arrivesAt - b.arrivesAt);
  const out: { commands: Command[]; firstAt: number; lastAt: number }[] = [];
  for (const c of nobles) {
    const last = out[out.length - 1];
    if (last && c.arrivesAt - last.lastAt <= 1000) { last.commands.push(c); last.lastAt = c.arrivesAt; }
    else out.push({ commands: [c], firstAt: c.arrivesAt, lastAt: c.arrivesAt });
  }
  return out.filter((t) => t.commands.length >= 2);
}
