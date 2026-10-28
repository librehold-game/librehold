import { registerCatalog, t } from '../../i18n/i18n';
import enGuides from '../../i18n/catalog/en/guides.json';
import plGuides from '../../i18n/catalog/pl/guides.json';
import '../encyclopedia/catalog';
import { guideParams } from './params';

registerCatalog('en', enGuides);
registerCatalog('pl', plGuides);

/** A catalogue text with the game's numbers in it, as paragraphs (blank lines split them). */
export function Paragraphs({ k }: { k: string }) {
  return <>{t(k, guideParams()).split(/\n\s*\n/).map((p, i) => <p key={i}>{p}</p>)}</>;
}
