import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { CONQUEST_BALANCE } from '../../data/conquest.data';
import { RESEARCH_BALANCE } from '../../data/research.data';
import { COMBAT_BALANCE, UNITS } from '../../data/units.data';
import { buildingCost, buildingPoints, buildingPopulation, buildTimeMs, farmCapacity, hideCapacity, productionPerHour,
  storageCapacity } from '../../engine/buildings';
import { researchBonus, researchCost, researchTimeMs, unitNeedsResearch } from '../../engine/research';
import { UNIT_IDS, type BuildingId, type ResourceSet, type UnitId } from '../../engine/types';

export type { BuildingId, UnitId } from '../../engine/types';

export interface UnitFacts {
  id: UnitId; cost: { wood: number; clay: number; iron: number }; pop: number; attack: number;
  defense: number; defenseCavalry: number; defenseArcher: number; minutesPerField: number; carry: number;
  building: BuildingId; recruitSec: number; requirements: { building: BuildingId; level: number }[];
}

export function unitFacts(id: UnitId): UnitFacts {
  const u = UNITS[id];
  return {
    id, cost: { ...u.cost }, pop: u.pop, attack: u.attack, defense: u.defense, defenseCavalry: u.defenseCavalry,
    defenseArcher: u.defenseArcher, minutesPerField: u.speed, carry: u.haul, building: u.building, recruitSec: u.baseRecruitSec,
    requirements: u.requirements.map((r) => ({ ...r })),
  };
}

export type BuildingEffect = 'production' | 'storage' | 'farm' | 'hide' | 'wall' | null;

export function buildingEffect(id: BuildingId): BuildingEffect {
  switch (id) {
    case 'wood': case 'clay': case 'iron': return 'production';
    case 'storage': return 'storage';
    case 'farm': return 'farm';
    case 'hide': return 'hide';
    case 'wall': return 'wall';
    default: return null;
  }
}

export interface LevelRow { level: number; cost: { wood: number; clay: number; iron: number }; timeMs: number; pop: number; points: number; effect?: number }

function effectAt(effect: BuildingEffect, level: number): number | undefined {
  switch (effect) {
    case 'production': return productionPerHour(level);
    case 'storage': return storageCapacity(level);
    case 'farm': return farmCapacity(level);
    case 'hide': return hideCapacity(level);
    case 'wall': return Math.round((COMBAT_BALANCE.wallDefenseFactor ** level - 1) * 100);
    default: return undefined;
  }
}

/** Every level of a building: cost, time (by default at world speed 1 with the Town Hall at level 1), total population and points. */
export function buildingLevels(id: BuildingId, timing: Timing = BASE_TIMING): LevelRow[] {
  const effect = buildingEffect(id);
  return Array.from({ length: BUILDINGS[id].maxLevel }, (_, i) => {
    const level = i + 1;
    const row: LevelRow = { level, cost: buildingCost(id, level), timeMs: timing.buildMs(id, level),
      pop: buildingPopulation(id, level), points: buildingPoints(id, level) };
    const e = effectAt(effect, level);
    if (e !== undefined) row.effect = e;
    return row;
  });
}

/** What depends on the world: build, recruit and research times, the noble's price, the coin cost. */
export interface Timing {
  worldSpeed: number;
  buildMs(id: BuildingId, level: number): number;
  recruitMs(unit: UnitId): number;
  researchMs(unit: UnitId, level: number): number;
  unitCost(unit: UnitId): ResourceSet;
  /** null when the world needs no coins. */
  coinCost: ResourceSet | null;
}

/** The public encyclopedia's numbers: world speed 1, Town Hall 1, a level 1 recruiting building, standard noble rules. */
export const BASE_TIMING: Timing = {
  worldSpeed: 1,
  buildMs: (id, level) => buildTimeMs(id, level, 1, 1),
  recruitMs: (unit) => UNITS[unit].baseRecruitSec * 1000,
  researchMs: (unit, level) => researchTimeMs(unit, level, 1),
  unitCost: (unit) => ({ ...UNITS[unit].cost }),
  coinCost: { ...CONQUEST_BALANCE.coinBaseCost },
};

export interface Requirement { building: BuildingId; level: number }

export interface ReqNode extends Requirement {
  /** The village's level of the building; null in the public view. */
  have: number | null;
  met: boolean;
  /** What this building itself needs: always in the public view, only while unmet with village levels. */
  children: ReqNode[];
}

/** The requirements as a tree; with the village's building levels each node says whether it is met and what is missing below it. */
export function requirementChain(reqs: readonly Requirement[], levels?: Record<BuildingId, number>, path: readonly BuildingId[] = []): ReqNode[] {
  return reqs.filter((r) => !path.includes(r.building)).map((r) => {
    const have = levels ? levels[r.building] : null;
    const met = have !== null && have >= r.level;
    const children = met ? [] : requirementChain(BUILDINGS[r.building].requirements, levels, [...path, r.building]);
    return { building: r.building, level: r.level, have, met, children };
  });
}

export interface Unlock { kind: 'unit' | 'building'; id: UnitId | BuildingId; level: number; recruitedHere: boolean }

/** Every unit and building whose requirements include this building, units first. */
export function unlocksOf(building: BuildingId): Unlock[] {
  const out: Unlock[] = [];
  for (const u of UNIT_IDS) {
    const r = UNITS[u].requirements.find((x) => x.building === building);
    if (r) out.push({ kind: 'unit', id: u, level: r.level, recruitedHere: UNITS[u].building === building });
  }
  for (const b of BUILDING_ORDER) {
    const r = BUILDINGS[b].requirements.find((x) => x.building === building);
    if (r) out.push({ kind: 'building', id: b, level: r.level, recruitedHere: false });
  }
  return out;
}

export interface ResearchRow {
  level: number;
  bonus: number;
  attack: number;
  defense: number;
  defenseCavalry: number;
  defenseArcher: number;
  cost: ResourceSet;
  timeMs: number;
  /** Level 1 of a unit that needs no unlocking in the Smithy. */
  free: boolean;
}

/** Each research level's battle stats (the bonus the combat rules apply to attack and defence), cost and time. The noble is not researched. */
export function researchRows(unit: UnitId, timing: Timing): ResearchRow[] {
  if (unit === 'noble') return [];
  const u = UNITS[unit];
  return Array.from({ length: RESEARCH_BALANCE.maxLevel }, (_, i) => {
    const level = i + 1;
    const bonus = researchBonus(level);
    const scale = (n: number) => Math.round(n * (1 + bonus));
    return {
      level, bonus, attack: scale(u.attack), defense: scale(u.defense), defenseCavalry: scale(u.defenseCavalry),
      defenseArcher: scale(u.defenseArcher), cost: researchCost(unit, level), timeMs: timing.researchMs(unit, level),
      free: level === 1 && !unitNeedsResearch(unit),
    };
  });
}
