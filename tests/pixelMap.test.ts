import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

/** `npm run assets:pixel-map` output: Wang atlases and the sprites of the map terrain layer. */

const DIR = join(__dirname, '..', 'src', 'assets', 'pixel', 'map');
const ATLASES = ['wang-water', 'wang-forest', 'wang-mountain'];
const VILLAGES = Array.from({ length: 6 }, (_, t) => [`village-${t}`, `village-${t}-barb`, `village-${t}-tint`, `village-${t}-roof`]).flat();
const FILES = [...ATLASES, 'tree-oak', 'tree-pine', ...Array.from({ length: 7 }, (_, i) => `peak-${i}`), ...VILLAGES, 'banner', 'banner-cloth',
  ...[0, 1, 2].map((i) => `decal-dry-${i}`), ...[0, 1].map((i) => `decal-dirt-${i}`),
  ...Array.from({ length: 5 }, (_, i) => `flower-${i}`), ...Array.from({ length: 12 }, (_, i) => `decor-${i}`)];

function pngSize(path: string): [number, number] {
  const b = readFileSync(path);
  return [b.readUInt32BE(16), b.readUInt32BE(20)];
}

describe('pixel map assets', () => {
  it('every atlas and sprite is exported', () => {
    const missing = FILES.filter((f) => !existsSync(join(DIR, `${f}.png`)));
    expect(missing).toEqual([]);
  });

  it('atlases are 4×4 cells of 32 px', () => {
    for (const a of ATLASES) expect(pngSize(join(DIR, `${a}.png`)), a).toEqual([128, 128]);
  });

  it('every village tier variant has the size of its tier', () => {
    for (let t = 0; t < 6; t++) {
      const size = pngSize(join(DIR, `village-${t}.png`));
      for (const v of ['barb', 'tint', 'roof']) expect(pngSize(join(DIR, `village-${t}-${v}.png`)), `${t} ${v}`).toEqual(size);
    }
  });

  it('villages.json records the size of every village tier and the banner', () => {
    const sizes = JSON.parse(readFileSync(join(DIR, 'villages.json'), 'utf8')) as Record<string, [number, number]>;
    for (let t = 0; t < 6; t++) expect(sizes[`village-${t}`], `tier ${t}`).toEqual(pngSize(join(DIR, `village-${t}.png`)));
    expect(sizes.banner).toEqual(pngSize(join(DIR, 'banner.png')));
  });

  it('the banner cloth mask fits the banner', () => {
    expect(pngSize(join(DIR, 'banner-cloth.png'))).toEqual(pngSize(join(DIR, 'banner.png')));
  });

  it('village tiers grow with points', () => {
    const areas = Array.from({ length: 6 }, (_, t) => pngSize(join(DIR, `village-${t}.png`))).map(([w, h]) => w * h);
    for (let t = 1; t < 6; t++) expect(areas[t]!, `tier ${t}`).toBeGreaterThan(areas[t - 1]!);
  });

  it('src/assets/pixel/map stays within 300 KB', () => {
    const total = readdirSync(DIR).reduce((t, f) => t + statSync(join(DIR, f)).size, 0);
    expect(total).toBeLessThanOrEqual(300 * 1024);
  });
});
