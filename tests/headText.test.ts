import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const ROOT = join(__dirname, '..');
const CATALOG = join(ROOT, 'src', 'i18n', 'catalog');
const WEB = join(ROOT, 'server', 'internal', 'adapters', 'web');

type Tree = { [k: string]: string | Tree };

function flatten(tree: Tree, prefix = '', out: Record<string, string | Tree> = {}) {
  for (const [k, v] of Object.entries(tree)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (typeof v === 'string' || 'other' in v) out[key] = v;
    else flatten(v, key, out);
  }
  return out;
}

/** Every message of a language, across all namespaces: the build exports them from wherever they live. */
function messages(lang: string) {
  return Object.assign({}, ...readdirSync(join(CATALOG, lang)).map((f) =>
    flatten(JSON.parse(readFileSync(join(CATALOG, lang, f), 'utf8')) as Tree)));
}

function serverKeys(): string[] {
  const files = [join(ROOT, 'server', 'internal', 'app', 'head.go')];
  if (existsSync(WEB)) files.push(...readdirSync(WEB).filter((f) => f.endsWith('.go') && !f.endsWith('_test.go')).map((f) => join(WEB, f)));
  const keys = new Set<string>();
  for (const f of files) {
    for (const m of readFileSync(f, 'utf8').matchAll(/"((?:head\.[\w.]+)|app\.name|world\.gameName)"/g)) keys.add(m[1]!);
  }
  return [...keys];
}

describe('head text', () => {
  it('every head key the server formats exists in both languages', () => {
    const keys = serverKeys();
    expect(keys).toContain('head.game.ended.title');
    for (const lang of ['en', 'pl']) {
      const all = messages(lang);
      for (const k of keys) {
        expect(all[k], `${lang} ${k}`).toBeDefined();
        if (k.startsWith('head.count.')) expect(typeof all[k], `${lang} ${k} is a plural`).toBe('object');
      }
    }
  });
});
