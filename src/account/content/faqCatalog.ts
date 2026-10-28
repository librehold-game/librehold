// The FAQ page's own copy (the five answers the front page shows live in site.json).
import { registerCatalog } from '../../i18n/i18n';
import enFaq from '../../i18n/catalog/en/faq.json';
import plFaq from '../../i18n/catalog/pl/faq.json';

registerCatalog('en', enFaq);
registerCatalog('pl', plFaq);
