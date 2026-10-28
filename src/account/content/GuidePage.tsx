import { Fragment } from 'react';
import { Breadcrumbs } from '../Breadcrumbs';
import { buildingPath, unitPath } from '../encyclopedia/entries';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link } from '../router';
import { idForSlug } from '../routes';
import { breadcrumbList } from '../structuredData';
import { useLocale, useT } from '../../i18n/i18n';
import { GUIDES, type GuideDef } from './guides';
import { guidePath, howToPlayTrail } from './HowToPlayPage';
import { Paragraphs } from './text';

export function GuidePage({ guide }: { guide: GuideDef }) {
  const t = useT();
  const locale = useLocale();
  const key = `guides.guide.${guide.id}`;
  const title = t(`${key}.title`);
  const items = howToPlayTrail(locale, { name: title, path: guidePath(guide.id, locale) });
  const next = GUIDES[(GUIDES.indexOf(guide) + 1) % GUIDES.length]!;
  const first = guide.shots[0]!;
  return (
    <div className="encyclopedia guide">
      <Head
        title={t('guides.pageTitle', { title, app: t('app.name') })} description={t(`${key}.description`)}
        image={first.src} imageAlt={t(`${key}.shots.${first.id}.alt`)} jsonLd={[breadcrumbList(items)]}
      />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <article className="card prose">
        <h1>{title}</h1>
        <p className="lead">{t(`${key}.lead`)}</p>
        {guide.sections.map((s) => (
          <section key={s}>
            <h2>{t(`${key}.sections.${s}.title`)}</h2>
            <Paragraphs k={`${key}.sections.${s}.body`} />
            {guide.shots.filter((sh) => sh.after === s).map((sh) => (
              <figure key={sh.id} className="guide-shot">
                <img src={sh.src} width={sh.width} height={sh.height} loading="lazy" alt={t(`${key}.shots.${sh.id}.alt`)} />
                <figcaption>{t(`${key}.shots.${sh.id}.caption`)}</figcaption>
              </figure>
            ))}
          </section>
        ))}
        <h2>{t('guides.related')}</h2>
        <p>
          {guide.related.map((r, i) => (
            <Fragment key={`${r.kind}-${r.id}`}>
              {i > 0 && ' · '}
              {r.kind === 'unit'
                ? <Link to={unitPath(r.id, locale)}>{t(`unit.${r.id}`)}</Link>
                : <Link to={buildingPath(r.id, locale)}>{t(`building.${r.id}`)}</Link>}
            </Fragment>
          ))}
        </p>
        <nav className="entry-nav">
          <Link to={guidePath(next.id, locale)}>{t('guides.next', { title: t(`guides.guide.${next.id}.title`) })}</Link>
        </nav>
      </article>
    </div>
  );
}

export default function GuideRoute({ params }: PageProps) {
  const locale = useLocale();
  const id = idForSlug('guide', params.guide ?? '', locale);
  const guide = GUIDES.find((g) => g.id === id);
  return guide ? <GuidePage guide={guide} /> : <NotFoundPage />;
}
