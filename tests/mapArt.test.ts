import { describe, it, expect } from 'vitest';
import { villageSize } from '../src/ui/map/MapArt';
import { MAP } from '../src/data/world.config';

describe('village tier on the map', () => {
  it('point thresholds match the world config', () => {
    expect(villageSize(0)).toBe(0);
    expect(villageSize(MAP.villageSizePoints[0]! - 1)).toBe(0);
    for (let i = 0; i < MAP.villageSizePoints.length; i++) {
      expect(villageSize(MAP.villageSizePoints[i]!)).toBe(Math.min(5, i + 1));
    }
    expect(villageSize(10_000_000)).toBe(5);
  });
});
