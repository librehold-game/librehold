import { COMBAT_BALANCE as combatBalance, UNITS as units, UNIT_BALANCE as unitBalance } from './balance.json';
import { UNIT_IDS, type BuildingId, type RecruitBuilding, type UnitId } from '../engine/types';

export interface UnitDef {
  id: UnitId;
  cost: { wood: number; clay: number; iron: number };
  pop: number;
  attack: number;
  defense: number;
  defenseCavalry: number;
  defenseArcher: number;
  /** Minutes per tile. */
  speed: number;
  haul: number;
  /** Seconds at level 1 of the recruiting building. */
  baseRecruitSec: number;
  building: RecruitBuilding;
  /** Attack class: infantry / cavalry / archer (for splitting defence). */
  attackClass: 'infantry' | 'cavalry' | 'archer';
  requirements: { building: BuildingId; level: number }[];
}

export const UNITS = Object.fromEntries(UNIT_IDS.map((id) => [id, units[id]])) as Record<UnitId, UnitDef>;

export const COMBAT_BALANCE = combatBalance;
export const UNIT_BALANCE = unitBalance;
