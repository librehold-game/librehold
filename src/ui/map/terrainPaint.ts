/**
 * What the pixel map draws for a biome field: a Wang cell per sub-grid square and the tree and peak sprites, in art
 * pixels (a map tile is SUB_X·CELL × SUB_Y·CELL). Placement is hashed from global vertex indices, so neighbouring
 * chunks agree on every sprite that crosses their border. Same rules as `art/pixel-spike/map/preview.py`.
 */
import { hashSeed, mulberry32 } from '../../engine/rng';
import { valueNoise } from './terrain';
import { BIOME, BIOME_PRIORITY, SUB_X, SUB_Y, biomeAt, type Biome, type BiomeGrid } from './biomes';

export const CELL = 32;

export type WangSet = 'water' | 'forest' | 'mountain';

const SET_OF: Partial<Record<Biome, WangSet>> = { [BIOME.water]: 'water', [BIOME.forest]: 'forest', [BIOME.mountain]: 'mountain' };

/** Mask bits: 8 NW, 4 NE, 2 SW, 1 SE, set where the corner is the cell's biome (the highest one present). */
export function wangCell(nw: Biome, ne: Biome, sw: Biome, se: Biome): { set: WangSet | null; mask: number } {
  const corners = [nw, ne, sw, se];
  const biome = BIOME_PRIORITY.find((b) => corners.includes(b));
  if (biome === undefined) return { set: null, mask: 0 };
  return { set: SET_OF[biome]!, mask: corners.reduce<number>((m, c, i) => (c === biome ? m | (8 >> i) : m), 0) };
}

export const TREES = ['oak', 'pine', 'oak'] as const;
export const PEAKS = 7;

/** Anchor in global art px; a sprite of size w×h is drawn at (x − w/2, y − h + h/5). */
export interface SpritePlacement { kind: 'tree' | 'peak' | 'decor'; index: number; x: number; y: number }

const PEAK_KEEP = 45;
const SPARSE_KEEP = 30;

/** Sprites of every vertex in [vx0, vx0 + w) × [vy0, vy0 + h), which must lie inside `g`, sorted by y, then x. */
export function spritePlacements(seed: number, g: BiomeGrid, vx0: number, vy0: number, w: number, h: number): SpritePlacement[] {
  const out: SpritePlacement[] = [];
  for (let vy = vy0; vy < vy0 + h; vy++) {
    for (let vx = vx0; vx < vx0 + w; vx++) {
      const b = biomeAt(g, vx, vy);
      const sparse = b === BIOME.grass && g.sparse[(vy - g.vy0) * g.w + vx - g.vx0] === 1;
      if (b !== BIOME.forest && b !== BIOME.mountain && !sparse) continue;
      const kind = b === BIOME.mountain ? 'peak' : 'tree';
      const density = b === BIOME.forest ? 2 : 1;
      const step = CELL / density;
      for (let a = 0; a < density; a++) {
        for (let c = 0; c < density; c++) {
          const hash = hashSeed(seed, 'spr', vx, vy, a, c);
          const keep = b === BIOME.mountain ? PEAK_KEEP : sparse ? SPARSE_KEEP : 100;
          if ((hash >>> 24) % 100 >= keep) continue;
          out.push({
            kind,
            index: (hash >>> 16) % (kind === 'peak' ? PEAKS : TREES.length),
            x: vx * CELL + a * step - CELL / 2 + step / 2 + (hash % 13) - 6,
            y: vy * CELL + c * step - CELL / 2 + step / 2 + ((hash >>> 8) % 11) - 5,
          });
        }
      }
    }
  }
  return out.sort((p, q) => p.y - q.y || p.x - q.x);
}

export const DECALS = { dry: 3, flower: 5, dirt: 2 } as const;
export const DECOR = 12;

/** A flat patch drawn on the ground under every sprite; x, y is its centre in global art px. */
export interface Decal { kind: keyof typeof DECALS; index: number; flip: boolean; vx: number; vy: number; x: number; y: number }

const meadowAround = (g: BiomeGrid, vx: number, vy: number) => {
  for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) if (biomeAt(g, vx + dx, vy + dy) !== BIOME.grass) return false;
  return true;
};

/**
 * Variety of the meadow for every vertex in [vx0, vx0 + w) × [vy0, vy0 + h), whose neighbours must lie inside `g`.
 * A low-frequency "tone" noise forms dry stretches (dry-grass decals) and flowery ones (flower clumps), with a little
 * bare dirt anywhere; a "decor" noise gathers bushes, boulders, stumps and haystacks into clusters. Nothing touches
 * another biome, and decorations stay out of village clearings.
 */
export function groundPlacements(seed: number, g: BiomeGrid, vx0: number, vy0: number, w: number, h: number): { decals: Decal[]; decor: SpritePlacement[] } {
  const rand = (tag: string, vx: number, vy: number) => mulberry32(hashSeed(seed, tag, vx, vy))();
  const decals: Decal[] = [], decor: SpritePlacement[] = [];
  for (let vy = vy0; vy < vy0 + h; vy++) {
    for (let vx = vx0; vx < vx0 + w; vx++) {
      if (!meadowAround(g, vx, vy)) continue;
      const tone = valueNoise(seed, 'tone', vx / SUB_X, vy / SUB_Y, 2.5);
      const r = rand('decal', vx, vy);
      const kind = tone > 0.6 && r < 0.75 ? 'dry' : tone < 0.32 && r < 0.45 ? 'flower' : r < 0.04 ? 'dirt' : null;
      if (kind) {
        decals.push({
          kind, vx, vy,
          index: Math.floor(rand('decalpick', vx, vy) * DECALS[kind]),
          flip: rand('decalflip', vx, vy) < 0.5,
          x: vx * CELL + Math.floor(rand('decalx', vx, vy) * 13) - 6,
          y: vy * CELL + Math.floor(rand('decaly', vx, vy) * 13) - 6,
        });
      }
      if (g.cleared[(vy - g.vy0) * g.w + vx - g.vx0]) continue;
      const cluster = Math.max(0, valueNoise(seed, 'decor', vx / SUB_X, vy / SUB_Y, 1.8) - 0.55) / 0.45;
      if (rand('decor', vx, vy) >= 0.04 + 0.5 * cluster) continue;
      decor.push({
        kind: 'decor',
        index: Math.floor(rand('decorpick', vx, vy) * DECOR),
        x: vx * CELL + Math.floor(rand('decorx', vx, vy) * 21) - 10,
        y: vy * CELL + Math.floor(rand('decory', vx, vy) * 17) - 8,
      });
    }
  }
  return { decals, decor: decor.sort((p, q) => p.y - q.y || p.x - q.x) };
}
