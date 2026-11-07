import { describe, it, expect } from 'vitest';
import { merchantsForLevel, merchantsTotal, merchantsAvailable } from '../src/engine/market';
import { normalizeDelivery } from '../src/engine/trade';
import { TRADE_BALANCE } from '../src/data/world.config';
import type { GameState } from '../src/engine/types';
import { activeVillage, newTestGame } from './helpers';

/** The published Tribal Wars market table: merchants per level, 1–25. */
const TW = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 14, 19, 26, 35, 46, 59, 74, 91, 110, 131, 154, 179, 206, 235];

describe('the merchant curve', () => {
  it('matches the Tribal Wars table level for level', () => {
    TW.forEach((want, i) => expect(merchantsForLevel(i + 1)).toBe(want));
  });

  it('gives one merchant per level up to the breakpoint, then accelerates', () => {
    const at = TRADE_BALANCE.merchantCurveAt;
    for (let level = 0; level <= at; level++) expect(merchantsForLevel(level)).toBe(level);
    expect(merchantsForLevel(at + 1)).toBeGreaterThan(at + 1);
  });

  it('reads the count off the village market level', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    v.buildings.market = 19;
    expect(merchantsTotal(v)).toBe(91);
    v.buildings.market = 0;
    expect(merchantsTotal(v)).toBe(0);
    expect(merchantsAvailable(g, v)).toBe(0);
  });

  it('subtracts the merchants already on the road, not the whole count', () => {
    const g: GameState = newTestGame();
    const v = activeVillage(g);
    v.buildings.market = 12; // 14 merchants
    g.commands = [
      { id: 'c1', type: 'trade', originId: v.id, targetId: 'v:0:0', units: {}, merchants: 5, departedAt: 0, arrivesAt: 1 },
      { id: 'c2', type: 'attack', originId: v.id, targetId: 'v:0:0', units: { spear: 1 }, departedAt: 0, arrivesAt: 1 },
    ];
    expect(merchantsTotal(v)).toBe(14);
    expect(merchantsAvailable(g, v)).toBe(9);
  });
});

describe('a posted delivery limit', () => {
  it('keeps "no limit" and clamps anything else to the band the server uses', () => {
    // Part of a deal's identity on the server, so the client has to normalize it the same way.
    expect(normalizeDelivery(0)).toBe(0);
    expect(normalizeDelivery(-5)).toBe(0);
    expect(normalizeDelivery(TRADE_BALANCE.minDeliveryMs)).toBe(TRADE_BALANCE.minDeliveryMs);
    expect(normalizeDelivery(TRADE_BALANCE.maxDeliveryMs)).toBe(TRADE_BALANCE.maxDeliveryMs);
    expect(normalizeDelivery(TRADE_BALANCE.maxDeliveryMs + 1)).toBe(TRADE_BALANCE.maxDeliveryMs);
    expect(normalizeDelivery(2 * TRADE_BALANCE.maxDeliveryMs)).toBe(TRADE_BALANCE.maxDeliveryMs);
  });
});
