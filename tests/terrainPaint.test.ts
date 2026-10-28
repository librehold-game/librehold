import { describe, it, expect } from 'vitest';
import { BIOME, biomeAt, biomeGrid } from '../src/ui/map/biomes';
import { CELL, groundPlacements, spritePlacements, wangCell } from '../src/ui/map/terrainPaint';

const SEED = 12345;
const { grass, water, forest, mountain } = BIOME;

describe('Wang cell', () => {
  it('a meadow cell has no set', () => expect(wangCell(grass, grass, grass, grass)).toEqual({ set: null, mask: 0 }));
  it('one water corner in the north-west', () => expect(wangCell(water, grass, grass, grass)).toEqual({ set: 'water', mask: 8 }));
  it('forest on three corners', () => expect(wangCell(forest, forest, grass, forest)).toEqual({ set: 'forest', mask: 13 }));
  it('the higher biome wins a mixed cell', () => expect(wangCell(mountain, forest, grass, grass)).toEqual({ set: 'mountain', mask: 8 }));
});

describe('sprite placement', () => {
  const g = biomeGrid(SEED, 0, 300, 120, 80, () => false);
  const sprites = spritePlacements(SEED, g, 2, 302, 116, 76);
  const vertexOf = (s: { x: number; y: number }) => ({ vx: Math.round(s.x / CELL), vy: Math.round(s.y / CELL) });

  it('peaks stand on mountains, trees on forest or sparse meadow', () => {
    expect(sprites.length).toBeGreaterThan(100);
    for (const s of sprites) {
      const { vx, vy } = vertexOf(s);
      const b = biomeAt(g, vx, vy);
      if (s.kind === 'peak') expect(b, `${vx},${vy}`).toBe(mountain);
      else expect(b === forest || (b === grass && g.sparse[(vy - g.vy0) * g.w + vx - g.vx0] === 1), `${vx},${vy}`).toBe(true);
    }
  });

  it('is sorted by y and deterministic', () => {
    for (let i = 1; i < sprites.length; i++) expect(sprites[i]!.y).toBeGreaterThanOrEqual(sprites[i - 1]!.y);
    expect(spritePlacements(SEED, g, 2, 302, 116, 76)).toEqual(sprites);
  });

  it('peaks cover about 45 % of mountain vertices', () => {
    let mountains = 0;
    for (let vy = 302; vy < 378; vy++) for (let vx = 2; vx < 118; vx++) if (biomeAt(g, vx, vy) === mountain) mountains++;
    const peaks = sprites.filter((s) => s.kind === 'peak').length;
    expect(mountains).toBeGreaterThan(50);
    expect(peaks / mountains).toBeGreaterThanOrEqual(0.35);
    expect(peaks / mountains).toBeLessThanOrEqual(0.55);
  });

  it('two windows agree on the sprites of their shared vertices', () => {
    const h = biomeGrid(SEED, 40, 320, 120, 80, () => false);
    const shared = (list: typeof sprites) => list.filter((s) => {
      const { vx, vy } = vertexOf(s);
      return vx >= 42 && vx < 118 && vy >= 322 && vy < 378;
    });
    expect(shared(spritePlacements(SEED, h, 42, 322, 116, 76))).toEqual(shared(sprites));
  });
});

describe('meadow variety', () => {
  const village = (x: number, y: number) => (x * 7 + y * 13) % 4 === 0;
  const g = biomeGrid(SEED, 0, 300, 160, 100, village);
  const at = (a: Uint8Array, vx: number, vy: number) => a[(vy - g.vy0) * g.w + vx - g.vx0];
  const meadowAround = (vx: number, vy: number) => {
    for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) if (biomeAt(g, vx + dx, vy + dy) !== grass) return false;
    return true;
  };
  const { decals, decor } = groundPlacements(SEED, g, 2, 302, 156, 96);

  it('decals and decorations lie only on meadow away from other biomes', () => {
    expect(decals.length).toBeGreaterThan(100);
    expect(decor.length).toBeGreaterThan(30);
    for (const d of decals) expect(meadowAround(d.vx, d.vy), `decal ${d.vx},${d.vy}`).toBe(true);
    for (const s of decor) {
      const vx = Math.round(s.x / CELL), vy = Math.round(s.y / CELL);
      expect(meadowAround(vx, vy), `decor ${vx},${vy}`).toBe(true);
      expect(at(g.cleared, vx, vy), `decor in a village clearing ${vx},${vy}`).toBe(0);
    }
  });

  it('dry stretches, flower clumps and bare dirt all occur', () => {
    const kinds = new Set(decals.map((d) => d.kind));
    expect([...kinds].sort()).toEqual(['dirt', 'dry', 'flower']);
  });

  it('two windows agree on the decals and decorations of their shared vertices', () => {
    const h = biomeGrid(SEED, 40, 320, 160, 100, village);
    const other = groundPlacements(SEED, h, 42, 322, 156, 96);
    const inShared = (vx: number, vy: number) => vx >= 42 && vx < 158 && vy >= 322 && vy < 398;
    expect(other.decals.filter((d) => inShared(d.vx, d.vy))).toEqual(decals.filter((d) => inShared(d.vx, d.vy)));
    const decorIn = (list: typeof decor) => list.filter((s) => inShared(Math.round(s.x / CELL), Math.round(s.y / CELL)));
    expect(decorIn(other.decor)).toEqual(decorIn(decor));
  });

  it('decorations are sorted by y like the other sprites', () => {
    for (let i = 1; i < decor.length; i++) expect(decor[i]!.y).toBeGreaterThanOrEqual(decor[i - 1]!.y);
  });
});
