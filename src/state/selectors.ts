import type { GameStore } from './gameStore';
import { selectActiveVillage, selectGame } from './gameStore';
import { villageFarmCapacity, villagePopulationUsed, villageStorageCapacity } from '../engine/resources';

export const selectNow = (s: GameStore): number => selectGame(s).now;
export const selectConfig = (s: GameStore) => selectGame(s).config;
export const selectActiveVillageId = (s: GameStore): string => selectGame(s).activeVillageId;
export const selectActiveBuildings = (s: GameStore) => selectActiveVillage(s).buildings;
export const selectActiveStorageCap = (s: GameStore): number => villageStorageCapacity(selectActiveVillage(s));
export const selectActiveFarmCap = (s: GameStore): number => villageFarmCapacity(selectActiveVillage(s));
export const selectActivePopUsed = (s: GameStore): number => villagePopulationUsed(selectActiveVillage(s), selectGame(s));
export const selectActivePoints = (s: GameStore): number => selectActiveVillage(s).points;
