import { registerCatalog } from './i18n';
import enHead from './catalog/en/head.json';
import plHead from './catalog/pl/head.json';

let loaded = false;

/** Page-head copy for the server and the pages loaded on demand; kept out of the front page's bundle. */
export function loadHeadCatalog(): void {
  if (loaded) return;
  loaded = true;
  registerCatalog('en', enHead);
  registerCatalog('pl', plHead);
}
