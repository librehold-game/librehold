import { describe, it, expect } from 'vitest';
import { hashSeed } from '../src/engine/rng';
import { BIOME, biomeAt, biomeGrid, rawVertex, type Biome, type BiomeGrid } from '../src/ui/map/biomes';

const SEED = 12345;
const noVillages = () => false;

function neighbours(g: BiomeGrid, i: number, j: number): Biome[] {
  const out: Biome[] = [];
  for (let dj = -1; dj <= 1; dj++) {
    for (let di = -1; di <= 1; di++) {
      const ii = i + di, jj = j + dj;
      if ((di || dj) && ii >= 0 && jj >= 0 && ii < g.w && jj < g.h) out.push(g.biome[jj * g.w + ii] as Biome);
    }
  }
  return out;
}

describe('biome field', () => {
  it('matches preview.py, the renderer of the approved preview', () => {
    // From art/pixel-spike/map/preview.py with WARP = 1.2: terrain(12345, vx / 3, vy / 2, True).
    const expected: [number, number, Biome][] = [
      [15, 350, BIOME.forest], [40, 360, BIOME.water], [30, 344, BIOME.grass],
      [47, 348, BIOME.mountain], [37, 362, BIOME.water], [23, 350, BIOME.forest],
    ];
    for (const [vx, vy, b] of expected) expect(rawVertex(SEED, vx, vy).biome, `${vx},${vy}`).toBe(b);
  });

  for (const [label, isVillage] of [['no villages', noVillages], ['a village', (x: number, y: number) => x === 20 && y === 13]] as const) {
    it(`windows agree on shared vertices (${label})`, () => {
      const a = biomeGrid(SEED, 30, 20, 40, 30, isVillage);
      const b = biomeGrid(SEED, 50, 34, 40, 30, isVillage);
      for (let vy = 34; vy < 50; vy++) {
        for (let vx = 50; vx < 70; vx++) {
          expect(biomeAt(a, vx, vy), `${vx},${vy}`).toBe(biomeAt(b, vx, vy));
          expect(a.sparse[(vy - 20) * a.w + vx - 30], `sparse ${vx},${vy}`).toBe(b.sparse[(vy - 34) * b.w + vx - 50]);
        }
      }
    });
  }

  it('a village clears a rounded meadow: its cross of vertices, not the tile corners', () => {
    let tile: [number, number] | null = null;
    for (let ty = 0; ty < 100 && !tile; ty++) {
      for (let tx = 0; tx < 60 && !tile; tx++) {
        let deep = true;
        for (let ly = 0; ly <= 2; ly++) for (let lx = 0; lx <= 3; lx++) deep &&= rawVertex(SEED, 3 * tx + lx, 2 * ty + ly).biome === BIOME.forest;
        if (deep) tile = [tx, ty];
      }
    }
    expect(tile).not.toBeNull();
    const [tx, ty] = tile!;
    const g = biomeGrid(SEED, 3 * tx - 5, 2 * ty - 5, 14, 13, (x, y) => x === tx && y === ty);
    for (let ly = 0; ly <= 2; ly++) {
      for (let lx = 0; lx <= 3; lx++) {
        const vx = 3 * tx + lx, vy = 2 * ty + ly;
        const corner = (lx === 0 || lx === 3) && ly !== 1;
        expect(biomeAt(g, vx, vy), `${lx},${ly}`).toBe(corner ? BIOME.forest : BIOME.grass);
      }
    }
  });

  const big = biomeGrid(SEED, 0, 0, 300, 200, noVillages);

  it('biomes never touch: forest only borders meadow or forest, mountains never border water', () => {
    for (let j = 0; j < big.h; j++) {
      for (let i = 0; i < big.w; i++) {
        const b = big.biome[j * big.w + i];
        const n = neighbours(big, i, j);
        if (b === BIOME.forest) expect(n.some((x) => x === BIOME.water || x === BIOME.mountain), `${i},${j}`).toBe(false);
        if (b === BIOME.mountain) expect(n.includes(BIOME.water), `${i},${j}`).toBe(false);
      }
    }
  });

  it('biome shares stay close to the approved preview (water 6, mountain 8, forest 24 %)', () => {
    const share = (b: Biome) => big.biome.filter((x) => x === b).length / big.biome.length;
    expect(share(BIOME.water)).toBeGreaterThanOrEqual(0.03);
    expect(share(BIOME.water)).toBeLessThanOrEqual(0.1);
    expect(share(BIOME.mountain)).toBeGreaterThanOrEqual(0.04);
    expect(share(BIOME.mountain)).toBeLessThanOrEqual(0.12);
    expect(share(BIOME.forest)).toBeGreaterThanOrEqual(0.15);
    expect(share(BIOME.forest)).toBeLessThanOrEqual(0.35);
  });

  it('no lone specks: every lake, forest and mountain vertex has a side neighbour of its biome', () => {
    const dense = (x: number, y: number) => hashSeed(1, x, y) % 10 < 3;
    const g = biomeGrid(SEED, 0, 0, 300, 200, dense);
    for (let j = 1; j < g.h - 1; j++) {
      for (let i = 1; i < g.w - 1; i++) {
        const b = g.biome[j * g.w + i];
        if (b === BIOME.grass) continue;
        const side = [g.biome[j * g.w + i - 1], g.biome[j * g.w + i + 1], g.biome[(j - 1) * g.w + i], g.biome[(j + 1) * g.w + i]];
        expect(side.includes(b), `${i},${j}`).toBe(true);
      }
    }
  });

  it('windows agree among dense villages', () => {
    const dense = (x: number, y: number) => hashSeed(1, x, y) % 10 < 3;
    const a = biomeGrid(SEED, 30, 20, 40, 30, dense);
    const b = biomeGrid(SEED, 50, 34, 40, 30, dense);
    for (let vy = 34; vy < 50; vy++) for (let vx = 50; vx < 70; vx++) expect(biomeAt(a, vx, vy), `${vx},${vy}`).toBe(biomeAt(b, vx, vy));
  });

  it('the grid remembers which vertices a village cleared', () => {
    const g = biomeGrid(SEED, 50, 20, 20, 15, (x, y) => x === 20 && y === 13);
    const cleared = (vx: number, vy: number) => g.cleared[(vy - 20) * g.w + vx - 50];
    expect(cleared(61, 26)).toBe(1);
    expect(cleared(60, 27)).toBe(1);
    expect(cleared(60, 26)).toBe(0);
    expect(cleared(55, 22)).toBe(0);
  });

  it('no sparse trees on the beach', () => {
    for (let j = 0; j < big.h; j++) {
      for (let i = 0; i < big.w; i++) {
        if (big.sparse[j * big.w + i]) expect(neighbours(big, i, j).includes(BIOME.water), `${i},${j}`).toBe(false);
      }
    }
  });
});
