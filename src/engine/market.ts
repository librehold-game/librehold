import { COMMAND_BALANCE, TRADE_BALANCE } from '../data/world.config';
import { villageStorageCapacity } from './resources';
import { MS_PER_MINUTE, RESOURCES, type GameState, type ResourceSet, type TradeOffer, type Village } from './types';
import { distance } from './world';
import { isHumanVillage } from './players';

/**
 * How many merchants a market level gives: one per level up to `merchantCurveAt`, then quadratic, the same shape
 * Tribal Wars uses (level 19 gives 91, level 25 gives 235). Mirrors `game.MerchantsForLevel` on the server.
 */
export function merchantsForLevel(level: number): number {
  if (level <= 0) return 0;
  const n = TRADE_BALANCE.merchantCurveAt;
  if (level <= n) return level;
  const steps = level - n + 1;
  return steps * steps + n - 1;
}

export function merchantsTotal(v: Village): number {
  return merchantsForLevel(v.buildings.market);
}

export function merchantsInTransit(state: GameState, villageId: string): number {
  let n = 0;
  for (const c of state.commands) {
    if (c.type !== 'trade') continue;
    const home = c.returning ? c.targetId : c.originId;
    if (home === villageId) n += c.merchants ?? 0;
  }
  // An offer on the board holds the merchants that will carry it, so posting costs merchants before anyone accepts.
  for (const o of state.tradeOffers) {
    if (o.originId === villageId) n += reservedMerchants(o);
  }
  return n;
}

/** The merchants an offer holds, from its own count or — for a post made before that was booked — from its give side. */
export function reservedMerchants(o: TradeOffer): number {
  if (o.reserved) return o.reserved;
  return merchantsNeeded(o.give);
}

export function merchantsAvailable(state: GameState, v: Village): number {
  return Math.max(0, merchantsTotal(v) - merchantsInTransit(state, v.id));
}

export function merchantsNeeded(amount: ResourceSet): number {
  return Math.ceil((amount.wood + amount.clay + amount.iron) / COMMAND_BALANCE.merchantCapacity);
}

/** Merchants behave like troops: they depend on unit speed, not world speed. */
export function tradeTravelMs(origin: Village, target: Village, unitSpeed: number): number {
  const d = distance(origin.x, origin.y, target.x, target.y);
  return Math.max(1, Math.round((d * COMMAND_BALANCE.merchantSpeed * MS_PER_MINUTE) / unitSpeed));
}

/** Travel time between two tile positions, for offers where only coordinates are known (e.g. other players' villages). */
export function tradeTravelMsByCoords(from: { x: number; y: number }, to: { x: number; y: number }, unitSpeed: number): number {
  const d = distance(from.x, from.y, to.x, to.y);
  return Math.max(1, Math.round((d * COMMAND_BALANCE.merchantSpeed * MS_PER_MINUTE) / unitSpeed));
}

export type TradeBlocker =
  | { kind: 'noMarket' }
  | { kind: 'notOwned' }
  | { kind: 'self' }
  | { kind: 'empty' }
  | { kind: 'resources' }
  /** The haul needs more merchants than the village has free right now (some are already on the road). */
  | { kind: 'merchants'; needed: number; available: number }
  /** The haul needs more merchants than the market will ever have: its level is the merchant count. */
  | { kind: 'marketTooSmall'; needed: number; total: number }
  | { kind: 'tradingOff' };

export function checkTrade(state: GameState, originId: string, targetId: string, amount: ResourceSet): { blockers: TradeBlocker[]; merchants: number; travelMs: number } {
  const origin = state.villages[originId];
  const target = state.villages[targetId];
  const blockers: TradeBlocker[] = [];
  if (!origin) throw new Error(`no village ${originId}`);
  if (origin.buildings.market < 1) blockers.push({ kind: 'noMarket' });
  if (!target || !isHumanVillage(target)) blockers.push({ kind: 'notOwned' });
  if (originId === targetId) blockers.push({ kind: 'self' });
  const clean = cleanAmount(amount);
  const total = clean.wood + clean.clay + clean.iron;
  if (total <= 0) blockers.push({ kind: 'empty' });
  if (clean.wood > origin.resources.wood || clean.clay > origin.resources.clay || clean.iron > origin.resources.iron) blockers.push({ kind: 'resources' });
  const merchants = merchantsNeeded(clean);
  const available = merchantsAvailable(state, origin);
  if (merchants > merchantsTotal(origin)) {
    // A bigger market would be needed: raising its level is the only fix, waiting will not help.
    blockers.push({ kind: 'marketTooSmall', needed: merchants, total: merchantsTotal(origin) });
  } else if (merchants > available) {
    // Enough merchants in total, but some are still on the road.
    blockers.push({ kind: 'merchants', needed: merchants, available });
  }
  const travelMs = target ? tradeTravelMs(origin, target, state.config.unitSpeed) : 0;
  return { blockers, merchants, travelMs };
}

function cleanAmount(a: ResourceSet): ResourceSet {
  return { wood: Math.max(0, Math.floor(a.wood || 0)), clay: Math.max(0, Math.floor(a.clay || 0)), iron: Math.max(0, Math.floor(a.iron || 0)) };
}

/** Surplus = resources above `ratio` of warehouse capacity. */
export function surplus(v: Village, ratio = 0.8): ResourceSet {
  const cap = villageStorageCapacity(v);
  const out = { wood: 0, clay: 0, iron: 0 };
  for (const r of RESOURCES) out[r] = Math.max(0, Math.floor(v.resources[r] - cap * ratio));
  return out;
}

/** "Split evenly": sends half of origin's surplus over target for each resource. */
export function balanceAmount(origin: Village, target: Village): ResourceSet {
  const out = { wood: 0, clay: 0, iron: 0 };
  for (const r of RESOURCES) out[r] = Math.max(0, Math.floor((origin.resources[r] - target.resources[r]) / 2));
  return out;
}

export function canAcceptTrader(state: GameState, v: Village): boolean {
  // The travelling trader is a fixed-rate shop, not a deal with another player, so `tradingEnabled` does not close
  // it: the same reason `AcceptTrader` on the server stopped checking the switch.
  const t = state.trader;
  return !!t && !t.accepted && v.buildings.market >= 1 && v.resources[t.offer.give] >= t.offer.giveAmount;
}
