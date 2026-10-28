import { COMMAND_BALANCE } from '../data/world.config';
import { villageStorageCapacity } from './resources';
import { MS_PER_MINUTE, RESOURCES, type GameState, type ResourceSet, type Village } from './types';
import { distance } from './world';
import { isHumanVillage } from './players';

export function merchantsTotal(v: Village): number {
  return v.buildings.market;
}

export function merchantsInTransit(state: GameState, villageId: string): number {
  let n = 0;
  for (const c of state.commands) {
    if (c.type !== 'trade') continue;
    const home = c.returning ? c.targetId : c.originId;
    if (home === villageId) n += c.merchants ?? 0;
  }
  return n;
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

export type TradeBlocker =
  | { kind: 'noMarket' }
  | { kind: 'notOwned' }
  | { kind: 'self' }
  | { kind: 'empty' }
  | { kind: 'resources' }
  | { kind: 'merchants'; needed: number; available: number };

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
  if (merchants > available) blockers.push({ kind: 'merchants', needed: merchants, available });
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
  const t = state.trader;
  return !!t && !t.accepted && v.buildings.market >= 1 && v.resources[t.offer.give] >= t.offer.giveAmount;
}
