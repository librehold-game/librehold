import { hashSeed, mulberry32 } from './rng';
import { BIOME, tileBiome, type Biome } from './biomes';
import { valueNoise } from './noise';

/**
 * The classic skin's minimap: one pixel per map field (at most 200 a side), coloured like the pixel terrain. Colours
 * are the mean colours of the map art (`npm run assets:pixel-map`): forests get a dark edge, mountains rock with
 * darker peaks, the meadow the dry stretches of its "tone" noise. Lakes have no shore line: at one pixel per field it
 * would swallow the small ones.
 */
export const MINIMAP_PALETTE = {
  meadow: '#607b33',
  dry: '#8a8040',
  water: '#234593',
  forest: '#376235',
  forestEdge: '#24452a',
  rock: '#7b828a',
  peak: '#455063',
} as const;

const MAX_SIZE = 200;

const rgb = (hex: string) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [number, number, number];

export function minimapPixels(seed: number, mapSize: number): { size: number; data: Uint8ClampedArray<ArrayBuffer> } {
  const P = MINIMAP_PALETTE;
  const size = Math.min(mapSize, MAX_SIZE);
  const tile = (i: number) => Math.floor((i * mapSize) / size);
  const biome = new Uint8Array(size * size);
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) biome[y * size + x] = tileBiome(seed, tile(x), tile(y));
  const at = (x: number, y: number) => biome[Math.min(size - 1, Math.max(0, y)) * size + Math.min(size - 1, Math.max(0, x))]!;
  const edge = (x: number, y: number, b: number) => at(x - 1, y) !== b || at(x + 1, y) !== b || at(x, y - 1) !== b || at(x, y + 1) !== b;
  const data = new Uint8ClampedArray(size * size * 4);
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const tx = tile(x), ty = tile(y);
      const b = biome[y * size + x] as Biome;
      let color: string;
      if (b === BIOME.water) color = P.water;
      else if (b === BIOME.forest) color = edge(x, y, b) ? P.forestEdge : P.forest;
      else if (b === BIOME.mountain) {
        const r = mulberry32(hashSeed(seed, 'minimap', tx, ty))();
        color = r < 0.3 ? P.peak : P.rock;
      } else color = valueNoise(seed, 'tone', tx + 0.5, ty + 0.5, 2.5) > 0.6 ? P.dry : P.meadow;
      data.set([...rgb(color), 255], (y * size + x) * 4);
    }
  }
  return { size, data };
}

const images = new Map<string, string>();

/** The minimap as a PNG data URL, cached per world; null where there is no canvas (tests). */
export function minimapImage(seed: number, mapSize: number): string | null {
  const key = `${seed}|${mapSize}`;
  const hit = images.get(key);
  if (hit) return hit;
  const { size, data } = minimapPixels(seed, mapSize);
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx || typeof canvas.toDataURL !== 'function') return null;
  ctx.putImageData(new ImageData(data, size, size), 0, 0);
  const url = canvas.toDataURL('image/png');
  images.set(key, url);
  return url;
}
