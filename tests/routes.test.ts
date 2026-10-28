import { describe, expect, it } from 'vitest';
import { localeOf, matchPath, pathFor, ROUTES, slugFor, idForSlug, twinPath } from '../src/account/routes';
import { GUIDE_IDS, SLUGS } from '../src/account/slugs';
import { BUILDING_IDS, UNIT_IDS } from '../src/engine/types';
import type { Locale } from '../src/i18n/i18n';

const LOCALES: Locale[] = ['en', 'pl'];

function sampleParams(pattern: string, locale: Locale): Record<string, string> {
  const params: Record<string, string> = {};
  for (const seg of pattern.split('/')) {
    if (!seg.startsWith(':')) continue;
    const name = seg.slice(1);
    if (name === 'guide') params[name] = slugFor('guide', GUIDE_IDS[0], locale);
    else if (name === 'unit') params[name] = slugFor('unit', UNIT_IDS[0]!, locale);
    else if (name === 'building') params[name] = slugFor('building', BUILDING_IDS[0]!, locale);
    else params[name] = ({ id: '7', name: 'kind-otter', code: 'x1' } as Record<string, string>)[name]!;
  }
  return params;
}

describe('routes', () => {
  it('every route round-trips in both languages', () => {
    for (const r of ROUTES) {
      for (const l of LOCALES) {
        const p = sampleParams(r[l], l);
        expect(matchPath(pathFor(r.id, l, p))).toEqual({ id: r.id, locale: l, params: p });
      }
    }
  });

  it('the public addresses', () => {
    expect(pathFor('home', 'pl')).toBe('/pl/');
    expect(pathFor('game', 'pl', { id: '41' })).toBe('/pl/gry/41');
    expect(pathFor('player', 'en', { name: 'kind-otter' })).toBe('/players/kind-otter');
    expect(pathFor('report', 'pl')).toBe('/pl/zglos');
    expect(pathFor('join', 'pl', { code: 'x1' })).toBe('/pl/dolacz/x1');
    expect(pathFor('changelog', 'pl')).toBe('/pl/zmiany');
  });

  it('missing params and notFound throw', () => {
    expect(() => pathFor('game', 'en')).toThrow('missing param id for game');
    expect(() => pathFor('notFound', 'en')).toThrow();
  });

  it('trailing slashes and unknown paths', () => {
    expect(matchPath('/privacy/')?.id).toBe('privacy');
    expect(matchPath('/pl')).toEqual({ id: 'home', locale: 'pl', params: {} });
    expect(matchPath('/nope')).toBeNull();
    expect(matchPath('/pl/nope')).toBeNull();
    expect(matchPath('/games/')).toBeNull();
    expect(matchPath('/players/%E0')).toBeNull();
  });

  it('twins', () => {
    expect(twinPath('/units/spearman')).toBe('/pl/jednostki/pikinier');
    expect(twinPath('/pl/gracze/Ala')).toBe('/players/Ala');
    expect(twinPath('/')).toBe('/pl/');
    expect(twinPath('/nope')).toBe('/pl/');
  });

  it('player names are escaped', () => {
    const path = pathFor('player', 'en', { name: 'a b/c' });
    expect(path).toBe('/players/a%20b%2Fc');
    expect(matchPath(path)?.params.name).toBe('a b/c');
  });

  it('slug table covers every unit, building and guide', () => {
    expect(Object.keys(SLUGS.unit).sort()).toEqual([...UNIT_IDS].sort());
    expect(Object.keys(SLUGS.building).sort()).toEqual([...BUILDING_IDS].sort());
    expect(Object.keys(SLUGS.guide).sort()).toEqual([...GUIDE_IDS].sort());
    for (const kind of ['guide', 'unit', 'building'] as const) {
      for (const l of LOCALES) {
        const slugs = Object.keys(SLUGS[kind]).map((id) => slugFor(kind, id, l));
        for (const s of slugs) expect(s).toMatch(/^[a-z0-9-]+$/);
        expect(new Set(slugs).size).toBe(slugs.length);
        for (const id of Object.keys(SLUGS[kind])) expect(idForSlug(kind, slugFor(kind, id, l), l)).toBe(id);
      }
    }
    expect(idForSlug('unit', 'nope', 'en')).toBeNull();
  });

  it('localeOf', () => {
    expect(localeOf('/pl/x')).toBe('pl');
    expect(localeOf('/pl')).toBe('pl');
    expect(localeOf('/plx')).toBe('en');
  });
});
