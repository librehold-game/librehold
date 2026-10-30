import { describe, expect, it } from 'vitest';
import { BUILDINGS } from '../src/data/buildings.data';
import { UNITS } from '../src/data/units.data';
import { buildingCost, buildingPoints, buildingPopulation, buildTimeMs } from '../src/engine/buildings';
import { buildingLevels, unitFacts } from '../src/shared/encyclopedia/facts';
import { render } from '../src/account/entry-server';
import { pathFor } from '../src/account/routes';
import { SLUGS } from '../src/account/slugs';
import enEncyclopedia from '../src/i18n/catalog/en/encyclopedia.json';
import plEncyclopedia from '../src/i18n/catalog/pl/encyclopedia.json';
import enGame from '../src/i18n/catalog/en/game.json';
import plGame from '../src/i18n/catalog/pl/game.json';
import type { BuildingId, UnitId } from '../src/engine/types';

const LOCALES = ['en', 'pl'] as const;
const GAME = { en: enGame, pl: plGame };
const unitIds = Object.keys(SLUGS.unit) as UnitId[];
const buildingIds = Object.keys(SLUGS.building) as BuildingId[];
const unitUrl = (id: UnitId, l: 'en' | 'pl') => pathFor('unit', l, { unit: SLUGS.unit[id]![l] });
const buildingUrl = (id: BuildingId, l: 'en' | 'pl') => pathFor('building', l, { building: SLUGS.building[id]![l] });
const h1 = (html: string) => /<h1[^>]*>(.*?)<\/h1>/.exec(html)?.[1];
const levelRows = (html: string) => (/<table class="levels"[\s\S]*?<tbody>([\s\S]*?)<\/tbody>/.exec(html)?.[1]?.match(/<tr/g) ?? []).length;

function allUrls(): string[] {
  const urls: string[] = [];
  for (const l of LOCALES) {
    urls.push(pathFor('units', l), pathFor('buildings', l));
    for (const id of unitIds) urls.push(unitUrl(id, l));
    for (const id of buildingIds) urls.push(buildingUrl(id, l));
  }
  return urls;
}

describe('encyclopedia', () => {
  it('unit facts come from the data', () => {
    const u = UNITS.spear;
    expect(unitFacts('spear')).toEqual({ id: 'spear', cost: u.cost, pop: u.pop, attack: u.attack, defense: u.defense,
      defenseCavalry: u.defenseCavalry, defenseArcher: u.defenseArcher, minutesPerField: u.speed, carry: u.haul,
      building: u.building, recruitSec: u.baseRecruitSec, requirements: u.requirements });
  });

  it('building levels match the engine', () => {
    for (const id of Object.keys(BUILDINGS) as BuildingId[]) {
      const rows = buildingLevels(id);
      expect(rows, id).toHaveLength(BUILDINGS[id].maxLevel);
      for (const r of rows) {
        expect(r.cost, `${id} ${r.level}`).toEqual(buildingCost(id, r.level));
        expect(r.timeMs).toBe(buildTimeMs(id, r.level, 1, 1));
        expect(r.pop).toBe(buildingPopulation(id, r.level));
        expect(r.points).toBe(buildingPoints(id, r.level));
      }
    }
  });

  it('every unit and building has a page in both languages', () => {
    expect(unitIds).toHaveLength(12);
    expect(buildingIds).toHaveLength(15);
    for (const l of LOCALES) {
      for (const url of [pathFor('units', l), pathFor('buildings', l)]) expect(render(url).head.notFound, url).toBeFalsy();
      for (const id of unitIds) {
        const page = render(unitUrl(id, l));
        expect(page.head.notFound, id).toBeFalsy();
        expect(h1(page.html), id).toBe(GAME[l].unit[id]);
      }
      for (const id of buildingIds) {
        const page = render(buildingUrl(id, l));
        expect(page.head.notFound, id).toBeFalsy();
        expect(h1(page.html), id).toBe(GAME[l].building[id]);
      }
    }
    const spear = render('/units/spearman').html;
    for (const n of ['50', '30', '10', '18']) expect(spear).toContain(`>${n}<`);
    expect(levelRows(render('/buildings/town-hall').html)).toBe(30);
    expect(levelRows(render(buildingUrl('statue', 'en')).html)).toBe(1);
  });

  it('the stable page lists what it unlocks and the whole requirement chain', () => {
    const html = render('/buildings/stable').html;
    expect(html).toContain('Unlocks');
    for (const id of ['scout', 'lightCav', 'mountedArcher', 'heavyCav'] as const) expect(html, id).toContain(`href="${unitUrl(id, 'en')}"`);
    const chain = /<ul class="req-chain">([\s\S]*?)<\/ul><\/li><\/ul>/.exec(html)?.[0] ?? '';
    expect(chain).toContain(`href="${buildingUrl('main', 'en')}"`);
    expect(chain).toContain(`href="${buildingUrl('smithy', 'en')}"`);
  });

  it('a unit page shows the stats at every research level, a noble none', () => {
    const html = render(unitUrl('lightCav', 'en')).html;
    expect(html).toContain('Stats by research level');
    for (const n of ['163', '182']) expect(html, n).toContain(`>${n}<`);
    expect(render(unitUrl('noble', 'en')).html).not.toContain('Stats by research level');
  });

  it('public pages never speak of a village: no progress, no colours', () => {
    for (const url of allUrls()) {
      const html = render(url).html;
      expect(html, url).not.toContain('you have');
      expect(html, url).not.toContain('req-met');
      expect(html, url).not.toContain('req-missing');
    }
  });

  it('titles and descriptions are unique and at most 160 characters', () => {
    const titles = new Set<string>();
    const descriptions = new Set<string>();
    for (const url of allUrls()) {
      const { head } = render(url);
      expect(head.title.length, url).toBeLessThanOrEqual(160);
      expect(head.description.length, url).toBeLessThanOrEqual(160);
      expect(titles.has(head.title), `title of ${url}`).toBe(false);
      expect(descriptions.has(head.description), `description of ${url}`).toBe(false);
      titles.add(head.title);
      descriptions.add(head.description);
    }
  });

  it("each entry's own text is two to four sentences in both languages", () => {
    for (const [l, cat] of [['en', enEncyclopedia], ['pl', plEncyclopedia]] as const) {
      const texts = [...Object.entries(cat.encyclopedia.unit), ...Object.entries(cat.encyclopedia.building)];
      expect(texts, l).toHaveLength(27);
      for (const [id, { text }] of texts) {
        const sentences = text.split(/[.!?](?:\s|$)/).filter((s) => s.trim() !== '').length;
        expect(sentences, `${l} ${id}: ${text}`).toBeGreaterThanOrEqual(2);
        expect(sentences, `${l} ${id}: ${text}`).toBeLessThanOrEqual(4);
      }
    }
  });

  it('entry pages carry a BreadcrumbList and their own card', () => {
    const page = render('/units/spearman');
    const crumbs = (page.head.jsonLd ?? []).filter((j) => j['@type'] === 'BreadcrumbList');
    expect(crumbs).toHaveLength(1);
    const items = crumbs[0]!.itemListElement as { name: string }[];
    expect(items).toHaveLength(3);
    expect(items[2]!.name).toBe('Spearman');
    expect(page.head.image).toBe('/og/unit-spear.png');
    expect(page.head.imageAlt).toBe('Spearman');
    expect(page.html).toMatch(/<nav[^>]*aria-label="[^"]+"[^>]*>[\s\S]*href="\/"[\s\S]*href="\/units"/);
    expect(render('/buildings/town-hall').head.image).toBe('/og/building-main.png');
  });

  it('an unknown slug renders the not-found page', () => {
    for (const url of ['/units/dragon', '/pl/jednostki/smok', '/buildings/castle', '/pl/budynki/zamek']) {
      expect(render(url).head.notFound, url).toBe(true);
    }
  });
});
