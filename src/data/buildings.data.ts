import {
  BONUS_BALANCE as bonusBalance, BUILDINGS as buildings, BUILDING_BALANCE as buildingBalance, BUILDING_ORDER as buildingOrder,
  RESOURCE_BALANCE as resourceBalance,
} from './balance.json';
import { BUILDING_IDS, type BuildingId } from '../engine/types';

// The buildings' tables come from the server (src/data/balance.json, written by `npm run gen`); the formulas
// that use them live in server/internal/game.

export interface BuildingRequirement {
  building: BuildingId;
  level: number;
}

export interface BuildingDef {
  id: BuildingId;
  maxLevel: number;
  baseCost: { wood: number; clay: number; iron: number };
  /** Cost multipliers per resource. */
  costFactor: { wood: number; clay: number; iron: number };
  basePop: number;
  popFactor: number;
  /** Base time (s). */
  baseTimeSec: number;
  /** Points at level 1; points(L) = round(base · 1.2^(L−1)). */
  basePoints: number;
  requirements: BuildingRequirement[];
}

export const BUILDINGS = Object.fromEntries(BUILDING_IDS.map((id) => [id, buildings[id]])) as Record<BuildingId, BuildingDef>;

/** Display order in the buildings table. */
export const BUILDING_ORDER = buildingOrder as readonly BuildingId[];

export const BUILDING_BALANCE = buildingBalance;
export const RESOURCE_BALANCE = resourceBalance;
/** Bonus villages' multipliers. */
export const BONUS_BALANCE = bonusBalance;
