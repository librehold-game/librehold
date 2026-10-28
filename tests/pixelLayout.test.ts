import { describe, it, expect } from 'vitest';
import { LAYOUT, pixelUrl, ringY, roadKeys, stageFile, wallKind } from '../src/ui/scene/pixel/layout';
import { facing, loop, pathLength, pointAt, type Pt } from '../src/ui/scene/pixel/path';
import { BUILDING_IDS, type BuildingId } from '../src/engine/types';

const zero = Object.fromEntries(BUILDING_IDS.map((id) => [id, 0])) as Record<BuildingId, number>;

describe('pixel layout', () => {
  it('pixelUrl resolves an exported file and throws for a missing one', () => {
    expect(pixelUrl('buildings/main-3.png')).toMatch(/main-3/);
    expect(() => pixelUrl('buildings/nope.png')).toThrow(/Missing pixel art asset: buildings\/nope.png/);
  });

  it('stageFile falls back to the nearest stage: the statue only has stage 3', () => {
    expect(stageFile('statue', 1)).toBe(stageFile('statue', 3));
    expect(stageFile('main', 2)).toBe('buildings/main-2.png');
  });

  it('roadKeys: gate and rally always, building roads by level or construction, side roads by iron and farm', () => {
    expect(roadKeys(zero, []).sort()).toEqual(['gate', 'rally']);
    expect(roadKeys(zero, ['academy'])).toContain('academy');
    const keys = roadKeys({ ...zero, iron: 1, farm: 2, wood: 1, main: 3 }, []);
    expect(keys).toEqual(expect.arrayContaining(['side_w', 'side_e', 'wood', 'main']));
    expect(keys).not.toContain('clay');
    for (const k of roadKeys({ ...Object.fromEntries(BUILDING_IDS.map((id) => [id, 1])) } as Record<BuildingId, number>, [])) {
      expect(LAYOUT.roads[k], k).toBeDefined();
    }
  });

  it('a side road always brings the road that connects it to the plaza', () => {
    expect(roadKeys({ ...zero, farm: 1 }, [])).toEqual(expect.arrayContaining(['side_e', 'academy']));
    expect(roadKeys({ ...zero, iron: 1 }, [])).toEqual(expect.arrayContaining(['side_w', 'storage']));
  });

  it('wallKind: 0 → none, 1–2 → palisade, 3 → stone', () => {
    expect([0, 1, 2, 3].map((v) => wallKind(v as 0 | 1 | 2 | 3))).toEqual([null, 'palisade', 'palisade', 'stone']);
  });

  it('ringY gives cy ± ry on the vertical axis and cy outside the ellipse', () => {
    const { cx, cy, ry, rx } = LAYOUT.ring;
    expect(ringY(cx, 'north')).toBeCloseTo(cy - ry);
    expect(ringY(cx, 'south')).toBeCloseTo(cy + ry);
    expect(ringY(cx + rx + 10, 'south')).toBe(cy);
  });
});

describe('path geometry', () => {
  const line: Pt[] = [[0, 0], [3, 4], [3, 10]];

  it('pathLength and pointAt walk the polyline linearly, clamping the distance', () => {
    expect(pathLength(line)).toBe(11);
    expect(pointAt(line, 5)).toEqual([3, 4]);
    expect(pointAt(line, 8)).toEqual([3, 7]);
    expect(pointAt(line, -2)).toEqual([0, 0]);
    expect(pointAt(line, 99)).toEqual([3, 10]);
  });

  it('facing: right → east, down → south, 45° diagonal → south', () => {
    expect(facing([0, 0], [5, 0])).toBe('east');
    expect(facing([0, 0], [-5, 1])).toBe('west');
    expect(facing([0, 0], [0, 5])).toBe('south');
    expect(facing([0, 0], [0, -5])).toBe('north');
    expect(facing([0, 0], [5, 5])).toBe('south');
  });

  it('loop goes there and back without repeating the ends', () => {
    expect(loop([[0, 0], [1, 1], [2, 2]])).toEqual([[0, 0], [1, 1], [2, 2], [1, 1]]);
  });
});
