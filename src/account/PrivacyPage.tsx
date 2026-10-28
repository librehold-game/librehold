import { useLocale, useT } from '../i18n/i18n';
import '../i18n/reportCatalog';
import { Head } from './Head';
import { Link } from './router';
import { pathFor } from './routes';

const SECTIONS = ['account', 'actions', 'address', 'deletion', 'cookies'] as const;

export function PrivacyPage() {
  const t = useT();
  const locale = useLocale();
  return (
    <section className="card prose">
      <Head title={t('head.privacy.title')} description={t('head.privacy.description')} />
      <h1>{t('privacy.title')}</h1>
      <p>{t('privacy.intro')}</p>
      {SECTIONS.map((k) => <p key={k}>{t(`privacy.${k}`)}</p>)}
      <section>
        <h2>{t('privacy.reportsTitle')}</h2>
        <p>{t('privacy.reports')} <Link to={pathFor('report', locale)}>{t('report.button')}</Link></p>
      </section>
    </section>
  );
}

export default PrivacyPage;
