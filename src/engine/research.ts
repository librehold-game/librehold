import { RESEARCH_BALANCE, RESEARCH_UNLOCK_COST } from '../data/research.data';
import { UNITS, UNIT_BALANCE } from '../data/units.data';
import type { ResourceSet, UnitId } from './types';

/**
 * Whether a unit must be unlocked in the Smithy (yes if its requirements include the Smithy). The others (spear, ram,
 * catapult, paladin, noble) get level 1 for free; the Smithy upgrades all but the noble to levels 2–3.
 */
export function unitNeedsResearch(unit: UnitId): boolean {
  return UNITS[unit].requirements.some((r) => r.building === 'smithy');
}

export function researchBonus(level: number): number {
  return UNIT_BALANCE.researchBonus[Math.max(0, Math.min(level, RESEARCH_BALANCE.maxLevel))] ?? 0;
}

export function researchCost(unit: UnitId, level: number): ResourceSet {
  const base = RESEARCH_UNLOCK_COST[unit];
  const mult = UNIT_BALANCE.researchCostMultiplier[level] ?? 0;
  return { wood: base.wood * mult, clay: base.clay * mult, iron: base.iron * mult };
}

export function researchTimeMs(unit: UnitId, level: number, worldSpeed: number): number {
  const c = researchCost(unit, level);
  const ms = ((c.wood + c.clay + c.iron) * UNIT_BALANCE.researchTimePerResourceSec * 1000) / worldSpeed;
  return Math.max(RESEARCH_BALANCE.minResearchTimeMs, Math.round(ms));
}
