import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { Link } from '../router';
import { pathFor, slugFor } from '../routes';
import { breadcrumbList } from '../structuredData';
import { t as tr, useLocale, useT, type Locale } from '../../i18n/i18n';
import { GUIDES, type GuideId } from './guides';
import { Paragraphs } from './text';

export const guidePath = (id: GuideId, l: Locale) => pathFor('guide', l, { guide: slugFor('guide', id, l) });

export function howToPlayTrail(locale: Locale, guide?: { name: string; path: string }) {
  const items = [
    { name: tr('encyclopedia.label.home'), path: pathFor('home', locale) },
    { name: tr('guides.breadcrumb'), path: pathFor('howToPlay', locale) },
  ];
  return guide ? [...items, guide] : items;
}

const SECTIONS = ['what', 'worlds', 'loop'];

export function HowToPlayPage() {
  const t = useT();
  const locale = useLocale();
  const items = howToPlayTrail(locale);
  return (
    <div className="encyclopedia guide">
      <Head title={t('guides.howToPlay.title')} description={t('guides.howToPlay.description')} jsonLd={[breadcrumbList(items)]} />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <article className="card prose">
        <h1>{t('guides.howToPlay.heading')}</h1>
        <p className="lead">{t('guides.howToPlay.lead')}</p>
        {SECTIONS.map((s) => (
          <section key={s}>
            <h2>{t(`guides.howToPlay.sections.${s}.title`)}</h2>
            <Paragraphs k={`guides.howToPlay.sections.${s}.body`} />
          </section>
        ))}
      </article>
      <section className="card">
        <h2>{t('guides.howToPlay.guidesTitle')}</h2>
        <ul className="guide-cards">
          {GUIDES.map((g) => (
            <li key={g.id}>
              <h3><Link to={guidePath(g.id, locale)}>{t(`guides.guide.${g.id}.title`)}</Link></h3>
              <p>{t(`guides.guide.${g.id}.lead`)}</p>
              <Link className="secondary" to={guidePath(g.id, locale)}>{t('guides.howToPlay.readGuide')}</Link>
            </li>
          ))}
        </ul>
      </section>
      <section className="card">
        <h2>{t('guides.howToPlay.lookUp')}</h2>
        <p>
          <Link to={pathFor('units', locale)}>{t('guides.howToPlay.unitsLink')}</Link>
          {' · '}
          <Link to={pathFor('buildings', locale)}>{t('guides.howToPlay.buildingsLink')}</Link>
        </p>
      </section>
    </div>
  );
}

export default HowToPlayPage;
