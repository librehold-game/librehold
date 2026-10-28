import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Fonts live in the repository and are described in ASSETS.md (licences). The game works offline, so no asset may
 * hang off an external CDN.
 */

const ROOT = join(__dirname, '..');
const FONTS = join(ROOT, 'src', 'assets', 'fonts');

describe('graphic assets', () => {
  it('ASSETS.md describes both fonts', () => {
    const doc = readFileSync(join(ROOT, 'ASSETS.md'), 'utf8');
    expect(doc).toContain('MedievalSharp');
    expect(doc).toContain('Cinzel');
  });
});

describe('the game works without internet', () => {
  it('fonts are self-hosted and the .woff2 files exist', () => {
    const css = readFileSync(join(FONTS, 'fonts.css'), 'utf8');
    expect(css).not.toMatch(/https?:\/\//);
    const files = readdirSync(FONTS);
    const used = [...css.matchAll(/url\('\.\/([\w-]+\.woff2)'\)/g)].map((m) => m[1]!);
    expect(used.length).toBeGreaterThan(0);
    for (const f of used) expect(files, `missing ${f}`).toContain(f);
  });

  it('OFL licences sit next to the fonts', () => {
    for (const f of ['OFL-MedievalSharp.txt', 'OFL-Cinzel.txt']) {
      expect(readFileSync(join(FONTS, f), 'utf8')).toContain('SIL Open Font License');
    }
  });

  it('pages load nothing from external servers', () => {
    for (const page of ['game.html', 'account.html']) {
      expect(readFileSync(join(ROOT, page), 'utf8')).not.toMatch(/https?:\/\//);
    }
  });
});

describe('pixel art village assets', () => {
  const DIR = join(ROOT, 'src', 'assets', 'pixel', 'village');
  const pngs = (dir: string, prefix = ''): string[] => readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? pngs(join(dir, e.name), `${prefix}${e.name}/`) : e.name.endsWith('.png') ? [`${prefix}${e.name}`] : []);

  const referenced = (v: unknown, out = new Set<string>()): Set<string> => {
    if (typeof v === 'string' && v.endsWith('.png')) out.add(v);
    else if (Array.isArray(v)) v.forEach((x) => referenced(x, out));
    else if (v && typeof v === 'object') Object.values(v).forEach((x) => referenced(x, out));
    return out;
  };

  it('every exported PNG is referenced by the layout (no orphans)', () => {
    const { files: _sizes, ...layout } = JSON.parse(readFileSync(join(DIR, 'layout.json'), 'utf8')) as Record<string, unknown>;
    const used = referenced(layout);
    const orphans = pngs(DIR).filter((f) => f !== 'ground.png' && !used.has(f));
    expect(orphans, `orphans: ${orphans.join(', ')}`).toHaveLength(0);
  });
});
