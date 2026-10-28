import { FARM_BALANCE } from '../data/world.config';
import type { GameState } from './types';
import { humanVillageIds } from './players';

// The farm assistant's targets come from the server (GET …/farm-targets); the browser only lists them.

export type FarmStatus = 'untouched' | 'full' | 'partial' | 'losses' | 'inbound';

export interface FarmTarget {
  id: string;
  x: number;
  y: number;
  distance: number;
  points: number;
  lastAttackAt: number | null;
  lastHaul: number | null;
  lastHaulCapacity: number | null;
  lastLosses: number | null;
  wall: number | null;
  /** Last seen garrison (from a report or scouting); null = unknown. */
  defenders: number | null;
  inbound: boolean;
  status: FarmStatus;
  /** Estimated loot available (from the last intel + production since then). */
  estimatedLoot: number;
}

export function autoFarmUnlocked(state: GameState): boolean {
  return state.farmAssistant.autoFarmUnlocked || humanVillageIds(state).length >= FARM_BALANCE.autoFarmUnlockVillages;
}
