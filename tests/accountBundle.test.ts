import { createHash } from 'node:crypto';
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const root = resolve(__dirname, '..');
const forbiddenDirs = ['src/ui', 'src/engine', 'src/state', 'src/data', 'src/assets'];
/** Our own pixel art and the OFL fonts may be shown publicly; game code may not. */
const sharedAssets = ['src/assets/pixel/ui/', 'src/assets/pixel/map/', 'src/assets/pixel/account/', 'src/assets/fonts/'];
const isShared = (file: string) => sharedAssets.some((d) => file.startsWith(d));
/** The encyclopedia and the guides show the game's own numbers and pictures; nothing else may reach game code. */
const ALLOWED_FROM: Record<string, string[]> = {
  'src/account/encyclopedia/': ['src/data/units.data.ts', 'src/data/buildings.data.ts', 'src/data/conquest.data.ts',
    'src/data/balance.json', 'src/engine/buildings.ts', 'src/engine/types.ts', 'src/assets/pixel/army/', 'src/assets/pixel/village/buildings/',
    'src/assets/pixel/village/walls/'],
  'src/account/worldConfig.ts': ['src/data/world.config.ts', 'src/data/balance.json'],
  'src/account/content/': ['src/data/units.data.ts', 'src/data/buildings.data.ts', 'src/data/conquest.data.ts', 'src/data/balance.json',
    'src/engine/types.ts'],
};
const isForbidden = (file: string) => !isShared(file) && forbiddenDirs.some((d) => file.startsWith(`${d}/`));

function resolveImport(from: string, spec: string): string | null {
  if (!spec.startsWith('.')) return null;
  const base = resolve(dirname(from), spec);
  for (const candidate of [base, `${base}.ts`, `${base}.tsx`, join(base, 'index.ts'), join(base, 'index.tsx')]) {
    if (existsSync(candidate) && statSync(candidate).isFile()) return candidate;
  }
  throw new Error(`cannot resolve ${spec} from ${relative(root, from)}`);
}

/** Every file reached from the entry through static, side-effect and dynamic imports; imports out of files under stopAt
 * are not followed. */
function importGraph(entry: string, stopAt: string[] = []): string[] {
  const seen = new Set<string>();
  const queue = [entry];
  const pattern = /\bimport\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]|\bimport\(\s*['"]([^'"]+)['"]\s*\)/g;
  while (queue.length > 0) {
    const file = queue.pop()!;
    if (seen.has(file)) continue;
    seen.add(file);
    if (!/\.(ts|tsx)$/.test(file) || stopAt.some((d) => relative(root, file).startsWith(d))) continue;
    for (const m of readFileSync(file, 'utf8').matchAll(pattern)) {
      const target = resolveImport(file, (m[1] ?? m[2])!);
      if (target) queue.push(target);
    }
  }
  return [...seen].map((f) => relative(root, f));
}

function filesUnder(dir: string): string[] {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name);
    return e.isDirectory() ? filesUnder(p) : [p];
  });
}

const sha = (file: string) => createHash('sha256').update(readFileSync(file)).digest('hex');

describe('the account site is separate from the game', () => {
  const main = join(root, 'src/account/main.tsx');
  const reached = importGraph(main);

  it('game code is reached only from the encyclopedia and the guides', () => {
    const outside = importGraph(main, Object.keys(ALLOWED_FROM));
    expect(outside.length).toBeGreaterThan(5);
    for (const file of outside) expect(isForbidden(file), `${file} is game code`).toBe(false);
    const allowed = Object.values(ALLOWED_FROM).flat();
    for (const file of reached.filter(isForbidden)) {
      expect(allowed.some((a) => file === a || (a.endsWith('/') && file.startsWith(a))), `${file} is not allowed`).toBe(true);
    }
  });

  it('the account site reaches terrain only through src/shared', () => {
    expect(reached).toContain('src/shared/terrain/minimap.ts');
    expect(reached.filter((f) => f.startsWith('src/ui/map/') || f === 'src/engine/rng.ts')).toEqual([]);
  });

  it('stylesheets reach only the account site and the shared pixel art and fonts', () => {
    for (const css of filesUnder(join(root, 'src/account')).filter((f) => f.endsWith('.css'))) {
      for (const m of readFileSync(css, 'utf8').matchAll(/url\(\s*['"]?([^'")]+)['"]?\s*\)|@import\s+['"]([^'"]+)['"]/g)) {
        const target = relative(root, resolve(dirname(css), (m[1] ?? m[2])!));
        expect(target.startsWith('src/account/') || isShared(target), `${relative(root, css)} reaches ${target}`).toBe(true);
      }
    }
  });

  it('wears the pixel skin of the game: panel, button, input, leather and the game fonts', () => {
    const css = readFileSync(join(root, 'src/account/account.css'), 'utf8');
    for (const piece of ['panel', 'btn', 'input', 'leather', 'tab', 'tab-active']) expect(css, piece).toContain(`pixel/ui/chrome/${piece}.png`);
    expect(css).toContain('assets/fonts/fonts.css');
  });

  it.skipIf(!existsSync(join(root, 'dist/account')))('the built site holds no game files', () => {
    const built = filesUnder(join(root, 'dist/account'));
    expect(built.some((f) => f.endsWith('account.html'))).toBe(true);
    // Third-party chunks both sites load (vendor-*, named in the build configs) are not game code.
    const gameFiles = new Set(filesUnder(join(root, 'dist/game')).filter((f) => /\.(js|css|json|map)$/.test(f) && !/\/vendor-[^/]*$/.test(f)).map(sha));
    for (const file of built) expect(gameFiles.has(sha(file)), `${file} is a game code file`).toBe(false);
  });
});
