import { registerCatalog } from './i18n';
import enCommon from './catalog/en/common.json';
import plCommon from './catalog/pl/common.json';
import enAccount from './catalog/en/account.json';
import plAccount from './catalog/pl/account.json';
import enSite from './catalog/en/site.json';
import plSite from './catalog/pl/site.json';
import enGame from './catalog/en/game.json';
import plGame from './catalog/pl/game.json';

export function loadAccountCatalogs(): void {
  registerCatalog('en', enCommon);
  registerCatalog('pl', plCommon);
  registerCatalog('en', enAccount);
  registerCatalog('pl', plAccount);
  registerCatalog('en', enSite);
  registerCatalog('pl', plSite);
}

export function loadGameCatalogs(): void {
  registerCatalog('en', enCommon);
  registerCatalog('pl', plCommon);
  registerCatalog('en', enGame);
  registerCatalog('pl', plGame);
}
