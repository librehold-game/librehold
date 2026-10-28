import type { Locale } from '../i18n/i18n';
import { SLUGS, type SlugKind } from './slugs';

export type RouteId =
  | 'home' | 'howToPlay' | 'guide' | 'units' | 'unit' | 'buildings' | 'building' | 'faq'
  | 'changelog' | 'privacy' | 'terms' | 'report' | 'worlds' | 'world' | 'game' | 'player' | 'login' | 'register'
  | 'lobby' | 'admin' | 'join' | 'notFound';

export interface RouteDef {
  id: RouteId;
  en: string;
  pl: string;
  index: 'yes' | 'no' | 'registered';
  build: 'prerender' | 'dynamic' | 'app';
}

export const ROUTES: RouteDef[] = [
  { id: 'home', en: '/', pl: '/pl/', index: 'yes', build: 'prerender' },
  { id: 'howToPlay', en: '/how-to-play', pl: '/pl/jak-grac', index: 'yes', build: 'prerender' },
  { id: 'guide', en: '/how-to-play/:guide', pl: '/pl/jak-grac/:guide', index: 'yes', build: 'prerender' },
  { id: 'units', en: '/units', pl: '/pl/jednostki', index: 'yes', build: 'prerender' },
  { id: 'unit', en: '/units/:unit', pl: '/pl/jednostki/:unit', index: 'yes', build: 'prerender' },
  { id: 'buildings', en: '/buildings', pl: '/pl/budynki', index: 'yes', build: 'prerender' },
  { id: 'building', en: '/buildings/:building', pl: '/pl/budynki/:building', index: 'yes', build: 'prerender' },
  { id: 'faq', en: '/faq', pl: '/pl/faq', index: 'yes', build: 'prerender' },
  { id: 'changelog', en: '/changelog', pl: '/pl/zmiany', index: 'yes', build: 'prerender' },
  { id: 'privacy', en: '/privacy', pl: '/pl/prywatnosc', index: 'yes', build: 'prerender' },
  { id: 'terms', en: '/terms', pl: '/pl/regulamin', index: 'yes', build: 'prerender' },
  { id: 'report', en: '/report', pl: '/pl/zglos', index: 'no', build: 'prerender' },
  { id: 'worlds', en: '/worlds', pl: '/pl/swiaty', index: 'yes', build: 'dynamic' },
  { id: 'world', en: '/worlds/:id', pl: '/pl/swiaty/:id', index: 'yes', build: 'dynamic' },
  { id: 'game', en: '/games/:id', pl: '/pl/gry/:id', index: 'yes', build: 'dynamic' },
  { id: 'player', en: '/players/:name', pl: '/pl/gracze/:name', index: 'registered', build: 'dynamic' },
  { id: 'login', en: '/login', pl: '/pl/logowanie', index: 'no', build: 'app' },
  { id: 'register', en: '/register', pl: '/pl/rejestracja', index: 'no', build: 'app' },
  { id: 'lobby', en: '/lobby', pl: '/pl/lobby', index: 'no', build: 'app' },
  { id: 'admin', en: '/admin', pl: '/pl/admin', index: 'no', build: 'app' },
  { id: 'join', en: '/join/:code', pl: '/pl/dolacz/:code', index: 'no', build: 'app' },
];

const segments = (path: string) => path.split('/').filter((s) => s !== '');

const compiled = ROUTES.flatMap((r) =>
  (['en', 'pl'] as const).map((locale) => ({ id: r.id, locale, segs: segments(r[locale]) })),
);

export function localeOf(pathname: string): Locale {
  return pathname === '/pl' || pathname.startsWith('/pl/') ? 'pl' : 'en';
}

export function pathFor(id: RouteId, locale: Locale, params: Record<string, string> = {}): string {
  const route = ROUTES.find((r) => r.id === id);
  if (!route) throw new Error(`no address for ${id}`);
  return route[locale].replace(/:([a-zA-Z]+)/g, (_, name: string) => {
    const value = params[name];
    if (value === undefined || value === '') throw new Error(`missing param ${name} for ${id}`);
    return encodeURIComponent(value);
  });
}

export function matchPath(pathname: string): { id: RouteId; locale: Locale; params: Record<string, string> } | null {
  const trimmed = pathname.length > 1 && pathname.endsWith('/') ? pathname.slice(0, -1) : pathname;
  if (trimmed.includes('//')) return null;
  const parts = segments(trimmed);
  for (const c of compiled) {
    if (c.segs.length !== parts.length) continue;
    const params: Record<string, string> = {};
    let ok = true;
    for (let i = 0; i < parts.length && ok; i++) {
      const seg = c.segs[i]!;
      if (seg.startsWith(':')) {
        try {
          params[seg.slice(1)] = decodeURIComponent(parts[i]!);
        } catch {
          return null;
        }
      } else ok = seg === parts[i];
    }
    if (ok) return { id: c.id, locale: c.locale, params };
  }
  return null;
}

export function slugFor(kind: SlugKind, id: string, locale: Locale): string {
  const entry = SLUGS[kind][id];
  if (!entry) throw new Error(`no ${kind} slug for ${id}`);
  return entry[locale];
}

export function idForSlug(kind: SlugKind, slug: string, locale: Locale): string | null {
  for (const [id, entry] of Object.entries(SLUGS[kind])) if (entry[locale] === slug) return id;
  return null;
}

const SLUG_PARAMS: Record<string, SlugKind> = { guide: 'guide', unit: 'unit', building: 'building' };

export function twinPath(pathname: string): string {
  const m = matchPath(pathname);
  if (!m) return pathFor('home', localeOf(pathname) === 'pl' ? 'en' : 'pl');
  const other: Locale = m.locale === 'pl' ? 'en' : 'pl';
  const params: Record<string, string> = {};
  for (const [name, value] of Object.entries(m.params)) {
    const kind = SLUG_PARAMS[name];
    const id = kind ? idForSlug(kind, value, m.locale) : null;
    params[name] = kind && id ? slugFor(kind, id, other) : value;
  }
  return pathFor(m.id, other, params);
}
