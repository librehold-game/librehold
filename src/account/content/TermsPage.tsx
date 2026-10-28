import { Head } from '../Head';
import { pathFor } from '../routes';
import { registerCatalog, useLocale, useT } from '../../i18n/i18n';
import enTerms from '../../i18n/catalog/en/terms.json';
import plTerms from '../../i18n/catalog/pl/terms.json';
import facts from './facts.json';
import './faqCatalog';
import { RichText } from './rich';

registerCatalog('en', enTerms);
registerCatalog('pl', plTerms);

const SECTIONS = ['operator', 'accounts', 'fairPlay', 'moderation', 'worlds', 'warranty', 'contact', 'changes'];

export function TermsPage() {
  const t = useT();
  const locale = useLocale();
  const link = { to: pathFor('report', locale), label: t('site.faq.reportLink') };
  return (
    <div className="encyclopedia">
      <Head title={t('terms.title')} description={t('terms.description')} />
      <article className="card prose terms">
        <h1>{t('terms.heading')}</h1>
        <p className="muted">{t('terms.updated')}</p>
        {SECTIONS.map((s) => (
          <section key={s}>
            <h2>{t(`terms.sections.${s}.title`)}</h2>
            <RichText text={t(`terms.sections.${s}.body`, facts)} link={link} />
          </section>
        ))}
      </article>
    </div>
  );
}

export default TermsPage;
