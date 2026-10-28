import worldDefault from './fixtures/world-default.json';
import worldTest from './fixtures/world-test.json';
import worldTestBot from './fixtures/world-test-bot.json';
import { toGameState } from '../src/net/toGameState';
import type { FarmTarget } from '../src/engine/farm';
import { HUMAN_ID, MS_PER_HOUR, MS_PER_MINUTE, emptyRecruitQueues, emptyUnits, type Command, type GameState, type Village } from '../src/engine/types';
import type { CommandView, MapTile, PlayerState } from '../src/net/types';

/**
 * The UI tests run on worlds the server built (`npm run gen` writes `tests/fixtures/world-*.json` from the Go
 * server): what GET …/state, …/map and …/farm-targets return for the human, plus for `test-bot` a village of
 * player `bot_test` on the nearest free field.
 */
export type WorldKind = 'default' | 'test' | 'test-bot';
export type WorldFixture = { state: PlayerState; map: MapTile[]; farmTargets: FarmTarget[]; botVillage?: Village };

const FIXTURES: Record<WorldKind, unknown> = { default: worldDefault, test: worldTest, 'test-bot': worldTestBot };
const fixtureOfGame = new WeakMap<GameState, WorldFixture>();

export function loadFixture(kind: WorldKind): WorldFixture {
  return structuredClone(FIXTURES[kind]) as WorldFixture;
}

/** The fixture a game was loaded from (the server stub answers the map and farm targets from it). */
export function fixtureOf(g: GameState): WorldFixture | undefined {
  return fixtureOfGame.get(g);
}

function gameOf(kind: WorldKind): GameState {
  const f = loadFixture(kind);
  const g = toGameState(f.state, [], []);
  fixtureOfGame.set(g, f);
  return g;
}

/** World speed 1×, no bots and an empty starting village; `test-bot` adds the bot village next door. */
export function newTestGame(kind: 'test' | 'test-bot' = 'test'): GameState {
  return gameOf(kind);
}

/** The default world settings: 100 bots, the starting guard. */
export function newDefaultGame(): GameState {
  return gameOf('default');
}

export function activeVillage(g: GameState): Village {
  const v = g.villages[g.activeVillageId];
  if (!v) throw new Error('no active village');
  return v;
}

function registerBot(g: GameState): void {
  g.players.bot_test ??= {
    id: 'bot_test', name: 'Test bot', isHuman: false, color: '#5a7fbf', villageIds: [], coins: 0, noblesTrained: 0,
    buildQueueSlots: 2, protectedUntil: 0, joinAt: 0, status: 'active', points: 0, history: { hourly: [], daily: [] },
  };
}

function freeFieldNear(g: GameState, home: Village): [number, number] {
  const taken = new Set([...Object.values(g.villages), ...(fixtureOf(g)?.map ?? [])].map((v) => `${v.x}|${v.y}`));
  for (let r = 1; r < 20; r++) {
    for (let dx = -r; dx <= r; dx++) for (let dy = -r; dy <= r; dy++) {
      if (Math.max(Math.abs(dx), Math.abs(dy)) === r && !taken.has(`${home.x + dx}|${home.y + dy}`)) return [home.x + dx, home.y + dy];
    }
  }
  throw new Error('no free field near the home village');
}

/**
 * A village of `bot_test` in the game. The first one is the server's (world `test-bot`); further ones, or one at a
 * given field, are copies of it on the nearest free field (or at x|y).
 */
export function makeBotVillage(g: GameState, x?: number, y?: number): Village {
  const template = fixtureOf(g)?.botVillage ?? loadFixture('test-bot').botVillage!;
  registerBot(g);
  const first = !g.villages[template.id] && x === undefined;
  const [vx, vy] = first ? [template.x, template.y] : x !== undefined && y !== undefined ? [x, y] : freeFieldNear(g, activeVillage(g));
  const v: Village = { ...structuredClone(template), id: `v:${vx}:${vy}`, x: vx, y: vy, name: 'Bot village' };
  g.villages[v.id] = v;
  const ids = g.players.bot_test!.villageIds;
  if (!ids.includes(v.id)) ids.push(v.id);
  return v;
}

/** The test world's bot village (registered in the game on first use). */
export function botVillage(g: GameState): Village {
  const id = (fixtureOf(g)?.botVillage ?? loadFixture('test-bot').botVillage!).id;
  return g.villages[id] ?? makeBotVillage(g);
}

/** The barbarian village nearest to the home village on the fixture's map, added to the game. */
export function barbarianVillage(g: GameState): Village {
  const home = activeVillage(g);
  const d = (t: { x: number; y: number }) => Math.hypot(t.x - home.x, t.y - home.y);
  const tile = (fixtureOf(g)?.map ?? []).filter((t) => t.ownerId === null).sort((a, b) => d(a) - d(b))[0];
  if (!tile) throw new Error('no barbarian village on the fixture map');
  const template = loadFixture('test-bot').botVillage!;
  const v: Village = { ...template, id: tile.id, x: tile.x, y: tile.y, name: tile.name, points: tile.points, ownerId: null };
  g.villages[v.id] = v;
  return v;
}

/** Another village of the human at x|y: the home village's buildings, no units and no queues. */
export function addOwnVillage(g: GameState, x: number, y: number, name: string): Village {
  const v: Village = { ...structuredClone(activeVillage(g)), id: `v:${x}:${y}`, x, y, name, units: emptyUnits(), buildQueue: [],
    recruitQueues: emptyRecruitQueues(), researchQueue: [], supportFrom: {} };
  g.villages[v.id] = v;
  g.players[HUMAN_ID]!.villageIds.push(v.id);
  return v;
}

let commandSeq = 0;

/** A command in the game as the server would list it: departed a minute ago, arriving in an hour unless given. */
export function addCommand(g: GameState, c: Partial<Command> & Pick<Command, 'type' | 'originId' | 'targetId'>): Command {
  const cmd: Command = { id: `c${++commandSeq}`, units: {}, departedAt: g.now - MS_PER_MINUTE, arrivesAt: g.now + MS_PER_HOUR, ...c };
  g.commands.push(cmd);
  g.commandsRev++;
  return cmd;
}

/**
 * The server's view of a game for its human: what `GET …/state` returns. The human's id becomes `me`
 * (as on the server, where humans are `h<account>`), others keep theirs.
 */
export function stateFixture(g: GameState, me = HUMAN_ID, worldId = 1): PlayerState {
  const id = (p: string) => (p === HUMAN_ID ? me : p);
  const ownIds = new Set(Object.values(g.villages).filter((v) => v.ownerId === HUMAN_ID).map((v) => v.id));
  const villages: PlayerState['villages'] = {};
  for (const vid of ownIds) villages[vid] = { ...structuredClone(g.villages[vid]!), ownerId: me };
  const intel: PlayerState['intel'] = {};
  for (const v of Object.values(g.villages)) {
    if (ownIds.has(v.id) || (!v.intel && !v.farmTargetFlags && v.lastFarmedAt === undefined)) continue;
    intel[v.id] = structuredClone({ intel: v.intel, farmTargetFlags: v.farmTargetFlags, lastFarmedAt: v.lastFarmedAt });
  }
  const players: PlayerState['players'] = {};
  for (const p of Object.values(g.players)) {
    players[id(p.id)] = { id: id(p.id), name: p.name, color: p.color, status: p.status, points: p.points, villageIds: [...p.villageIds], isBot: !p.isHuman };
  }
  const { brain: _brain, isHuman: _isHuman, ...self } = structuredClone(g.players[HUMAN_ID]!);
  const commands: CommandView[] = [];
  for (const c of g.commands) {
    const home = c.type === 'return' || (c.type === 'trade' && c.returning) ? c.targetId : c.originId;
    if (ownIds.has(home)) commands.push(structuredClone(c));
    else if (ownIds.has(c.targetId)) {
      const { units: _units, ...rest } = structuredClone(c);
      commands.push(rest);
    }
  }
  return {
    me, worldId, worldName: 'Café Test World', now: g.now, serverTime: g.lastRealTime, clockOriginLocalMs: g.clock.originLocalMs,
    paused: !!g.config.paused, config: structuredClone(g.config), villages, player: { ...self, id: me }, players, commands, intel,
    scheduled: g.scheduled.map((s) => ({ ...structuredClone(s), ownerId: id(s.ownerId) })),
    human: {
      stats: structuredClone(g.stats), quests: structuredClone(g.quests), farmAssistant: structuredClone(g.farmAssistant),
      trader: structuredClone(g.trader), villageGroups: structuredClone(g.villageGroups),
      buildTemplates: structuredClone(g.buildTemplates), aggressorUntil: g.humanAggressorUntil,
    },
    unreadReports: g.reports.filter((r) => !r.read).length,
  };
}
