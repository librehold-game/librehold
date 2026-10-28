// The words of hosting a game and of its errors, loaded where a game is hosted or joined (the site's front page
// stays light).
import { registerCatalog } from './i18n';
import en from './catalog/en/host.json';
import pl from './catalog/pl/host.json';

registerCatalog('en', en);
registerCatalog('pl', pl);
