import { renderToString } from 'react-dom/server';
import { App } from './App';
import { HeadSink, type HeadData } from './Head';
import './pagesEager';
import { setServerPath } from './router';
import { localeOf, pathFor, ROUTES } from './routes';
import { SLUGS, type SlugKind } from './slugs';
import { loadAccountCatalogs } from '../i18n/catalogs';
import { loadHeadCatalog } from '../i18n/headCatalog';
import { catalogEntries, LOCALES, setLocale, type Locale, type Plural } from '../i18n/i18n';

export { matchPath, ROUTES, twinPath } from './routes';
export { headTags } from './headTags';

let loaded = false;

/** One page as HTML: the app, the head it asked for, and the preloads React put in front of it. */
export function render(url: string): { html: string; head: HeadData; hoisted: string } {
  if (!loaded) {
    loadAccountCatalogs();
    loadHeadCatalog();
    loaded = true;
  }
  const path = url.split(/[?#]/)[0]!;
  setServerPath(path);
  setLocale(localeOf(path));
  const head: HeadData = { title: '', description: '' };
  const out = renderToString(
    <HeadSink.Provider value={head}>
      <App />
    </HeadSink.Provider>,
  );
  const lead = /^(?:<link [^>]*>)*/.exec(out)![0];
  return { html: out.slice(lead.length), head, hoisted: lead };
}

const SLUG_PARAM: Record<string, SlugKind> = { guide: 'guide', unit: 'unit', building: 'building' };

/** Every address built ahead of time, in both languages, with slug routes expanded. */
export function prerenderUrls(): string[] {
  const urls: string[] = [];
  for (const r of ROUTES.filter((x) => x.build === 'prerender')) {
    const param = /:([a-zA-Z]+)/.exec(r.en)?.[1];
    for (const l of LOCALES) {
      if (!param) urls.push(pathFor(r.id, l));
      else for (const id of Object.keys(SLUGS[SLUG_PARAM[param]!])) urls.push(pathFor(r.id, l, { [param]: SLUGS[SLUG_PARAM[param]!][id]![l] }));
    }
  }
  return urls;
}

/** The copy the server needs to write heads of pages it fills at request time. */
export function headText(): Record<Locale, Record<string, string | Plural>> {
  if (!loaded) {
    loadAccountCatalogs();
    loadHeadCatalog();
    loaded = true;
  }
  const out = {} as Record<Locale, Record<string, string | Plural>>;
  for (const l of LOCALES) {
    out[l] = { ...catalogEntries(l, 'app.name'), ...catalogEntries(l, 'world.gameName'), ...catalogEntries(l, 'head.') };
  }
  return out;
}
