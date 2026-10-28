import { createElement, StrictMode } from 'react';
import { createRoot, hydrateRoot } from 'react-dom/client';
import { App } from './App';
import { rewriteLegacyHash } from './legacyLinks';
import { preloadRoute } from './pages';
import { localeOf, matchPath } from './routes';
import { loadAccountCatalogs } from '../i18n/catalogs';
import { detectLocale, setLocale } from '../i18n/i18n';

/**
 * Takes over a prerendered page, or renders the app into the empty shell. An old `#/…` link has no language, so it
 * is rewritten in the reader's language and then rendered fresh: the prerendered HTML is of another page. The page's
 * own code is loaded first, so hydration finds the same markup the prerender wrote.
 */
export async function startAccountSite(root: HTMLElement): Promise<'hydrate' | 'render'> {
  loadAccountCatalogs();
  setLocale(window.location.hash ? detectLocale() : localeOf(window.location.pathname));
  const rewritten = rewriteLegacyHash();
  setLocale(localeOf(window.location.pathname));
  await preloadRoute(matchPath(window.location.pathname)?.id ?? 'notFound');
  const app = createElement(StrictMode, null, createElement(App));
  if (root.hasChildNodes() && !rewritten) {
    hydrateRoot(root, app);
    return 'hydrate';
  }
  root.replaceChildren();
  createRoot(root).render(app);
  return 'render';
}
