import { useSyncExternalStore } from 'react';

export type Locale = 'en' | 'pl';
export const LOCALES: Locale[] = ['en', 'pl'];

type Params = Record<string, string | number>;
export type Plural = { one: string; few?: string; many?: string; other: string };
type Message = string | Plural;
type Tree = { [k: string]: string | Tree };

const catalogs: Record<Locale, Map<string, Message>> = { en: new Map(), pl: new Map() };
const listeners = new Set<() => void>();
const warned = new Set<string>();
let current: Locale = 'en';

const isPlural = (v: Tree): boolean => typeof v.other === 'string';

function flatten(tree: Tree, prefix: string, into: Map<string, Message>) {
  for (const [k, v] of Object.entries(tree)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (typeof v === 'string') into.set(key, v);
    else if (isPlural(v)) into.set(key, v as unknown as Plural);
    else flatten(v, key, into);
  }
}

/** Adds a namespace of nested messages; each bundle registers only what it imports. */
export function registerCatalog(locale: Locale, messages: Record<string, unknown>): void {
  flatten(messages as Tree, '', catalogs[locale]);
  notify();
}

function notify() {
  for (const l of listeners) l();
}

function lookup(key: string, locale: Locale = current): Message | undefined {
  const own = catalogs[locale].get(key);
  if (own !== undefined) return own;
  const fallback = catalogs.en.get(key);
  if (fallback === undefined && !warned.has(key)) {
    warned.add(key);
    if (import.meta.env?.DEV || import.meta.env?.MODE === 'test') console.warn(`missing translation: ${key}`);
  }
  return fallback;
}

const numberFormats = new Map<Locale, Intl.NumberFormat>();
function formatCount(n: number, locale: Locale): string {
  let f = numberFormats.get(locale);
  if (!f) numberFormats.set(locale, (f = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })));
  return f.format(n);
}

function pickPlural(p: Plural, count: number, locale: Locale): string {
  const cat = new Intl.PluralRules(locale).select(count) as keyof Plural;
  return (p[cat] ?? p.other).replace(/#/g, formatCount(count, locale));
}

/** `t` in another language, without switching the page's language. */
export function translate(locale: Locale, key: string, params?: Params): string {
  const msg = lookup(key, locale);
  if (msg === undefined) return key;
  const text = typeof msg === 'string' ? msg : pickPlural(msg, Number(params?.count ?? 0), locale);
  if (!params) return text;
  return text.replace(/\{(\w+)\}/g, (m, name: string) => (name in params ? String(params[name]) : m));
}

export function t(key: string, params?: Params): string {
  return translate(current, key, params);
}

/** Messages whose key starts with `prefix`, the locale's own over English. */
export function catalogEntries(locale: Locale, prefix: string): Record<string, string | Plural> {
  const out: Record<string, Message> = {};
  for (const source of [catalogs.en, catalogs[locale]]) {
    for (const [key, msg] of source) if (key.startsWith(prefix)) out[key] = msg;
  }
  return out;
}

export function hasKey(key: string): boolean {
  return catalogs[current].has(key) || catalogs.en.has(key);
}

export function getLocale(): Locale {
  return current;
}

export function setLocale(l: Locale): void {
  if (!isLocale(l)) return;
  current = l;
  if (typeof document !== 'undefined') {
    document.documentElement.lang = l;
  }
  notify();
}

/** An explicit choice: the server redirects later visits to pages in this language. */
export function rememberLocale(l: Locale): void {
  if (!isLocale(l)) return;
  if (typeof document !== 'undefined') document.cookie = `lang=${l}; Path=/; Max-Age=31536000; SameSite=Lax`;
  setLocale(l);
}

const isLocale = (v: unknown): v is Locale => v === 'en' || v === 'pl';

export function detectLocale(): Locale {
  if (typeof document !== 'undefined') {
    const m = /(?:^|;\s*)lang=(\w+)/.exec(document.cookie);
    if (m && isLocale(m[1])) return m[1];
  }
  if (typeof navigator !== 'undefined' && navigator.language?.toLowerCase().startsWith('pl')) return 'pl';
  return 'en';
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

/** `t` that re-renders the component when the language changes. */
export function useT(): typeof t {
  useSyncExternalStore(subscribe, getLocale, getLocale);
  return t;
}

export function useLocale(): Locale {
  return useSyncExternalStore(subscribe, getLocale, getLocale);
}
