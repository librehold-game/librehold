import { BUILDINGS } from '../data/buildings.data';
import { buildTimeMs, buildingCost, buildingPopulation, meetsRequirements } from './buildings';
import { hasResources, villageFarmCapacity, villagePopulationUsed } from './resources';
import { modifiersFor } from './modifiers';
import { buildQueueSlotsFor } from './players';
import type { BuildingId, GameState, ResourceSet, Village } from './types';

/** Building levels including queued jobs (build +1, demolish −1). */
export function effectiveBuildingLevels(v: Village): Record<BuildingId, number> {
  const levels = { ...v.buildings };
  for (const job of v.buildQueue) levels[job.building] = job.targetLevel;
  return levels;
}

export type BuildBlocker =
  | { kind: 'maxLevel' }
  | { kind: 'queueFull'; slots: number }
  | { kind: 'requirements'; missing: { building: BuildingId; level: number }[] }
  | { kind: 'resources'; missing: ResourceSet }
  | { kind: 'population'; needed: number; free: number };

export interface BuildCheck {
  targetLevel: number;
  cost: ResourceSet;
  popDelta: number;
  durationMs: number;
  blockers: BuildBlocker[];
}

/** Checks whether the next building level can be queued (no mutation). */
export function checkBuild(state: GameState, v: Village, building: BuildingId): BuildCheck {
  const def = BUILDINGS[building];
  const levels = effectiveBuildingLevels(v);
  const targetLevel = levels[building] + 1;
  const cost = buildingCost(building, Math.min(targetLevel, def.maxLevel));
  const popDelta = buildingPopulation(building, targetLevel) - buildingPopulation(building, targetLevel - 1);
  const durationMs = Math.round(buildTimeMs(building, Math.min(targetLevel, def.maxLevel), levels.main, state.config.worldSpeed) / modifiersFor(state.config, v).building);
  const blockers: BuildBlocker[] = [];

  if (targetLevel > def.maxLevel) blockers.push({ kind: 'maxLevel' });
  const slots = buildQueueSlotsFor(state, v);
  if (v.buildQueue.length >= slots) {
    blockers.push({ kind: 'queueFull', slots });
  }
  if (!meetsRequirements(levels, building)) {
    blockers.push({
      kind: 'requirements',
      missing: def.requirements.filter((r) => levels[r.building] < r.level),
    });
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
  if (popDelta > 0 && popDelta > free) blockers.push({ kind: 'population', needed: popDelta, free });

  return { targetLevel, cost, popDelta, durationMs, blockers };
}

export function canDemolish(state: GameState, v: Village, building: BuildingId): boolean {
  const levels = effectiveBuildingLevels(v);
  const min = building === 'main' ? 1 : 0;
  return levels[building] > min && v.buildQueue.length < buildQueueSlotsFor(state, v);
}
