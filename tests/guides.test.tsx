import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { GUIDES } from '../src/account/content/guides';
import { guideParams } from '../src/account/content/params';
import { render } from '../src/account/entry-server';
import { pathFor } from '../src/account/routes';
import { SLUGS } from '../src/account/slugs';
import enGuides from '../src/i18n/catalog/en/guides.json';
import plGuides from '../src/i18n/catalog/pl/guides.json';

const LOCALES = ['en', 'pl'] as const;
type Section = { title: string; body: string };
type GuideCopy = { lead: string; sections: Record<string, Section> };
const CATALOG = { en: enGuides.guides, pl: plGuides.guides };
const guideUrl = (id: string, l: 'en' | 'pl') => pathFor('guide', l, { guide: SLUGS.guide[id]![l] });
const fill = (s: string) => s.replace(/\{(\w+)\}/g, (_, k: string) => String(guideParams()[k]));
const words = (s: string) => fill(s).split(/\s+/).filter((w) => /\p{L}/u.test(w)).length;
const countOf = (c: GuideCopy) => words(c.lead) + Object.values(c.sections).reduce((n, s) => n + words(s.body), 0);
const textOf = (html: string) => html.replace(/<[^>]+>/g, ' ');

function urls(): string[] {
  return LOCALES.flatMap((l) => [pathFor('howToPlay', l), ...GUIDES.map((g) => guideUrl(g.id, l))]);
}

describe('guides', () => {
  it('every guide and the overview render in both languages', () => {
    for (const url of urls()) {
      const page = render(url);
      expect(page.head.notFound, url).toBeFalsy();
      expect(page.html, url).toMatch(/<h1[^>]*>[^<]+<\/h1>/);
    }
    for (const l of LOCALES) {
      const overview = render(pathFor('howToPlay', l)).html;
      for (const g of GUIDES) expect(overview, `${l} links ${g.id}`).toContain(`href="${guideUrl(g.id, l)}"`);
      expect(overview).toContain(`href="${pathFor('units', l)}"`);
      expect(overview).toContain(`href="${pathFor('buildings', l)}"`);
    }
  });

  it('guides are 400 to 900 words and the overview 300 to 600, in both languages', () => {
    for (const l of LOCALES) {
      const c = CATALOG[l];
      const overview = countOf(c.howToPlay as GuideCopy);
      expect(overview, `${l} overview`).toBeGreaterThanOrEqual(300);
      expect(overview, `${l} overview`).toBeLessThanOrEqual(600);
      for (const g of GUIDES) {
        const copy = (c.guide as Record<string, GuideCopy>)[g.id]!;
        expect(Object.keys(copy.sections), `${l} ${g.id} sections`).toEqual(g.sections);
        const n = countOf(copy);
        expect(n, `${l} ${g.id}`).toBeGreaterThanOrEqual(400);
        expect(n, `${l} ${g.id}`).toBeLessThanOrEqual(900);
      }
    }
  });

  it('no placeholder is left unfilled', () => {
    for (const url of urls()) expect(textOf(render(url).html), url).not.toMatch(/[{}]/);
  });

  it('every placeholder used is a guideParams key', () => {
    const keys = new Set(Object.keys(guideParams()));
    for (const l of LOCALES) {
      const used = [...JSON.stringify(CATALOG[l]).matchAll(/\{(\w+)\}/g)].map((m) => m[1]!);
      for (const k of used) expect(keys.has(k) || k === 'title' || k === 'app', `${l} {${k}}`).toBe(true);
    }
  });

  it('every shot of every guide is shown with width and height and loads lazily', () => {
    for (const l of LOCALES) {
      for (const g of GUIDES) {
        const html = render(guideUrl(g.id, l)).html;
        const imgs = [...html.matchAll(/<img [^>]*>/g)].map((m) => m[0]);
        expect(imgs, `${l} ${g.id}`).toHaveLength(g.shots.length);
        for (const img of imgs) {
          expect(img).toMatch(/width="1200"/);
          expect(img).toMatch(/height="675"/);
          expect(img).toMatch(/loading="lazy"/);
          expect(img).toMatch(/alt="[^"]{10,}"/);
        }
      }
    }
  });

  it("related links point at encyclopedia pages in the page's language", () => {
    for (const l of LOCALES) {
      for (const g of GUIDES) {
        const html = render(guideUrl(g.id, l)).html;
        for (const r of g.related) {
          const path = r.kind === 'unit'
            ? pathFor('unit', l, { unit: SLUGS.unit[r.id]![l] })
            : pathFor('building', l, { building: SLUGS.building[r.id]![l] });
          expect(html, `${l} ${g.id} → ${r.id}`).toContain(`href="${path}"`);
        }
      }
    }
  });

  it('titles and descriptions are unique', () => {
    const seen = new Set<string>();
    for (const url of urls()) {
      const { head } = render(url);
      expect(head.description.length, url).toBeLessThanOrEqual(160);
      for (const v of [head.title, head.description]) {
        expect(seen.has(v), `${url}: ${v}`).toBe(false);
        seen.add(v);
      }
    }
  });

  it('no guide promises the same balance as another game', () => {
    const dir = join(__dirname, '..', 'src', 'i18n', 'catalog');
    for (const l of readdirSync(dir)) {
      for (const f of readdirSync(join(dir, l))) {
        expect(readFileSync(join(dir, l, f), 'utf8'), `${l}/${f}`).not.toMatch(/same balance|identical balance|ten sam balans|identyczny balans/i);
      }
    }
  });
});
