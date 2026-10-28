import { Head } from '../Head';
import { registerCatalog, useLocale, useT } from '../../i18n/i18n';
import enChangelog from '../../i18n/catalog/en/changelog.json';
import plChangelog from '../../i18n/catalog/pl/changelog.json';

registerCatalog('en', enChangelog);
registerCatalog('pl', plChangelog);

/** Newest first. */
export const CHANGELOG = ['2026-10-26', '2026-10-20', '2026-10-19', '2026-10-18', '2026-08-23', '2026-08-22'];

export function ChangelogPage() {
  const t = useT();
  const locale = useLocale();
  const date = new Intl.DateTimeFormat(locale, { dateStyle: 'long', timeZone: 'UTC' });
  return (
    <div className="encyclopedia">
      <Head title={t('changelog.title')} description={t('changelog.description')} />
      <article className="card prose changelog">
        <h1>{t('changelog.heading')}</h1>
        <p>{t('changelog.intro')}</p>
        {CHANGELOG.map((d) => (
          <section key={d} data-date={d}>
            <h2><time dateTime={d}>{date.format(new Date(`${d}T00:00:00Z`))}</time> — {t(`changelog.entries.${d}.title`)}</h2>
            <ul>{t(`changelog.entries.${d}.body`).split('\n').map((line, i) => <li key={i}>{line}</li>)}</ul>
          </section>
        ))}
      </article>
    </div>
  );
}

export default ChangelogPage;
