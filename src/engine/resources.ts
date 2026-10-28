import { RESOURCE_BALANCE, BONUS_BALANCE } from '../data/buildings.data';
import { UNITS } from '../data/units.data';
import { productionPerHour, storageCapacity, farmCapacity, buildingsPopulation } from './buildings';
import {
  MS_PER_HOUR, RESOURCES, UNIT_IDS,
  type GameState, type Resource, type ResourceSet, type Village, type VillageBonus, type WorldConfig,
} from './types';
import { unitsAway } from './commands';
import { modifiersFor } from './modifiers';

export function bonusProductionMultiplier(bonus: VillageBonus | undefined, r: Resource): number {
  if (!bonus) return 1;
  if (bonus.kind === r) return BONUS_BALANCE.singleResource;
  if (bonus.kind === 'allResources') return BONUS_BALANCE.allResources;
  return 1;
}

/** Hourly production of a resource in a village (with the 5/h base, bonus, world speed and side multiplier). */
export function villageProductionPerHour(v: Village, r: Resource, cfg: WorldConfig): number {
  const fromBuilding = productionPerHour(v.buildings[r]);
  const rate = (RESOURCE_BALANCE.baseProductionPerHour + fromBuilding) * bonusProductionMultiplier(v.bonus, r);
  return rate * cfg.worldSpeed * modifiersFor(cfg, v).resources;
}

export function villageStorageCapacity(v: Village): number {
  return storageCapacity(v.buildings.storage, v.bonus);
}

export function villageFarmCapacity(v: Village): number {
  return farmCapacity(v.buildings.farm, v.bonus);
}

/**
 * Population used: buildings (including queued levels — population is reserved
 * when ordered) + units in the village + units in recruitment queues.
 */
export function villagePopulationUsed(v: Village, state?: GameState): number {
  const levels = { ...v.buildings };
  for (const job of v.buildQueue) levels[job.building] = Math.max(levels[job.building], job.targetLevel);
  let pop = buildingsPopulation(levels);
  for (const u of UNIT_IDS) pop += v.units[u] * UNITS[u].pop;
  for (const queue of Object.values(v.recruitQueues)) {
    for (const job of queue) pop += (job.total - job.done) * UNITS[job.unit].pop;
  }
  // Barbarian villages have no troops away from home — skip the costly iteration over commands.
  if (state && v.ownerId !== null) {
    const away = unitsAway(state, v.id);
    for (const u of UNIT_IDS) pop += away[u] * UNITS[u].pop;
  }
  return pop;
}

/**
 * Advances a village's resources to `now`. The function is linear in time and capped
 * by warehouse capacity — advancing in one jump or in steps gives the same result.
 */
export function accrue(v: Village, now: number, cfg: WorldConfig): void {
  if (now <= v.resourcesUpdatedAt) return;
  const hours = (now - v.resourcesUpdatedAt) / MS_PER_HOUR;
  const cap = villageStorageCapacity(v);
  for (const r of RESOURCES) {
    const rate = villageProductionPerHour(v, r, cfg);
    const current = v.resources[r];
    // Resources above capacity (e.g. from loot) don't grow, but aren't cut down here
    // below the current amount either — the surplus is cut when it arrives (loot/trade).
    v.resources[r] = current >= cap ? current : Math.min(cap, current + rate * hours);
  }
  v.resourcesUpdatedAt = now;
}

/** World-time ms until the warehouse fills up for a resource (Infinity if production is 0). */
export function msUntilFull(v: Village, r: Resource, cfg: WorldConfig): number {
  const cap = villageStorageCapacity(v);
  const rate = villageProductionPerHour(v, r, cfg);
  if (v.resources[r] >= cap) return 0;
  if (rate <= 0) return Infinity;
  return ((cap - v.resources[r]) / rate) * MS_PER_HOUR;
}

/** World-time ms until `amount` of a resource is available (0 if already there). */
export function msUntilAmount(v: Village, r: Resource, amount: number, cfg: WorldConfig): number {
  if (v.resources[r] >= amount) return 0;
  const cap = villageStorageCapacity(v);
  if (amount > cap) return Infinity;
  const rate = villageProductionPerHour(v, r, cfg);
  if (rate <= 0) return Infinity;
  return ((amount - v.resources[r]) / rate) * MS_PER_HOUR;
}

export function hasResources(have: ResourceSet, cost: ResourceSet): boolean {
  return have.wood >= cost.wood && have.clay >= cost.clay && have.iron >= cost.iron;
}

