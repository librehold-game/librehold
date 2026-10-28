/**
 * The map's cosmetic terrain (forests, lakes): value noise from the world's seed, coherent patches instead of random
 * tiles. It has no effect on play: the engine counts villages and distances; the terrain is only drawn.
 */
import { hashSeed, mulberry32 } from './rng';

export type Terrain = 'plain' | 'sparse' | 'forest' | 'water';

export const TERRAIN_BALANCE = {
  forestScale: 4,
  forestDetailScale: 2,
  forestThreshold: 0.6,
  sparseThreshold: 0.52,
  waterScale: 5,
  waterThreshold: 0.83,
} as const;

function lattice(seed: number, tag: string, ix: number, iy: number): number {
  return mulberry32(hashSeed(seed, tag, ix, iy))();
}

const smooth = (t: number) => t * t * (3 - 2 * t);

/** Value noise in 0..1: bilinear interpolation between random grid nodes `scale` apart. */
export function valueNoise(seed: number, tag: string, x: number, y: number, scale: number): number {
  const fx = x / scale, fy = y / scale;
  const ix = Math.floor(fx), iy = Math.floor(fy);
  const tx = smooth(fx - ix), ty = smooth(fy - iy);
  const a = lattice(seed, tag, ix, iy), b = lattice(seed, tag, ix + 1, iy);
  const c = lattice(seed, tag, ix, iy + 1), d = lattice(seed, tag, ix + 1, iy + 1);
  return (a * (1 - tx) + b * tx) * (1 - ty) + (c * (1 - tx) + d * tx) * ty;
}

export function terrainAt(seed: number, x: number, y: number): Terrain {
  const T = TERRAIN_BALANCE;
  if (valueNoise(seed, 'water', x, y, T.waterScale) > T.waterThreshold) return 'water';
  const f = 0.65 * valueNoise(seed, 'forest', x, y, T.forestScale) + 0.35 * valueNoise(seed, 'forest2', x, y, T.forestDetailScale);
  if (f > T.forestThreshold) return 'forest';
  if (f > T.sparseThreshold) return 'sparse';
  return 'plain';
}

/** A drawing variant (0..n-1) and a horizontal flip, fixed per tile. */
export function tileVariant(seed: number, x: number, y: number, n: number): { variant: number; flip: boolean } {
  const h = hashSeed(seed, 'tilevar', x, y);
  return { variant: h % n, flip: (h >>> 8) % 2 === 1 };
}
