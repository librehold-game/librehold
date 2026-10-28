import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import * as oldRng from '../src/engine/rng';
import * as oldMinimap from '../src/ui/map/minimap';
import * as sharedRng from '../src/shared/terrain/rng';
import { minimapPixels } from '../src/shared/terrain/minimap';
import { ONLINE_TERRAIN_SEED } from '../src/shared/terrain/seed';

// Taken from src/ui/map/minimap.ts before the terrain code moved to src/shared/terrain.
const DIGESTS: Record<number, string> = {
  40: '9db5c72d5c19728cfd753179487fbcf321539123c9f0c57a4201b47999f79fdc',
  200: '296075be013499689e73c59942f60dd7a8d67b2687be5d27749341bb5d4e55a6',
  500: '82ebb97211f3e11970e521eef0d821d83b7c9043171a0a49521c347d6d922e5e',
};

describe('shared terrain', () => {
  it('the shared minimap matches the old one byte for byte', () => {
    for (const [size, digest] of Object.entries(DIGESTS)) {
      const data = minimapPixels(ONLINE_TERRAIN_SEED, Number(size)).data;
      expect(createHash('sha256').update(data).digest('hex'), `map ${size}`).toBe(digest);
    }
  });

  it('the old paths re-export the shared code', () => {
    expect(oldMinimap.minimapPixels).toBe(minimapPixels);
    expect(oldRng.hashSeed).toBe(sharedRng.hashSeed);
  });

  it('online worlds draw terrain with the shared seed', () => {
    expect(ONLINE_TERRAIN_SEED).toBe(0);
    const src = readFileSync(join(__dirname, '..', 'src', 'net', 'toGameState.ts'), 'utf8');
    expect(src).toContain('worldSeed: ONLINE_TERRAIN_SEED');
    expect(src).not.toMatch(/worldSeed:\s*0\b/);
  });
});
