import { HUMAN_ID, type GameState, type NewsItem, type Player, type Report, type WorldConfig } from '../engine/types';
import { toClientId, toClientRef } from './ids';
import { ONLINE_TERRAIN_SEED } from '../shared/terrain/seed';
import type { NewsEntry, PlayerState } from './types';

let commandsRev = 0;

function publicAsPlayer(p: PlayerState['players'][string], id: string): Player {
  return {
    id, name: p.name, isHuman: !p.isBot, color: p.color, villageIds: [...p.villageIds], coins: 0, noblesTrained: 0,
    buildQueueSlots: 2, protectedUntil: 0, joinAt: 0, status: p.status, points: p.points, history: { hourly: [], daily: [] },
  };
}

/**
 * The engine's GameState as far as this player may know it, so the existing selectors and display helpers
 * keep working: own villages, the public face of every player (the own one in full), own commands and the
 * ones coming in (those without troops), reports and news oldest first, as the engine kept them.
 */
export function toGameState(s: PlayerState, reports: Report[], news: (NewsItem | NewsEntry)[]): GameState {
  const me = s.me;
  const villages: GameState['villages'] = {};
  for (const [id, v] of Object.entries(s.villages)) villages[id] = { ...v, ownerId: toClientRef(v.ownerId, me) };
  const players: GameState['players'] = {};
  for (const p of Object.values(s.players)) {
    const id = toClientId(p.id, me);
    players[id] = publicAsPlayer(p, id);
  }
  players[HUMAN_ID] = { ...players[HUMAN_ID], ...s.player, id: HUMAN_ID, isHuman: true } as Player;
  const stance = (s.config.botStance as string) === 'allAgainstHumans' ? 'allAgainstMe' : s.config.botStance;
  const config = { ...s.config, botStance: stance, paused: s.paused } as WorldConfig;
  const own = Object.keys(villages).sort((a, b) => villages[a]!.name.localeCompare(villages[b]!.name, 'pl') || a.localeCompare(b));
  return {
    worldSeed: ONLINE_TERRAIN_SEED,
    now: s.now,
    lastRealTime: s.serverTime,
    config,
    villages,
    players,
    activeVillageId: own[0] ?? '',
    commands: s.commands.map((c) => ({ ...c, units: c.units ?? {} })),
    commandsRev: ++commandsRev,
    reports: [...reports].reverse(),
    quests: s.human.quests,
    farmAssistant: s.human.farmAssistant,
    trader: s.human.trader,
    news: [...news].reverse().map(({ for: _for, ...n }: NewsEntry) => n as NewsItem),
    villageGroups: s.human.villageGroups,
    buildTemplates: s.human.buildTemplates,
    stats: s.human.stats,
    clock: { originLocalMs: s.clockOriginLocalMs },
    scheduled: s.scheduled.map((x) => ({ ...x, ownerId: toClientId(x.ownerId, me) })),
    humanAggressorUntil: s.human.aggressorUntil,
  };
}
