import { readdirSync, readFileSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const root = resolve(__dirname, '..');
// Engine functions that read the world seed: the client has no seed, so they would invent a different map.
const SEEDED = ['tileHasVillage', 'barbarianTemplate', 'tileInfo', 'villageExistsAt', 'ensureVillage', 'villagesInRadius', 'generatedVillagesNear'];

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name);
    return e.isDirectory() ? files(p) : /\.(ts|tsx)$/.test(e.name) ? [p] : [];
  });
}

describe('the client does not compute the map from the world seed', () => {
  it('src/ui, src/state and src/net import no functions that read the seed', () => {
    const offenders: string[] = [];
    for (const dir of ['src/ui', 'src/state', 'src/net']) {
      for (const file of files(join(root, dir))) {
        for (const m of readFileSync(file, 'utf8').matchAll(/import\s*(?:type\s*)?\{([^}]*)\}\s*from\s*['"][^'"]*engine\/[^'"]*['"]/g)) {
          for (const name of m[1]!.split(',').map((s) => s.trim().replace(/^type\s+/, '').split(/\s+as\s+/)[0]!)) {
            if (SEEDED.includes(name)) offenders.push(`${relative(root, file)}: ${name}`);
          }
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
