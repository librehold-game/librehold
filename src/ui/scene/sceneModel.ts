/**
 * Pure model of the village illustration (16.6). Computed from GameState, without React/DOM — testable.
 * The animation layer only READS this model; it never writes to the game state.
 */
import { BUILDINGS } from '../../data/buildings.data';
import { villageProductionPerHour, villageStorageCapacity, villagePopulationUsed } from '../../engine/resources';
import { hashSeed, mulberry32 } from '../../engine/rng';
import { RESOURCE_BALANCE } from '../../data/buildings.data';
import { MS_PER_DAY, MS_PER_HOUR, MS_PER_MINUTE, UNIT_IDS, type BuildingId, type GameState, type UnitId, type VillageBonusKind } from '../../engine/types';
import { isHumanVillage, humanVillageIds, humanCoins } from '../../engine/players';

export const SCENE_BALANCE = {
  cycleMinMs: 1200,
  cycleMaxMs: 6000,
  beatThrottleMs: 800,
  /** After a skip of > 60 s the beat queue is cleared (16.6.4). */
  beatClearSpanMs: 60_000,
  maxAnimated: 30,
  alarmWindowMs: 10 * MS_PER_MINUTE,
  defeatWindowMs: 30 * MS_PER_MINUTE,
  conquestFlagWindowMs: 5 * MS_PER_MINUTE,
  citizensMin: 2,
  citizensMax: 18,
} as const;

export type DayPhase = 'dawn' | 'day' | 'dusk' | 'night';
export type Variant = 0 | 1 | 2 | 3;

export interface WorkshopState { active: boolean; unit: UnitId | null; stage: number; cycleMs: number; figures: number }
export interface QueueState { active: boolean; unit: UnitId | null; remaining: number; cycleMs: number; figures: number }

export interface SceneModel {
  villageId: string;
  levels: Record<BuildingId, number>;
  variants: Record<BuildingId, Variant>;
  construction: { building: BuildingId; ratio: number; started: boolean }[];
  /** Woodcutter swing period in ms (lower = higher production). */
  woodSwingMs: number;
  clayActive: boolean;
  /** Iron mine smoke density 0..3. */
  ironSmoke: number;
  guards: number;
  troopFigures: number;
  nobles: number;
  paladinHome: boolean;
  caravan: boolean;
  storageFull: boolean;
  alarm: boolean;
  alarmEtaMs: number | null;
  defeated: boolean;
  conqueredFlag: boolean;
  bonus: VillageBonusKind | null;
  barracks: QueueState;
  stable: QueueState;
  workshop: WorkshopState;
  smithy: { active: boolean; progress: number; twoForges: boolean; racks: number };
  academy: { coins: number; nobleProgress: number | null };
  statue: { active: boolean; knightHome: boolean };
  main: { messengers: number };
  citizens: number;
  dayPhase: DayPhase;
  rain: boolean;
  population: number;
}

export const clamp = (x: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, x));

/** Three stages of building art — thresholds proportional to the max level (1/3 and 2/3). */
export function variantFor(level: number, maxLevel = 30): Variant {
  if (level <= 0) return 0;
  const stage = Math.ceil((level / Math.max(1, maxLevel)) * 3);
  return clamp(stage, 1, 3) as Variant;
}

/** 16.6.4: czas cyklu animacji = clamp(perUnitMs, 1200, 6000). */
export function cycleMs(perUnitMs: number): number {
  return clamp(perUnitMs, SCENE_BALANCE.cycleMinMs, SCENE_BALANCE.cycleMaxMs);
}

/** 16.6.2: liczba_postaci = clamp(round(sqrt(populacja)/4), 2, 18). */
export function citizenCount(population: number): number {
  return clamp(Math.round(Math.sqrt(Math.max(0, population)) / 4), SCENE_BALANCE.citizensMin, SCENE_BALANCE.citizensMax);
}

/** Drill ground: count = clamp(round(log2(remaining)), 2, 8). */
export function trainingFigures(remaining: number): number {
  return clamp(Math.round(Math.log2(Math.max(1, remaining))), 2, 8);
}

/** Troops at the rally point on a log scale (10 vs 10 000 differ, but not 1000×). */
export function troopFigures(total: number): number {
  if (total <= 0) return 0;
  return clamp(Math.ceil(Math.log2(total + 1)), 1, 8);
}

export function wallGuards(wallLevel: number): number {
  return wallLevel <= 0 ? 0 : Math.ceil(wallLevel / 5);
}

/** Time of day from local wall-clock time (`clock.originLocalMs + now`), the same hour as the clock and the bots. */
export function dayPhase(localMs: number): DayPhase {
  const h = (localMs % MS_PER_DAY) / MS_PER_HOUR;
  if (h >= 5 && h < 7) return 'dawn';
  if (h >= 7 && h < 19) return 'day';
  if (h >= 19 && h < 21) return 'dusk';
  return 'night';
}

/** Purely cosmetic rain: ~10 % of world hours, deterministic from the village seed. */
export function isRaining(villageId: string, now: number): boolean {
  const hour = Math.floor(now / MS_PER_HOUR);
  return mulberry32(hashSeed(villageId, 'rain', hour))() < 0.1;
}

/** Woodcutter swing period scaled by production: 30/h → 2 s, 1000/h → ~0.6 s. */
export function woodSwingMs(perHour: number): number {
  if (perHour <= 0) return 0;
  return clamp(Math.round(2000 * Math.pow(RESOURCE_BALANCE.productionBase / Math.max(1, perHour), 0.35)), 450, 2400);
}

function roundRatio(x: number): number {
  return Math.round(clamp(x, 0, 1) * 50) / 50;
}

const FREE_RESEARCH: readonly UnitId[] = ['spear', 'ram', 'catapult', 'paladin'];

export function sceneModel(state: GameState, villageId: string): SceneModel | null {
  const v = state.villages[villageId];
  if (!v) return null;
  const now = state.now;
  const levels = v.buildings;
  const variants = {} as Record<BuildingId, Variant>;
  for (const b of Object.keys(levels) as BuildingId[]) variants[b] = variantFor(levels[b], BUILDINGS[b].maxLevel);

  const construction = v.buildQueue.map((j) => ({
    building: j.building,
    started: now >= j.startedAt,
    ratio: now < j.startedAt ? 0 : roundRatio((now - j.startedAt) / Math.max(1, j.completesAt - j.startedAt)),
  }));

  const woodPerHour = villageProductionPerHour(v, 'wood', state.config);
  const cap = villageStorageCapacity(v);
  const storageFull = cap > 0 && (['wood', 'clay', 'iron'] as const).some((r) => v.resources[r] / cap >= RESOURCE_BALANCE.storageWarningRatio);

  let troops = 0;
  for (const u of UNIT_IDS) if (u !== 'noble' && u !== 'paladin') troops += v.units[u];
  for (const sup of Object.values(v.supportFrom)) for (const u of UNIT_IDS) troops += sup[u] ?? 0;

  let alarmEta: number | null = null;
  for (const c of state.commands) {
    if (c.type !== 'attack' || c.targetId !== villageId) continue;
    const origin = state.villages[c.originId];
    if (!origin || isHumanVillage(origin)) continue;
    const eta = c.arrivesAt - now;
    if (eta <= SCENE_BALANCE.alarmWindowMs && (alarmEta === null || eta < alarmEta)) alarmEta = eta;
  }
  const caravan = state.commands.some((c) => c.type === 'trade' && c.originId === villageId && !c.returning);

  let defeated = false;
  for (let i = state.reports.length - 1; i >= 0; i--) {
    const r = state.reports[i]!;
    if (now - r.time > SCENE_BALANCE.defeatWindowMs) break;
    if (r.type === 'attack' && r.targetId === villageId && r.outcome === 'attackerWon' && !humanVillageIds(state).includes(r.originId)) { defeated = true; break; }
  }

  const queueState = (b: 'barracks' | 'stable'): QueueState => {
    const q = v.recruitQueues[b];
    const job = q[0];
    if (!job || now < job.startedAt) return { active: false, unit: null, remaining: 0, cycleMs: 0, figures: 0 };
    let remaining = 0;
    for (const j of q) remaining += j.total - j.done;
    return { active: true, unit: job.unit, remaining, cycleMs: cycleMs(job.perUnitMs), figures: trainingFigures(remaining) };
  };
  const wq = v.recruitQueues.workshop[0];
  const workshop: WorkshopState = wq && now >= wq.startedAt
    ? { active: true, unit: wq.unit, stage: clamp(Math.floor((((now - wq.startedAt) % wq.perUnitMs) / wq.perUnitMs) * 4), 0, 3), cycleMs: cycleMs(wq.perUnitMs), figures: 2 }
    : { active: false, unit: null, stage: 0, cycleMs: 0, figures: 0 };

  const rj = v.researchQueue[0];
  let racks = 0;
  for (const u of UNIT_IDS) racks += Math.max(0, v.research[u] - (FREE_RESEARCH.includes(u) ? 1 : 0));
  const smithy = {
    active: !!rj && now >= rj.startedAt,
    progress: rj && now >= rj.startedAt ? roundRatio((now - rj.startedAt) / Math.max(1, rj.completesAt - rj.startedAt)) : 0,
    twoForges: levels.smithy >= 12,
    racks: clamp(racks, 0, 6),
  };
  const aj = v.recruitQueues.academy[0];
  const academy = {
    coins: humanCoins(state),
    nobleProgress: aj && now >= aj.startedAt ? roundRatio(((now - aj.startedAt) % aj.perUnitMs) / aj.perUnitMs) : null,
  };
  const paladinAway = state.commands.some((c) => (c.units.paladin ?? 0) > 0 && (c.type === 'return' ? c.targetId : c.originId) === villageId);
  const statue = { active: v.recruitQueues.statue.length > 0 && now >= (v.recruitQueues.statue[0]?.startedAt ?? Infinity), knightHome: v.units.paladin > 0 && !paladinAway };

  const population = villagePopulationUsed(v, state);
  const phase = dayPhase(state.clock.originLocalMs + now);
  let citizens = citizenCount(population);
  if (phase === 'night') citizens = Math.max(1, Math.floor(citizens / 2));
  if (defeated) citizens = Math.max(1, Math.floor(citizens / 2));
  if (alarmEta !== null) citizens = 0;

  return {
    villageId,
    levels: { ...levels },
    variants,
    construction,
    woodSwingMs: levels.wood > 0 ? woodSwingMs(woodPerHour) : 0,
    clayActive: levels.clay > 0,
    ironSmoke: levels.iron <= 0 ? 0 : clamp(1 + Math.floor(levels.iron / 10), 1, 3),
    guards: wallGuards(levels.wall),
    troopFigures: troopFigures(troops),
    nobles: v.units.noble,
    paladinHome: statue.knightHome,
    caravan,
    storageFull,
    alarm: alarmEta !== null,
    alarmEtaMs: alarmEta,
    defeated,
    conqueredFlag: v.conqueredAt !== undefined && now - v.conqueredAt <= SCENE_BALANCE.conquestFlagWindowMs,
    bonus: v.bonus?.kind ?? null,
    barracks: queueState('barracks'),
    stable: queueState('stable'),
    workshop,
    smithy,
    academy,
    statue,
    main: { messengers: v.buildQueue.filter((j) => now >= j.startedAt).length },
    citizens,
    dayPhase: phase,
    rain: isRaining(villageId, now),
    population,
  };
}

/** Beats (16.6.4): throttled to 1 per 800 ms per building; with faster production a group of 3–5 figures. */
export interface Beat { building: string; at: number; count: number }

export function mergeBeat(prev: Beat | undefined, building: string, nowMs: number, units: number): Beat {
  if (prev && nowMs - prev.at < SCENE_BALANCE.beatThrottleMs) return { building, at: prev.at, count: clamp(prev.count + units, 3, 5) };
  return { building, at: nowMs, count: clamp(units, 1, 5) };
}

/** Guards patrolling the inner wall, one per arc of the pixel scene. */
export const PATROLS = 2;

/**
 * Animated elements of the pixel scene within the budget of 30 (16.6.6): state indicators and frame strips first,
 * then guards, troops and walkers. Counts only what the scene actually draws.
 */
export function animationBudget(m: SceneModel, mode: 'full' | 'limited' | 'off'): { citizens: number; weather: boolean; troops: number; guards: number } {
  if (mode !== 'full') return { citizens: 0, weather: false, troops: 0, guards: 0 };
  const strips = (['wood', 'iron', 'farm'] as const).filter((b) => m.levels[b] > 0 && m.variants[b] === 2).length;
  const workers = (m.levels.wood > 0 ? 1 : 0) + (m.construction.some((c) => c.started) ? 1 : 0);
  let used = (m.alarm ? 2 : 0) + (m.defeated ? 1 : 0) + (m.caravan ? 1 : 0) + (m.conqueredFlag ? 1 : 0)
    + (m.levels.wall > 0 ? 1 : 0) + strips + workers;
  const max = SCENE_BALANCE.maxAnimated;
  const guards = clamp(max - used, 0, Math.min(m.guards, PATROLS));
  used += guards;
  const troops = clamp(max - used, 0, m.troopFigures);
  used += troops;
  const citizens = clamp(max - used, 0, m.citizens);
  used += citizens;
  return { citizens, weather: used + 2 <= max, troops, guards };
}
