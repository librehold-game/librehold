import { describe, expect, it } from 'vitest';
import { render } from '../src/account/entry-server';
import { pathFor } from '../src/account/routes';
import enTerms from '../src/i18n/catalog/en/terms.json';
import plTerms from '../src/i18n/catalog/pl/terms.json';

const LOCALES = ['en', 'pl'] as const;
const textOf = (html: string) => html.replace(/<[^>]+>/g, ' ');

describe('content pages', () => {
  it('the FAQ has eleven questions in both languages', () => {
    for (const l of LOCALES) {
      const page = render(pathFor('faq', l));
      expect(page.head.notFound).toBeFalsy();
      expect(page.html.match(/<dt[ >]/g), l).toHaveLength(11);
      expect(textOf(page.html), l).not.toMatch(/[{}]/);
    }
  });

  it('the report answer links to the report form', () => {
    for (const l of LOCALES) {
      const html = render(pathFor('faq', l)).html;
      const answer = /<dd[^>]*data-id="report"[^>]*>([\s\S]*?)<\/dd>/.exec(html)?.[1] ?? '';
      expect(answer, l).toContain(`href="${pathFor('report', l)}"`);
    }
  });

  it('the changelog is newest first with six entries', () => {
    for (const l of LOCALES) {
      const html = render(pathFor('changelog', l)).html;
      const dates = [...html.matchAll(/data-date="([\d-]+)"/g)].map((m) => m[1]);
      expect(dates, l).toEqual(['2026-10-26', '2026-10-20', '2026-10-19', '2026-10-18', '2026-08-23', '2026-08-22']);
      expect(html).toMatch(/<li>/);
    }
  });

  it('the terms name tomcatflow.com and link the report form and are 400 to 800 words', () => {
    for (const [l, cat] of [['en', enTerms], ['pl', plTerms]] as const) {
      const html = render(pathFor('terms', l)).html;
      expect(html, l).toContain('tomcatflow.com');
      expect(html, l).toContain(`href="${pathFor('report', l)}"`);
      expect(textOf(html), l).not.toMatch(/[{}]/);
      const words = Object.values(cat.terms.sections).map((s) => s.body.split(/\s+/).filter((w) => /\p{L}/u.test(w)).length)
        .reduce((a, b) => a + b, 0);
      expect(words, l).toBeGreaterThanOrEqual(400);
      expect(words, l).toBeLessThanOrEqual(800);
    }
  });

  it('faq, changelog and terms have unique titles and descriptions', () => {
    const seen = new Set<string>();
    for (const id of ['faq', 'changelog', 'terms'] as const) {
      for (const l of LOCALES) {
        const { head } = render(pathFor(id, l));
        expect(head.notFound, `${id} ${l}`).toBeFalsy();
        expect(head.description.length).toBeLessThanOrEqual(160);
        for (const v of [head.title, head.description]) {
          expect(seen.has(v), v).toBe(false);
          seen.add(v);
        }
      }
    }
  });
});
