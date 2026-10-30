import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link } from '../router';
import { idForSlug } from '../routes';
import { breadcrumbList } from '../structuredData';
import { formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { UnitId } from '../../engine/types';
import { neighbours, trail, UNIT_ORDER, unitPath } from './entries';
import { UnitEntry } from '../../shared/encyclopedia/entries';
import { BASE_TIMING, unitFacts } from '../../shared/encyclopedia/facts';
import { UnitSprite } from '../../shared/encyclopedia/sections';
import { SiteEntryLinks } from './SiteEntryLinks';

export function UnitPage({ id }: { id: UnitId }) {
  const t = useT();
  const locale = useLocale();
  const f = unitFacts(id);
  const name = t(`unit.${id}`);
  const items = trail(locale, 'units', { name, path: unitPath(id, locale) });
  const n = formatNumber;
  const [prev, next] = neighbours(UNIT_ORDER, id);
  return (
    <div className="encyclopedia">
      <Head
        title={t('encyclopedia.unitTitle', { name, app: t('app.name') })}
        description={t('encyclopedia.unitMeta', { name, wood: n(f.cost.wood), clay: n(f.cost.clay), iron: n(f.cost.iron),
          attack: n(f.attack), defense: n(f.defense), speed: n(f.minutesPerField) })}
        image={`/og/unit-${id}.png`} imageAlt={name} jsonLd={[breadcrumbList(items)]}
      />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <article className="card entry">
        <div className="entry-head">
          <UnitSprite id={id} />
          <h1>{name}</h1>
        </div>
        <SiteEntryLinks><UnitEntry id={id} timing={BASE_TIMING} /></SiteEntryLinks>
        <nav className="entry-nav">
          <Link to={unitPath(prev, locale)}>{t('encyclopedia.label.prev', { name: t(`unit.${prev}`) })}</Link>
          <Link to={unitPath(next, locale)}>{t('encyclopedia.label.next', { name: t(`unit.${next}`) })}</Link>
        </nav>
      </article>
    </div>
  );
}

export default function UnitRoute({ params }: PageProps) {
  const locale = useLocale();
  const id = idForSlug('unit', params.unit ?? '', locale) as UnitId | null;
  return id ? <UnitPage id={id} /> : <NotFoundPage />;
}
