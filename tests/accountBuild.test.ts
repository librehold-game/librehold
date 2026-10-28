import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { gzipSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';
import { ROUTES } from '../src/account/routes';

const DIST = join(__dirname, '..', 'dist', 'account');
const SSR = join(__dirname, '..', 'dist', 'account-ssr');
const read = (rel: string) => readFileSync(join(DIST, rel), 'utf8');
const PAGES: [string, 'en' | 'pl'][] = [
  ['index.html', 'en'], ['pl/index.html', 'pl'], ['privacy/index.html', 'en'], ['pl/prywatnosc/index.html', 'pl'],
];
/** The front page's JavaScript, gzip level 9: 89_860 bytes on React 19.3, plus about 2 kB of headroom. */
const FRONT_PAGE_JS_BUDGET = 92_000;

function filesUnder(dir: string): string[] {
  return readdirSync(dir).flatMap((n) => (statSync(join(dir, n)).isDirectory() ? filesUnder(join(dir, n)).map((f) => `${n}/${f}`) : [n]));
}

describe.skipIf(!existsSync(join(DIST, 'routes.json')))('account build', () => {
  it('writes the pages, the shell and the server files', () => {
    for (const f of ['index.html', 'pl/index.html', 'privacy/index.html', 'pl/prywatnosc/index.html', '404.html', 'pl/404.html',
      'account.html', 'routes.json', 'pages.json', 'head-text.json']) {
      expect(existsSync(join(DIST, f)), f).toBe(true);
    }
  });

  it('the shell keeps the placeholders for the server', () => {
    const shell = read('account.html');
    expect(shell).toContain('<!--dynamic-head-->');
    expect(shell).toContain('lang="__LANG__"');
    expect(shell).not.toContain('<title>');
  });

  it('every prerendered page is complete', () => {
    for (const [page, lang] of PAGES) {
      const html = read(page);
      expect(html, page).toContain(`<html lang="${lang}"`);
      expect(html.match(/<title>/g), page).toHaveLength(1);
      expect(html, page).toMatch(/<link rel="canonical" href="__SITE_URL__/);
      expect(html, page).toContain('property="og:image"');
      expect(html, page).not.toContain('<div id="root"></div>');
      for (const m of html.matchAll(/["(](\/assets\/[^"')]+)/g)) expect(existsSync(join(DIST, m[1]!)), `${page} ${m[1]}`).toBe(true);
    }
  });

  it('routes.json is the route table', () => {
    expect(JSON.parse(read('routes.json'))).toEqual(ROUTES);
  });

  it('front page JavaScript stays within its budget', () => {
    const html = read('index.html');
    const srcs = [...html.matchAll(/<script[^>]+src="([^"]+)"|<link rel="modulepreload"[^>]*href="([^"]+)"/g)].map((m) => m[1] ?? m[2]!);
    expect(srcs.length).toBeGreaterThan(0);
    const total = srcs.reduce((sum, src) => sum + gzipSync(readFileSync(join(DIST, src)), { level: 9 }).length, 0);
    expect(total).toBeLessThanOrEqual(FRONT_PAGE_JS_BUDGET);
  });

  it('server-side render files stay out of the public folder', () => {
    if (!existsSync(SSR)) return;
    const published = new Set(filesUnder(DIST).map((f) => f.split('/').pop()));
    for (const f of filesUnder(SSR)) expect(published.has(f.split('/').pop()), f).toBe(false);
  });
});
