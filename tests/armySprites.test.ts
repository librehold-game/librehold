import { describe, it, expect } from 'vitest';
import { UNIT_IDS } from '../src/engine/types';
import { ARMY, armySprite, boatSprite, groundImage, standIn } from '../src/ui/map/armySprites';

describe('army sprites', () => {
  it('every unit and the generic groups resolve to a sprite with four directions', () => {
    for (const id of [...UNIT_IDS, 'torch', 'shield'] as const) {
      const s = armySprite(id);
      for (const dir of ['south', 'east', 'north', 'west'] as const) expect(s.file[dir], `${id} ${dir}`).toBeTruthy();
      expect(s.frames).toBeGreaterThanOrEqual(1);
    }
  });

  it('every unit and both generic groups have their own marching art', () => {
    expect(UNIT_IDS.filter((u) => !ARMY.units[u])).toEqual([]);
    expect(ARMY.torch && ARMY.shield && ARMY.boat).toBeTruthy();
    expect(Object.keys(ARMY.ground).sort()).toEqual(['forest', 'meadow', 'mountain', 'water']);
  });

  it('a unit without its own walk uses the spearman stand-in', () => {
    const missing = UNIT_IDS.find((u) => !ARMY.units[u]);
    if (missing) expect(armySprite(missing)).toEqual(standIn());
    if (!ARMY.torch) expect(armySprite('torch')).toEqual(standIn());
  });

  it('boat and ground are optional', () => {
    expect(boatSprite() === null || boatSprite()!.frames >= 1).toBe(true);
    for (const g of ['meadow', 'forest', 'mountain', 'water'] as const) {
      const img = groundImage(g);
      expect(img === null || typeof img === 'string').toBe(true);
      expect(img !== null).toBe(!!ARMY.ground[g]);
    }
  });
});
