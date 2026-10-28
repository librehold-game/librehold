import { describe, it, expect } from 'vitest';
import { BIOME, tileBiome } from '../src/ui/map/biomes';
import { MINIMAP_PALETTE as P, minimapPixels } from '../src/ui/map/minimap';

const hex = (data: Uint8ClampedArray, i: number) => `#${[0, 1, 2].map((k) => data[i * 4 + k]!.toString(16).padStart(2, '0')).join('')}`;

const ALLOWED = {
  [BIOME.grass]: [P.meadow, P.dry],
  [BIOME.water]: [P.water],
  [BIOME.forest]: [P.forest, P.forestEdge],
  [BIOME.mountain]: [P.rock, P.peak],
};

describe('pixel minimap', () => {
  it('has one sample per field, at most 200 a side', () => {
    expect(minimapPixels(777, 100).size).toBe(100);
    expect(minimapPixels(777, 200).size).toBe(200);
    expect(minimapPixels(777, 400).size).toBe(200);
  });

  const { size, data } = minimapPixels(777, 200);

  it('is fully opaque', () => {
    expect(data.length).toBe(size * size * 4);
    for (let i = 3; i < data.length; i += 4) expect(data[i]).toBe(255);
  });

  it("every pixel has a colour of its field's biome", () => {
    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) {
        const b = tileBiome(777, x, y);
        expect(ALLOWED[b], `${x}|${y}`).toContain(hex(data, y * size + x));
      }
    }
  });

  it('forests have an edge, mountains peaks, meadows dry stretches; small lakes keep all their water', () => {
    const used = new Set(Array.from({ length: size * size }, (_, i) => hex(data, i)));
    for (const c of [P.water, P.forest, P.forestEdge, P.meadow, P.dry, P.rock, P.peak]) expect(used, c).toContain(c);
  });

  it('is the same every time for a world', () => {
    expect(minimapPixels(777, 200).data).toEqual(data);
  });
});
