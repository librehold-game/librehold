import { translate, type Locale } from '../i18n/i18n';
import type { HeadData } from './Head';
import { localeOf, matchPath, pathFor, ROUTES, twinPath } from './routes';

export const DEFAULT_IMAGE = '/og/home.png';
export const SITE_URL = '__SITE_URL__';

const OG_LOCALE: Record<Locale, string> = { en: 'en_US', pl: 'pl_PL' };

export const escapeHtml = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export const jsonLdScript = (obj: Record<string, unknown>) =>
  `<script type="application/ld+json">${JSON.stringify(obj).replace(/</g, '\\u003c')}</script>`;

interface Resolved {
  locale: Locale;
  noIndex: boolean;
  canonical: string | null;
  twins: { en: string; pl: string } | null;
}

function resolve(head: HeadData, pathname: string): Resolved {
  const m = matchPath(pathname);
  const locale = m?.locale ?? localeOf(pathname);
  if (!m || head.notFound) return { locale, noIndex: true, canonical: null, twins: null };
  const route = ROUTES.find((r) => r.id === m.id)!;
  const own = pathFor(m.id, m.locale, m.params);
  const twin = twinPath(own);
  return {
    locale,
    noIndex: route.index === 'no' || !!head.noIndex,
    canonical: own,
    twins: m.locale === 'en' ? { en: own, pl: twin } : { en: twin, pl: own },
  };
}

/** The page's head tags, in the order the server writes them too. */
export function headTags(head: HeadData, pathname: string, origin = SITE_URL): string {
  const r = resolve(head, pathname);
  const tr = (key: string) => translate(r.locale, key);
  const meta = (attr: 'name' | 'property', key: string, value: string) =>
    `<meta ${attr}="${key}" content="${escapeHtml(value)}">`;
  const out = [`<title>${escapeHtml(head.title)}</title>`, meta('name', 'description', head.description)];
  if (r.noIndex) out.push('<meta name="robots" content="noindex">');
  if (r.canonical) out.push(`<link rel="canonical" href="${escapeHtml(origin + r.canonical)}">`);
  if (r.twins) {
    for (const [lang, path] of [['en', r.twins.en], ['pl', r.twins.pl], ['x-default', r.twins.en]] as const) {
      out.push(`<link rel="alternate" hreflang="${lang}" href="${escapeHtml(origin + path)}">`);
    }
  }
  const other: Locale = r.locale === 'pl' ? 'en' : 'pl';
  out.push(
    meta('property', 'og:type', 'website'),
    meta('property', 'og:site_name', tr('app.name')),
    meta('property', 'og:title', head.title),
    meta('property', 'og:description', head.description),
    meta('property', 'og:url', origin + (r.canonical ?? pathname)),
    meta('property', 'og:image', origin + (head.image ?? DEFAULT_IMAGE)),
    meta('property', 'og:image:width', '1200'),
    meta('property', 'og:image:height', '630'),
    meta('property', 'og:image:alt', head.imageAlt ?? tr('head.imageAlt')),
    meta('property', 'og:locale', OG_LOCALE[r.locale]),
    meta('property', 'og:locale:alternate', OG_LOCALE[other]),
    meta('name', 'twitter:card', 'summary_large_image'),
  );
  for (const obj of head.jsonLd ?? []) out.push(jsonLdScript(obj));
  return out.join('\n');
}

function ensure(selector: string, create: () => HTMLElement): HTMLElement {
  return document.head.querySelector<HTMLElement>(selector) ?? document.head.appendChild(create());
}

function setLink(selector: string, attrs: Record<string, string> | null) {
  const existing = document.head.querySelector(selector);
  if (!attrs) {
    existing?.remove();
    return;
  }
  const el = existing ?? document.head.appendChild(document.createElement('link'));
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
}

function setMeta(name: string, content: string | null) {
  if (content === null) {
    document.head.querySelector(`meta[name="${name}"]`)?.remove();
    return;
  }
  ensure(`meta[name="${name}"]`, () => {
    const m = document.createElement('meta');
    m.setAttribute('name', name);
    return m;
  }).setAttribute('content', content);
}

/** Keeps the document's head right after client-side navigation. */
export function applyHead(head: HeadData, pathname: string): void {
  const r = resolve(head, pathname);
  const origin = window.location.origin;
  document.title = head.title;
  setMeta('description', head.description);
  setMeta('robots', r.noIndex ? 'noindex' : null);
  setLink('link[rel="canonical"]', r.canonical ? { rel: 'canonical', href: origin + r.canonical } : null);
  for (const [lang, key] of [['en', 'en'], ['pl', 'pl'], ['x-default', 'en']] as const) {
    setLink(`link[rel="alternate"][hreflang="${lang}"]`,
      r.twins ? { rel: 'alternate', hreflang: lang, href: origin + r.twins[key] } : null);
  }
}
