import { Head } from '../Head';
import { pathFor } from '../routes';
import { useLocale, useT } from '../../i18n/i18n';
import facts from './facts.json';
import './faqCatalog';
import { withLink } from './rich';

export const FAQ_ORDER = ['free', 'payToWin', 'bots', 'phone', 'time', 'instantVsLong', 'guestVsAccount', 'away', 'protection',
  'languages', 'report'];

export function FaqPage() {
  const t = useT();
  const locale = useLocale();
  const link = { to: pathFor('report', locale), label: t('site.faq.reportLink') };
  return (
    <div className="encyclopedia">
      <Head title={t('site.faq.title')} description={t('site.faq.description')} />
      <article className="card prose faq">
        <h1>{t('site.faq.heading')}</h1>
        <dl>
          {FAQ_ORDER.map((id) => (
            <div key={id}>
              <dt>{t(`site.faq.items.${id}.q`)}</dt>
              <dd data-id={id}>{withLink(t(`site.faq.items.${id}.a`, facts), link)}</dd>
            </div>
          ))}
        </dl>
      </article>
    </div>
  );
}

export default FaqPage;
