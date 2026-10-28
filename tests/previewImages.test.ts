import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { BUILDING_IDS, UNIT_IDS } from '../src/engine/types';

const OG = join(__dirname, '..', 'public', 'account', 'og');
const PNG_SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const files = ['home.png', ...UNIT_IDS.map((u) => `unit-${u}.png`), ...BUILDING_IDS.map((b) => `building-${b}.png`)];

describe('preview images', () => {
  it('cover the front page, every unit and every building', () => {
    expect(files).toHaveLength(28);
    for (const f of files) expect(existsSync(join(OG, f)), f).toBe(true);
  });

  it.each(files)('%s is a 1200×630 PNG of at most 300 kB', (f) => {
    const data = readFileSync(join(OG, f));
    expect(data.subarray(0, 8).equals(PNG_SIGNATURE)).toBe(true);
    expect(data.readUInt32BE(16)).toBe(1200);
    expect(data.readUInt32BE(20)).toBe(630);
    expect(data.length).toBeLessThanOrEqual(300_000);
  });
});
