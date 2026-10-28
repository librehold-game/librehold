import { BUILDINGS, BUILDING_BALANCE, RESOURCE_BALANCE, BONUS_BALANCE } from '../data/buildings.data';
import { BUILDING_IDS, type BuildingId, type ResourceSet, type VillageBonus } from './types';

export function buildingCost(id: BuildingId, level: number): ResourceSet {
  const def = BUILDINGS[id];
  return {
    wood: Math.round(def.baseCost.wood * Math.pow(def.costFactor.wood, level - 1)),
    clay: Math.round(def.baseCost.clay * Math.pow(def.costFactor.clay, level - 1)),
    iron: Math.round(def.baseCost.iron * Math.pow(def.costFactor.iron, level - 1)),
  };
}

/** Population used by a building at level L (cumulative value). */
export function buildingPopulation(id: BuildingId, level: number): number {
  if (level <= 0) return 0;
  const def = BUILDINGS[id];
  if (def.basePop === 0) return 0;
  return Math.round(def.basePop * Math.pow(def.popFactor, level - 1));
}

/** Building points at level L — a cumulative value, not a sum over levels. */
export function buildingPoints(id: BuildingId, level: number): number {
  if (level <= 0) return 0;
  return Math.round(BUILDINGS[id].basePoints * Math.pow(BUILDING_BALANCE.pointsFactor, level - 1));
}

/** Build time of level `level` in world-time ms. */
export function buildTimeMs(id: BuildingId, level: number, mainLevel: number, worldSpeed: number): number {
  const def = BUILDINGS[id];
  const B = BUILDING_BALANCE;
  const exponent = level <= 1 ? B.timeMinExponent : Math.max(B.timeMinExponent, level - 1 - B.timeShift / (level - 1));
  const baseSec = def.baseTimeSec * B.timeConst * Math.pow(B.timeFactor, exponent);
  const mainFactor = Math.pow(B.mainTimeFactor, -Math.max(0, mainLevel));
  const ms = (baseSec * mainFactor * 1000) / worldSpeed;
  return Math.max(B.minBuildTimeMs, Math.round(ms));
}

export function productionPerHour(level: number): number {
  if (level <= 0) return 0;
  return Math.round(RESOURCE_BALANCE.productionBase * Math.pow(RESOURCE_BALANCE.productionFactor, level - 1));
}

/** Warehouse capacity: L1 = 1000, L15 ≈ 18 000, L30 ≈ 400 000. */
export function storageCapacity(level: number, bonus?: VillageBonus): number {
  const base = level <= 0
    ? RESOURCE_BALANCE.storageLevel0
    : Math.round(RESOURCE_BALANCE.storageBase * Math.pow(RESOURCE_BALANCE.storageFactor, level - 1));
  return bonus?.kind === 'storage' ? Math.round(base * BONUS_BALANCE.storageCapacity) : base;
}

export function farmCapacity(level: number, bonus?: VillageBonus): number {
  const base = level <= 0
    ? RESOURCE_BALANCE.farmLevel0
    : Math.round(RESOURCE_BALANCE.farmBase * Math.pow(RESOURCE_BALANCE.farmFactor, level - 1));
  return bonus?.kind === 'farm' ? Math.round(base * BONUS_BALANCE.farmCapacity) : base;
}

export function hideCapacity(level: number): number {
  if (level <= 0) return 0;
  return Math.round(RESOURCE_BALANCE.hideBase * Math.pow(RESOURCE_BALANCE.hideFactor, Math.min(level, BUILDINGS.hide.maxLevel) - 1));
}

export function buildingsPopulation(buildings: Record<BuildingId, number>): number {
  let sum = 0;
  for (const id of BUILDING_IDS) sum += buildingPopulation(id, buildings[id]);
  return sum;
}

export function meetsRequirements(buildings: Record<BuildingId, number>, id: BuildingId): boolean {
  return BUILDINGS[id].requirements.every((r) => buildings[r.building] >= r.level);
}

