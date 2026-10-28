import { RESEARCH_BALANCE as researchBalance, RESEARCH_UNLOCK_COST as researchUnlockCost } from './balance.json';
import { UNIT_IDS, type ResourceSet, type UnitId } from '../engine/types';

/** Cost of unlocking a unit (research level 1 in the Smithy); levels 2 and 3 cost UNIT_BALANCE.researchCostMultiplier times it. */
export const RESEARCH_UNLOCK_COST = Object.fromEntries(UNIT_IDS.map((id) => [id, researchUnlockCost[id]])) as Record<UnitId, ResourceSet>;

export const RESEARCH_BALANCE = researchBalance;
