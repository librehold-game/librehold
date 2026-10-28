import { BUILDINGS } from '../../data/buildings.data';
import { COMBAT_BALANCE, UNITS } from '../../data/units.data';
import { buildingCost, buildingPoints, buildingPopulation, buildTimeMs, farmCapacity, hideCapacity, productionPerHour,
  storageCapacity } from '../../engine/buildings';
import type { BuildingId, UnitId } from '../../engine/types';

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

/** Every level of a building at world speed 1 with the Town Hall at level 1: cost, time, total population and points. */
export function buildingLevels(id: BuildingId): LevelRow[] {
  const effect = buildingEffect(id);
  return Array.from({ length: BUILDINGS[id].maxLevel }, (_, i) => {
    const level = i + 1;
    const row: LevelRow = { level, cost: buildingCost(id, level), timeMs: buildTimeMs(id, level, 1, 1),
      pop: buildingPopulation(id, level), points: buildingPoints(id, level) };
    const e = effectAt(effect, level);
    if (e !== undefined) row.effect = e;
    return row;
  });
}
