import { CONQUEST_BALANCE } from '../data/conquest.data';
import { MS_PER_HOUR, type GameState, type PlayerId, type Village, type WorldConfig } from './types';

/** World time per loyalty point (+1/h, sped up by world speed). */
function msPerLoyaltyPoint(worldSpeed: number): number {
  return MS_PER_HOUR / (CONQUEST_BALANCE.loyaltyPerHour * worldSpeed);
}

/** Loyalty at `now` without mutation (for the UI). */
export function loyaltyAt(v: Village, now: number, worldSpeed: number): number {
  const points = Math.max(0, Math.floor((now - v.loyaltyUpdatedAt) / msPerLoyaltyPoint(worldSpeed)));
  return Math.min(CONQUEST_BALANCE.maxLoyalty, v.loyalty + points);
}

/** The coin cost is fixed. */
function coinCost(_state: GameState): { wood: number; clay: number; iron: number } {
  return { ...CONQUEST_BALANCE.coinBaseCost };
}

/** Coins needed for the next noble: villages + nobles owned. */
export function coinsForNextNoble(state: GameState, playerId: PlayerId, ownedNobles: number): number {
  return (state.players[playerId]?.villageIds.length ?? 0) * CONQUEST_BALANCE.coinsPerVillage + ownedNobles * CONQUEST_BALANCE.coinsPerNoble;
}

export type MintBlocker = { kind: 'academy' } | { kind: 'resources'; missing: { wood: number; clay: number; iron: number } };

export function checkMint(state: GameState, v: Village): { cost: { wood: number; clay: number; iron: number }; blockers: MintBlocker[] } {
  const cost = coinCost(state);
  const blockers: MintBlocker[] = [];
  if (v.buildings.academy < 1) blockers.push({ kind: 'academy' });
  if (v.resources.wood < cost.wood || v.resources.clay < cost.clay || v.resources.iron < cost.iron) {
    blockers.push({
      kind: 'resources',
      missing: {
        wood: Math.max(0, cost.wood - v.resources.wood),
        clay: Math.max(0, cost.clay - v.resources.clay),
        iron: Math.max(0, cost.iron - v.resources.iron),
      },
    });
  }
  return { cost, blockers };
}

/** How many more nobles a loyalty needs; a village with an owner may drop faster in the world's own range. */
export function noblesNeededFor(config: WorldConfig, owned: boolean, loyalty: number): number {
  const lo = owned && config.ownedNobleDropMax ? config.ownedNobleDropMin ?? 0 : CONQUEST_BALANCE.nobleDropMin;
  const hi = owned && config.ownedNobleDropMax ? config.ownedNobleDropMax : CONQUEST_BALANCE.nobleDropMax;
  return Math.max(1, Math.ceil(loyalty / ((lo + hi) / 2)));
}
