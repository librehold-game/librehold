import { describe, expect, it } from 'vitest';
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

/** Every catalogue namespace has the same keys in every language, and guarded code never hard-codes UI text. */
const ROOT = join(__dirname, '..');
const CATALOG = join(ROOT, 'src', 'i18n', 'catalog');
/** Directories whose user-facing text must go through the catalogues. */
export const GUARDED = ['src/i18n', 'src/account', 'src/shared', 'src/ui', 'src/state', 'src/net', 'src/data', 'src/engine', 'src/main.tsx'];
/** Go sources scanned for Polish text (tests excluded). */
export const GUARDED_GO: string[] = ['server/internal'];
const POLISH = /[ąćęłńóśźżĄĆĘŁŃÓŚŹŻ]/;
/** Letters between a closing `>` (not an arrow) and the next tag: text a JSX element renders as is. */
const JSX_TEXT = /(?<![=-])>\s*[A-Za-z][^<>{}()=;&|]*[a-z][^<>{}()=;&|]*</;
/** Attributes people read or hear, written as a literal. */
const JSX_ATTR = /\b(?:title|aria-label|alt|placeholder)="[^"]*[A-Za-z]{2}[^"]*"/;

function files(path: string): string[] {
  const full = join(ROOT, path);
  if (!existsSync(full)) return [];
  if (!statSync(full).isDirectory()) return [full];
  return readdirSync(full, { withFileTypes: true })
    .filter((e) => !['node_modules', 'dist', '.cache', 'catalog', 'testdata'].includes(e.name))
    .flatMap((e) => files(join(path, e.name)));
}

const stripComments = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:'"`])\/\/.*$/gm, '$1');

type Tree = { [k: string]: string | Tree };
const isPlural = (v: Tree) => 'other' in v && typeof v.other === 'string';

function flatten(tree: Tree, prefix = '', out: Record<string, string | Tree> = {}) {
  for (const [k, v] of Object.entries(tree)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (typeof v === 'string' || isPlural(v)) out[key] = v;
    else flatten(v, key, out);
  }
  return out;
}

const load = (lang: string, ns: string): Tree => JSON.parse(readFileSync(join(CATALOG, lang, `${ns}.json`), 'utf8'));
const namespaces = () => readdirSync(join(CATALOG, 'en')).map((f) => f.replace(/\.json$/, ''));

describe('translation guard', () => {
  it('catalogues have identical key sets', () => {
    for (const ns of namespaces()) {
      const en = flatten(load('en', ns));
      const pl = flatten(load('pl', ns));
      expect(Object.keys(pl).sort(), ns).toEqual(Object.keys(en).sort());
      for (const [key, value] of Object.entries(pl)) {
        if (typeof value !== 'string') expect(Object.keys(value).sort(), `pl ${ns}.${key}`).toEqual(['few', 'many', 'one', 'other']);
      }
      for (const [key, value] of Object.entries(en)) {
        if (typeof value !== 'string') expect(Object.keys(value).sort(), `en ${ns}.${key}`).toEqual(['one', 'other']);
      }
    }
  });

  it('the JSX text pattern catches literal text and skips code', () => {
    expect(JSX_TEXT.test('<h1>Hello world</h1>')).toBe(true);
    expect(JSX_TEXT.test('<button className="btn">Save changes</button>')).toBe(true);
    expect(JSX_TEXT.test('<h1>{t(\'x\')}</h1>')).toBe(false);
    expect(JSX_TEXT.test('(f) => error?.field === f && <p>')).toBe(false);
    expect(JSX_TEXT.test('<span>{n} · {m}</span>')).toBe(false);
    expect(JSX_ATTR.test('<svg aria-label="Ilustracja wioski">')).toBe(true);
    expect(JSX_ATTR.test('<img alt="" title={t(\'x\')} />')).toBe(false);
  });

  it('guarded sources contain no hard-coded UI text', () => {
    const offenders = GUARDED.flatMap(files)
      .filter((f) => /\.(ts|tsx)$/.test(f))
      .flatMap((f) => {
        const src = stripComments(readFileSync(f, 'utf8'));
        const bad = POLISH.test(src) || (f.endsWith('.tsx') && (JSX_TEXT.test(src) || JSX_ATTR.test(src)));
        return bad ? [relative(ROOT, f)] : [];
      });
    expect(offenders).toEqual([]);
  });

  it('guarded Go sources contain no Polish text', () => {
    const offenders = GUARDED_GO.flatMap(files)
      .filter((f) => f.endsWith('.go') && !f.endsWith('_test.go') && !f.includes('/gametest/') && !f.includes('/apptest/'))
      .filter((f) => POLISH.test(stripComments(readFileSync(f, 'utf8'))))
      .map((f) => relative(ROOT, f));
    expect(offenders).toEqual([]);
  });
});
