import { BONUS_BALANCE } from '../data/buildings.data';
import { RESEARCH_BALANCE, RESEARCH_UNLOCK_COST } from '../data/research.data';
import { UNITS, UNIT_BALANCE } from '../data/units.data';
import { coinsForNextNoble } from './conquest';
import { unitsAway } from './commands';
import { hasResources, villageFarmCapacity, villagePopulationUsed } from './resources';
import {
  type BuildingId, type GameState, type RecruitBuilding,
  type PlayerId, type ResourceSet, type UnitId, type Village, type WorldConfig,
} from './types';
import { modifiersFor } from './modifiers';
import { playerVillages } from './players';

// ---------- Research ----------

/**
 * Whether a unit must be unlocked in the Smithy (yes if its requirements include the Smithy). The others (spear, ram,
 * paladin) get level 1 for free and are upgraded to levels 2–3 in the Smithy like every other unit.
 */
export function unitNeedsResearch(unit: UnitId): boolean {
  return UNITS[unit].requirements.some((r) => r.building === 'smithy');
}

/** Current research level, counting the free unlock. */
export function effectiveResearchLevel(v: Village, unit: UnitId): number {
  const lvl = v.research[unit] ?? 0;
  return unitNeedsResearch(unit) ? lvl : Math.max(1, lvl);
}

export function researchBonus(level: number): number {
  return UNIT_BALANCE.researchBonus[Math.max(0, Math.min(level, RESEARCH_BALANCE.maxLevel))] ?? 0;
}

export function researchCost(unit: UnitId, level: number): ResourceSet {
  const base = RESEARCH_UNLOCK_COST[unit];
  const mult = UNIT_BALANCE.researchCostMultiplier[level] ?? 0;
  return { wood: base.wood * mult, clay: base.clay * mult, iron: base.iron * mult };
}

export function researchTimeMs(unit: UnitId, level: number, worldSpeed: number): number {
  const c = researchCost(unit, level);
  const ms = ((c.wood + c.clay + c.iron) * UNIT_BALANCE.researchTimePerResourceSec * 1000) / worldSpeed;
  return Math.max(RESEARCH_BALANCE.minResearchTimeMs, Math.round(ms));
}

export type ResearchBlocker =
  | { kind: 'maxLevel' }
  | { kind: 'busy' }
  | { kind: 'requirements'; missing: { building: BuildingId; level: number }[] }
  | { kind: 'resources'; missing: ResourceSet };

export interface ResearchCheck {
  currentLevel: number;
  targetLevel: number;
  cost: ResourceSet;
  durationMs: number;
  blockers: ResearchBlocker[];
}

export function checkResearch(state: GameState, v: Village, unit: UnitId): ResearchCheck {
  const current = effectiveResearchLevel(v, unit);
  const queued = v.researchQueue.find((j) => j.unit === unit);
  const targetLevel = (queued ? queued.targetLevel : current) + 1;
  const lvl = Math.min(targetLevel, RESEARCH_BALANCE.maxLevel);
  const cost = researchCost(unit, lvl);
  const blockers: ResearchBlocker[] = [];
  if (targetLevel > RESEARCH_BALANCE.maxLevel) blockers.push({ kind: 'maxLevel' });
  if (v.researchQueue.length > 0) blockers.push({ kind: 'busy' });
  const missing = UNITS[unit].requirements.filter((r) => v.buildings[r.building] < r.level);
  if (v.buildings.smithy < 1) missing.push({ building: 'smithy', level: 1 });
  if (missing.length) blockers.push({ kind: 'requirements', missing });
  if (!hasResources(v.resources, cost)) {
    blockers.push({
      kind: 'resources',
      missing: {
        wood: Math.max(0, cost.wood - v.resources.wood),
        clay: Math.max(0, cost.clay - v.resources.clay),
        iron: Math.max(0, cost.iron - v.resources.iron),
      },
    });
  }
  return { currentLevel: current, targetLevel, cost, durationMs: researchTimeMs(unit, lvl, state.config.worldSpeed), blockers };
}

// ---------- Recruitment ----------

export function recruitBuildingFor(unit: UnitId): RecruitBuilding {
  return UNITS[unit].building;
}

/** Recruit time for one unit; without a village — no bonus and no side multiplier. */
export function recruitTimeMs(unit: UnitId, buildingLevel: number, cfg: WorldConfig, v?: Village): number {
  const def = UNITS[unit];
  const side = v ? modifiersFor(cfg, v).recruitment : 1;
  let ms = (def.baseRecruitSec * Math.pow(UNIT_BALANCE.recruitTimeFactor, Math.max(1, buildingLevel) - 1) * 1000) / (cfg.worldSpeed * side);
  if (v?.bonus?.kind === 'infantry' && def.building === 'barracks') ms *= BONUS_BALANCE.recruitTimeFactor;
  if (v?.bonus?.kind === 'cavalry' && def.building === 'stable') ms *= BONUS_BALANCE.recruitTimeFactor;
  return Math.max(UNIT_BALANCE.minRecruitTimeMs, Math.round(ms));
}

export function unitCost(unit: UnitId, count: number, config?: WorldConfig): ResourceSet {
  const own = config?.nobleCost;
  const c = unit === 'noble' && own && (own.wood > 0 || own.clay > 0 || own.iron > 0) ? own : UNITS[unit].cost;
  return { wood: c.wood * count, clay: c.clay * count, iron: c.iron * count };
}

/** How many of a unit are in the village + in queues (for limits like "1 paladin"). */
export function unitCountIncludingQueued(v: Village, unit: UnitId): number {
  let n = v.units[unit];
  for (const q of Object.values(v.recruitQueues)) for (const j of q) if (j.unit === unit) n += j.total - j.done;
  return n;
}

export type RecruitBlocker =
  | { kind: 'requirements'; missing: { building: BuildingId; level: number }[] }
  | { kind: 'research' }
  | { kind: 'resources'; missing: ResourceSet }
  | { kind: 'population'; needed: number; free: number }
  | { kind: 'limit'; max: number }
  | { kind: 'coins'; have: number; needed: number }
  | { kind: 'count' };

/** Nobles belonging to a village: present + queued + away. */
export function nobleCountOf(v: Village, state?: GameState): number {
  let n = unitCountIncludingQueued(v, 'noble');
  if (state) n += unitsAway(state, v.id).noble;
  return n;
}

/** All of a player's nobles (all their villages: present + queued + on the move). */
export function totalNoblesOwned(state: GameState, playerId: PlayerId): number {
  let n = 0;
  for (const v of playerVillages(state, playerId)) n += nobleCountOf(v, state);
  return n;
}

/** Coins a player needs for the next `count` nobles (each one costs 1 more). */
export function coinsNeededFor(state: GameState, playerId: PlayerId, count: number): number {
  const owned = totalNoblesOwned(state, playerId);
  let total = 0;
  for (let i = 0; i < count; i++) total += coinsForNextNoble(state, playerId, owned + i);
  return total;
}

export function nobleLimit(config: WorldConfig | undefined, v: Village): number {
  if (v.buildings.academy <= 0) return 0;
  return v.buildings.academy + (config?.academyExtraNobles ?? 0);
}

export interface RecruitCheck {
  cost: ResourceSet;
  pop: number;
  perUnitMs: number;
  /** Max units that can be ordered with current resources and population (0 if locked). */
  maxAffordable: number;
  blockers: RecruitBlocker[];
}

export function maxRecruitable(v: Village, unit: UnitId, state?: GameState): number {
  const def = UNITS[unit];
  const cost = unitCost(unit, 1, state?.config);
  const free = villageFarmCapacity(v) - villagePopulationUsed(v, state);
  const byPop = def.pop > 0 ? Math.floor(free / def.pop) : Infinity;
  const byRes = Math.min(
    cost.wood > 0 ? Math.floor(v.resources.wood / cost.wood) : Infinity,
    cost.clay > 0 ? Math.floor(v.resources.clay / cost.clay) : Infinity,
    cost.iron > 0 ? Math.floor(v.resources.iron / cost.iron) : Infinity,
  );
  let n = Math.max(0, Math.min(byPop, byRes));
  if (unit === 'paladin') n = Math.min(n, Math.max(0, 1 - unitCountIncludingQueued(v, 'paladin')));
  if (unit === 'noble') {
    n = Math.min(n, Math.max(0, nobleLimit(state?.config, v) - nobleCountOf(v, state)));
    const owner = state && v.ownerId !== null ? state.players[v.ownerId] : undefined;
    if (state && owner && !state.config.noCoins) {
      let affordable = 0;
      while (affordable < n && coinsNeededFor(state, owner.id, affordable + 1) <= owner.coins) affordable++;
      n = affordable;
    }
  }
  return Number.isFinite(n) ? n : 0;
}

export function checkRecruit(state: GameState, v: Village, unit: UnitId, count: number): RecruitCheck {
  const def = UNITS[unit];
  const cost = unitCost(unit, count, state.config);
  const pop = def.pop * count;
  const perUnitMs = recruitTimeMs(unit, v.buildings[def.building], state.config, v);
  const blockers: RecruitBlocker[] = [];

  const missing = def.requirements.filter((r) => v.buildings[r.building] < r.level);
  if (missing.length) blockers.push({ kind: 'requirements', missing });
  if (effectiveResearchLevel(v, unit) < 1) blockers.push({ kind: 'research' });
  if (!Number.isInteger(count) || count < 1) blockers.push({ kind: 'count' });
  if (unit === 'paladin' && unitCountIncludingQueued(v, 'paladin') + count > 1) blockers.push({ kind: 'limit', max: 1 });
  if (unit === 'noble') {
    if (nobleCountOf(v, state) + count > nobleLimit(state.config, v)) blockers.push({ kind: 'limit', max: nobleLimit(state.config, v) });
    const owner = v.ownerId === null ? undefined : state.players[v.ownerId];
    if (owner && !state.config.noCoins) {
      const needed = coinsNeededFor(state, owner.id, count);
      if (owner.coins < needed) blockers.push({ kind: 'coins', have: owner.coins, needed });
    }
  }
  if (!hasResources(v.resources, cost)) {
    blockers.push({
      kind: 'resources',
      missing: {
        wood: Math.max(0, cost.wood - v.resources.wood),
        clay: Math.max(0, cost.clay - v.resources.clay),
        iron: Math.max(0, cost.iron - v.resources.iron),
      },
    });
  }
  const free = villageFarmCapacity(v) - villagePopulationUsed(v, state);
  if (pop > free) blockers.push({ kind: 'population', needed: pop, free });

  const hardBlocked = blockers.some((b) => b.kind === 'requirements' || b.kind === 'research');
  return { cost, pop, perUnitMs, maxAffordable: hardBlocked ? 0 : maxRecruitable(v, unit, state), blockers };
}

/**
 * Lazy recruitment settlement: units present at `now` = f(queue, now).
 * Called on every time advance (before events and at the end of `advance`).
 */
export function settleRecruitQueues(v: Village, now: number): void {
  for (const queue of Object.values(v.recruitQueues)) {
    while (queue.length > 0) {
      const job = queue[0]!;
      if (now < job.startedAt) break;
      const done = Math.min(job.total, Math.floor((now - job.startedAt) / job.perUnitMs));
      if (done > job.done) {
        v.units[job.unit] += done - job.done;
        job.done = done;
      }
      if (job.done < job.total) break;
      queue.shift();
    }
  }
}

/** Units recruitable in a given building (in table order). */
export function unitsForBuilding(building: RecruitBuilding): UnitId[] {
  return (Object.keys(UNITS) as UnitId[]).filter((u) => UNITS[u].building === building);
}
