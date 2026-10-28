import { useLocale, useT } from '../i18n/i18n';
import { Head } from './Head';
import { Link } from './router';
import { pathFor } from './routes';

export function NotFoundPage() {
  const t = useT();
  const locale = useLocale();
  return (
    <section className="card prose">
      <Head notFound title={t('head.notFound.title')} description={t('head.notFound.description')} />
      <h1>{t('notFound.title')}</h1>
      <p>{t('notFound.text')}</p>
      <p><Link to={pathFor('home', locale)}>{t('notFound.home')}</Link></p>
    </section>
  );
}
