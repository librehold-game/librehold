// The report form's words, loaded only where a report is written or read (the site's front page stays light).
import { registerCatalog } from './i18n';
import en from './catalog/en/report.json';
import pl from './catalog/pl/report.json';

registerCatalog('en', en);
registerCatalog('pl', pl);
