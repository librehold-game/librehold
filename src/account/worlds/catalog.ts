// The words of the worlds pages; kept out of the front page's bundle, so each of those pages imports this.
import { registerCatalog } from '../../i18n/i18n';
import enWorlds from '../../i18n/catalog/en/worlds.json';
import plWorlds from '../../i18n/catalog/pl/worlds.json';

registerCatalog('en', enWorlds);
registerCatalog('pl', plWorlds);
