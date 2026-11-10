import { TRADE_BALANCE } from '../data/world.config';
import { checkTrade, type TradeBlocker } from './market';
import { humanVillageIds } from './players';
import { HUMAN_ID, RESOURCES, type GameState, type ResourceSet, type TradeOffer } from './types';

/** Everything I posted. */
export function myOffers(g: GameState): TradeOffer[] {
  return g.tradeOffers.filter((o) => o.fromId === HUMAN_ID);
}

/** Board posts from other players the server made visible (in range of me). */
export function publicBoard(g: GameState): TradeOffer[] {
  return g.tradeOffers.filter((o) => o.fromId !== HUMAN_ID);
}

/** What I may do with an offer: take my own back, or accept someone else's. */
export function offerActions(offer: TradeOffer, me: string): ('accept' | 'cancel')[] {
  return offer.fromId === me ? ['cancel'] : ['accept'];
}

/** Why a deal cannot settle at `villageId` right now; empty means it can. `copies` checks the whole batch of
 * identical offers at once: my leg is scaled up, so both resources and merchants must cover every copy. */
export function acceptBlockers(g: GameState, offer: TradeOffer, villageId: string, copies = 1): TradeBlocker[] {
  const ownIds = new Set(humanVillageIds(g));
  if (!ownIds.has(villageId)) return [{ kind: 'notOwned' }];
  const v = g.villages[villageId];
  if (!v || v.buildings.market < 1) return [{ kind: 'noMarket' }];
  // My leg is a plain market trade of the "want" resources; nothing to ship when it is empty.
  const want = RESOURCES.reduce((n, r) => n + offer.want[r], 0);
  if (want === 0) return [];
  const n = Math.max(1, Math.floor(copies));
  const scaled: ResourceSet = { wood: offer.want.wood * n, clay: offer.want.clay * n, iron: offer.want.iron * n };
  const check = checkTrade(g, villageId, offer.originId, scaled);
  return check.blockers.filter((b) => b.kind !== 'self' && b.kind !== 'notOwned');
}

/** A posted delivery limit as the server stores it: 0 stays "no limit", anything else is clamped to the band.
 * Mirrors `normalizeDelivery` in `server/internal/game/trade.go`, which is part of a deal's identity there. */
export function normalizeDelivery(ms: number): number {
  if (!(ms > 0)) return 0;
  return Math.min(TRADE_BALANCE.maxDeliveryMs, Math.max(TRADE_BALANCE.minDeliveryMs, ms));
}
