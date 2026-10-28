import { loadAccountCatalogs, loadGameCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

// Tests written before translation assert Polish copy; tests of English copy switch with setLocale('en').
loadAccountCatalogs();
loadGameCatalogs();
setLocale('pl');
