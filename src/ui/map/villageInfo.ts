import { hasKey, t as tr } from '../../i18n/i18n';
import { villageName } from '../../i18n/names';
import { villageLabelOf } from '../villageName';
import { buildingName } from '../../i18n/names';
/*
 * Data for the village menu on the map — a pure function of state (no React), seen from the player's side:
 * own village = economy and threats, foreign = what the player actually knows (intel, reports, movements).
 */
import { BUILDINGS } from '../../data/buildings.data';
import { loyaltyAt } from '../../engine/conquest';
import { reportIcon } from '../../engine/reports';
import { villageFarmCapacity, villagePopulationUsed, villageStorageCapacity } from '../../engine/resources';
import { UNIT_IDS, type BuildingId, type GameState, type Report, type ReportType, type UnitId, type Village, type VillageBonus } from '../../engine/types';
import type { Intel } from '../../net/types';
import { distance } from '../../engine/world';
import { tileAt } from './tiles';
import { isHumanVillage, humanVillageIds } from '../../engine/players';

export interface ReportBrief {
  id: string;
  icon: string;
  type: ReportType;
  outcome: 'attackerWon' | 'defenderWon';
  time: number;
  haul: number | null;
  haulCapacity: number | null;
  read: boolean;
  /** Report from our own action (we attack/scout); false = an AI attack on us. */
  ours: boolean;
}

export interface CommandBrief {
  id: string;
  kind: 'attack' | 'support' | 'return' | 'trade';
  arrivesAt: number;
  units: number;
}

export interface OwnVillageInfo {
  kind: 'own';
  id: string;
  name: string;
  x: number;
  y: number;
  points: number;
  active: boolean;
  resources: { wood: number; clay: number; iron: number };
  storage: number;
  popUsed: number;
  popMax: number;
  unitsHome: Partial<Record<UnitId, number>>;
  unitsTotal: number;
  build: { building: BuildingId; name: string; level: number; completesAt: number } | null;
  queueLength: number;
  incoming: { count: number; firstAt: number | null };
  outgoing: number;
  reportCount: number;
}

export interface ForeignVillageInfo {
  kind: 'foreign';
  id: string;
  name: string;
  /** Owning player; `null` = barbarians. */
  ownerId: string | null;
  ownerName: string | null;
  ownerIsBot: boolean;
  /** The owner's public points (hourly ranking). */
  ownerPoints: number | null;
  x: number;
  y: number;
  points: number;
  bonus?: VillageBonus;
  materialized: boolean;
  distance: number;
  intel: {
    at: number;
    units: Partial<Record<UnitId, number>>;
    wall: number;
    loyalty: number;
    lastHaul: number | null;
    lastHaulCapacity: number | null;
    lastLosses: number | null;
  } | null;
  lastAttackAt: number | null;
  reports: ReportBrief[];
  reportCount: number;
  /** Our commands on their way to this village. */
  ours: CommandBrief[];
  /** The next attack from this village on us. */
  threat: { arrivesAt: number; targetId: string; targetName: string; units: number } | null;
}

export type VillageInfo = OwnVillageInfo | ForeignVillageInfo;

export const REPORT_PREVIEW_COUNT = 3;

/** What a bonus village gives, in the reader's words. */
export const bonusLabel = (kind: string): string => (hasKey(`bonus.${kind}`) ? tr(`bonus.${kind}`) : kind);

function countUnits(units: Partial<Record<UnitId, number>>): number {
  let n = 0;
  for (const u of UNIT_IDS) n += units[u] ?? 0;
  return n;
}

/** Reports involving the village (as target or origin), newest first. */
export function villageReports(state: GameState, villageId: string): Report[] {
  const out: Report[] = [];
  for (let i = state.reports.length - 1; i >= 0; i--) {
    const r = state.reports[i]!;
    if (r.targetId === villageId || r.originId === villageId) out.push(r);
  }
  return out;
}

/** `intel`: what the player knows of other villages, by id (from the server); villages kept in `state` may carry their own. */
export function villageInfo(state: GameState, x: number, y: number, fromId: string = state.activeVillageId, intel: Record<string, Intel> = {}): VillageInfo | null {
  const t = tileAt(state, x, y);
  if (t.kind !== 'village') return null;
  const id = t.id;
  const v = state.villages[id];
  const player = new Set(humanVillageIds(state));
  const reports = villageReports(state, id);

  if (v && isHumanVillage(v)) {
    let incoming = 0, firstAt: number | null = null, outgoing = 0;
    for (const c of state.commands) {
      if (c.targetId === id && c.type === 'attack' && !player.has(c.originId)) {
        incoming++;
        if (firstAt === null || c.arrivesAt < firstAt) firstAt = c.arrivesAt;
      }
      if (c.originId === id && (c.type === 'attack' || c.type === 'support')) outgoing++;
    }
    const job = v.buildQueue[0];
    return {
      kind: 'own', id, name: villageLabelOf(state, v), x, y, points: v.points, active: id === state.activeVillageId,
      resources: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) },
      storage: villageStorageCapacity(v),
      popUsed: villagePopulationUsed(v, state), popMax: villageFarmCapacity(v),
      unitsHome: { ...v.units }, unitsTotal: countUnits(v.units),
      build: job ? { building: job.building, name: buildingName(job.building), level: job.targetLevel, completesAt: job.completesAt } : null,
      queueLength: v.buildQueue.length,
      incoming: { count: incoming, firstAt },
      outgoing,
      reportCount: reports.length,
    };
  }

  const from = state.villages[fromId];
  const d = from ? distance(from.x, from.y, x, y) : 0;
  const ours: CommandBrief[] = [];
  let threat: ForeignVillageInfo['threat'] = null;
  for (const c of state.commands) {
    if (c.targetId === id && player.has(c.originId) && (c.type === 'attack' || c.type === 'support' || c.type === 'trade')) {
      ours.push({ id: c.id, kind: c.type, arrivesAt: c.arrivesAt, units: countUnits(c.units) });
    }
    if (c.originId === id && c.type === 'attack' && player.has(c.targetId) && (!threat || c.arrivesAt < threat.arrivesAt)) {
      threat = { arrivesAt: c.arrivesAt, targetId: c.targetId, targetName: state.villages[c.targetId]?.name ?? c.targetId, units: countUnits(c.units) };
    }
  }
  ours.sort((a, b) => a.arrivesAt - b.arrivesAt);

  const known = intel[id];
  const seen = known?.intel ?? v?.intel;
  return {
    kind: 'foreign', id, name: villageName(t, t.ownerName), ownerId: t.ownerId, ownerIsBot: !!t.ownerIsBot, ownerName: t.ownerName ? `${t.ownerName}${t.ownerStatus === 'quit' || t.ownerStatus === 'eliminated' ? tr('village.inactive') : ''}` : null,
    ownerPoints: t.ownerId ? state.players[t.ownerId]?.points ?? null : null, x, y, points: t.points, bonus: t.bonus, materialized: t.materialized, distance: d,
    intel: seen ? {
      at: seen.at, units: { ...seen.units }, wall: seen.wall,
      loyalty: Math.round(loyaltyAt({ ...v, loyalty: seen.loyalty, loyaltyUpdatedAt: seen.at } as Village, state.now, state.config.worldSpeed)),
      lastHaul: seen.lastHaul ? seen.lastHaul.wood + seen.lastHaul.clay + seen.lastHaul.iron : null,
      lastHaulCapacity: seen.lastHaulCapacity ?? null,
      lastLosses: seen.lastLosses ?? null,
    } : null,
    lastAttackAt: known?.lastFarmedAt ?? v?.lastFarmedAt ?? null,
    reports: reports.slice(0, REPORT_PREVIEW_COUNT).map((r) => ({
      id: r.id, icon: reportIcon(r), type: r.type, outcome: r.outcome, time: r.time,
      haul: r.haul ? r.haul.wood + r.haul.clay + r.haul.iron : null, haulCapacity: r.haulCapacity ?? null, read: r.read,
      ours: player.has(r.originId),
    })),
    reportCount: reports.length,
    ours,
    threat,
  };
}
