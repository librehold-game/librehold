/**
 * Biome field of the pixel map (classic skin). Terrain vertices sit on a SUB_X × SUB_Y sub-grid per map tile and
 * sample the game's terrain noise at a domain-warped point, so lakes and forests get organic outlines instead of
 * following tile borders. Same rules as `art/pixel-spike/map/preview.py`, the renderer of the approved preview.
 * Drawing only: `terrainAt` stays the game's terrain.
 */
import { TERRAIN_BALANCE, valueNoise } from './noise';

export const SUB_X = 3, SUB_Y = 2;

export const BIOME = { grass: 0, water: 1, forest: 2, mountain: 3 } as const;
export type Biome = (typeof BIOME)[keyof typeof BIOME];

/** Higher first: a lower biome never touches a higher one. */
export const BIOME_PRIORITY: readonly Biome[] = [BIOME.water, BIOME.mountain, BIOME.forest];

export const BIOME_BALANCE = { warp: 1.2, warpScale: 2.5, warpDetailScale: 0.9, warpDetail: 0.25, mountainScale: 6, mountainThreshold: 0.8 } as const;

/** The rules below chain four one-vertex neighbourhoods, so a window padded by this gives the same vertices wherever it starts. */
const PAD = 4;

/** vx, vy are global vertex indices; the noise point is (vx / SUB_X, vy / SUB_Y). */
export function rawVertex(seed: number, vx: number, vy: number): { biome: Biome; sparse: boolean } {
  const B = BIOME_BALANCE, T = TERRAIN_BALANCE;
  const x = vx / SUB_X, y = vy / SUB_Y;
  const n = (tag: string, scale: number) => valueNoise(seed, tag, x, y, scale) - 0.5;
  const wx = x + B.warp * n('warpx', B.warpScale) + B.warpDetail * B.warp * n('warpx2', B.warpDetailScale);
  const wy = y + B.warp * n('warpy', B.warpScale) + B.warpDetail * B.warp * n('warpy2', B.warpDetailScale);
  if (valueNoise(seed, 'water', wx, wy, T.waterScale) > T.waterThreshold) return { biome: BIOME.water, sparse: false };
  if (valueNoise(seed, 'mountain', wx, wy, B.mountainScale) > B.mountainThreshold) return { biome: BIOME.mountain, sparse: false };
  const f = 0.65 * valueNoise(seed, 'forest', wx, wy, T.forestScale) + 0.35 * valueNoise(seed, 'forest2', wx, wy, T.forestDetailScale);
  if (f > T.forestThreshold) return { biome: BIOME.forest, sparse: false };
  return { biome: BIOME.grass, sparse: f > T.sparseThreshold };
}

/** `cleared` marks the vertices of village clearings. */
export interface BiomeGrid { vx0: number; vy0: number; w: number; h: number; biome: Uint8Array; sparse: Uint8Array; cleared: Uint8Array }

/** Map tiles that have (vx, vy) as a corner: one tile inside it, two on a tile border. */
function owners(v: number, sub: number): number[] {
  const t = Math.floor(v / sub);
  return v - t * sub === 0 ? [t - 1, t] : [t];
}

/**
 * A village clears the vertices its glyph covers: the middle row and the middle columns of its tile. The tile corners
 * keep their biome, so a clearing is rounded instead of a square cut out of a lake or a forest.
 */
function inClearing(lx: number, ly: number): boolean {
  return (lx > 0 && lx < SUB_X) || ly === SUB_Y / 2;
}

export function biomeGrid(seed: number, vx0: number, vy0: number, w: number, h: number, isVillage: (x: number, y: number) => boolean): BiomeGrid {
  const pw = w + 2 * PAD, ph = h + 2 * PAD;
  const biome = new Uint8Array(pw * ph), sparse = new Uint8Array(pw * ph), cleared = new Uint8Array(pw * ph);
  const villages = new Map<string, boolean>();
  const village = (x: number, y: number) => {
    const k = `${x}|${y}`;
    let v = villages.get(k);
    if (v === undefined) villages.set(k, (v = isVillage(x, y)));
    return v;
  };
  for (let j = 0; j < ph; j++) {
    for (let i = 0; i < pw; i++) {
      const vx = vx0 - PAD + i, vy = vy0 - PAD + j;
      if (owners(vx, SUB_X).some((tx) => owners(vy, SUB_Y).some((ty) => inClearing(vx - tx * SUB_X, vy - ty * SUB_Y) && village(tx, ty)))) {
        cleared[j * pw + i] = 1;
        continue;
      }
      const r = rawVertex(seed, vx, vy);
      biome[j * pw + i] = r.biome;
      sparse[j * pw + i] = r.sparse ? 1 : 0;
    }
  }
  const touches = (i: number, j: number, test: (b: number) => boolean) => {
    for (let jj = Math.max(0, j - 1); jj <= Math.min(ph - 1, j + 1); jj++) {
      for (let ii = Math.max(0, i - 1); ii <= Math.min(pw - 1, i + 1); ii++) if ((ii !== i || jj !== j) && test(biome[jj * pw + ii]!)) return true;
    }
    return false;
  };
  for (let j = 0; j < ph; j++) for (let i = 0; i < pw; i++) if (sparse[j * pw + i] && touches(i, j, (b) => b === BIOME.water)) sparse[j * pw + i] = 0;
  for (let p = 1; p < BIOME_PRIORITY.length; p++) {
    const b = BIOME_PRIORITY[p]!, higher = BIOME_PRIORITY.slice(0, p) as number[];
    for (let j = 0; j < ph; j++) {
      for (let i = 0; i < pw; i++) {
        if (biome[j * pw + i] === b && touches(i, j, (x) => higher.includes(x))) biome[j * pw + i] = BIOME.grass;
      }
    }
  }
  // A biome vertex alone on its sides would be drawn as a speck (a puddle, a rock chip) between clearings.
  for (let j = 0; j < ph; j++) {
    for (let i = 0; i < pw; i++) {
      const b = biome[j * pw + i]!;
      if (b === BIOME.grass) continue;
      const side = (ii: number, jj: number) => ii >= 0 && jj >= 0 && ii < pw && jj < ph && biome[jj * pw + ii] === b;
      if (!side(i - 1, j) && !side(i + 1, j) && !side(i, j - 1) && !side(i, j + 1)) biome[j * pw + i] = BIOME.grass;
    }
  }
  const inner = (a: Uint8Array) => {
    const out = new Uint8Array(w * h);
    for (let j = 0; j < h; j++) out.set(a.subarray((j + PAD) * pw + PAD, (j + PAD) * pw + PAD + w), j * w);
    return out;
  };
  return { vx0, vy0, w, h, biome: inner(biome), sparse: inner(sparse), cleared: inner(cleared) };
}

export function biomeAt(g: BiomeGrid, vx: number, vy: number): Biome {
  return g.biome[(vy - g.vy0) * g.w + vx - g.vx0] as Biome;
}

/** Label source for the hover title of an empty tile. */
export function tileBiome(seed: number, x: number, y: number): Biome {
  return rawVertex(seed, x * SUB_X + 1, y * SUB_Y + 1).biome;
}
