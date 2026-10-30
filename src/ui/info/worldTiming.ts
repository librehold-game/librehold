import { UNITS } from '../../data/units.data';
import { buildTimeMs } from '../../engine/buildings';
import { checkMint } from '../../engine/conquest';
import { modifiersFor } from '../../engine/modifiers';
import { effectiveBuildingLevels } from '../../engine/queues';
import { researchTimeMs } from '../../engine/research';
import { recruitTimeMs, unitCost } from '../../engine/units';
import type { GameState, Village } from '../../engine/types';
import type { Timing } from '../../shared/encyclopedia/facts';

/** What the village's own screens would charge and take: world speed, side modifiers, Headquarters level, the world's noble rules. */
export function worldTiming(game: GameState, v: Village): Timing {
  const cfg = game.config;
  const main = effectiveBuildingLevels(v).main;
  return {
    worldSpeed: cfg.worldSpeed,
    buildMs: (id, level) => Math.round(buildTimeMs(id, level, main, cfg.worldSpeed) / modifiersFor(cfg, v).building),
    recruitMs: (unit) => recruitTimeMs(unit, v.buildings[UNITS[unit].building], cfg, v),
    researchMs: (unit, level) => researchTimeMs(unit, level, cfg.worldSpeed),
    unitCost: (unit) => unitCost(unit, 1, cfg),
    coinCost: cfg.noCoins ? null : checkMint(game, v).cost,
  };
}
