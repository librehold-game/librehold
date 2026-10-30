import { BUILDINGS } from '../../data/buildings.data';
import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link } from '../router';
import { idForSlug } from '../routes';
import { breadcrumbList } from '../structuredData';
import { formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { BuildingId } from '../../engine/types';
import { BuildingSprite } from '../../shared/encyclopedia/sections';
import { BUILDING_ORDER, buildingPath, neighbours, trail } from './entries';
import { BuildingEntry } from '../../shared/encyclopedia/entries';
import { BASE_TIMING, buildingLevels } from '../../shared/encyclopedia/facts';
import { SiteEntryLinks } from './SiteEntryLinks';

export function BuildingPage({ id }: { id: BuildingId }) {
  const t = useT();
  const locale = useLocale();
  const def = BUILDINGS[id];
  const name = t(`building.${id}`);
  const items = trail(locale, 'buildings', { name, path: buildingPath(id, locale) });
  const levels = buildingLevels(id);
  const n = formatNumber;
  const first = levels[0]!.cost;
  const [prev, next] = neighbours(BUILDING_ORDER, id);
  return (
    <div className="encyclopedia">
      <Head
        title={t('encyclopedia.buildingTitle', { name, app: t('app.name') })}
        description={t('encyclopedia.buildingMeta', { name, maxLevel: def.maxLevel, wood: n(first.wood), clay: n(first.clay), iron: n(first.iron) })}
        image={`/og/building-${id}.png`} imageAlt={name} jsonLd={[breadcrumbList(items)]}
      />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <article className="card entry">
        <div className="entry-head">
          <BuildingSprite id={id} />
          <h1>{name}</h1>
        </div>
        <SiteEntryLinks><BuildingEntry id={id} timing={BASE_TIMING} levelsOpen /></SiteEntryLinks>
        <nav className="entry-nav">
          <Link to={buildingPath(prev, locale)}>{t('encyclopedia.label.prev', { name: t(`building.${prev}`) })}</Link>
          <Link to={buildingPath(next, locale)}>{t('encyclopedia.label.next', { name: t(`building.${next}`) })}</Link>
        </nav>
      </article>
    </div>
  );
}

export default function BuildingRoute({ params }: PageProps) {
  const locale = useLocale();
  const id = idForSlug('building', params.building ?? '', locale) as BuildingId | null;
  return id ? <BuildingPage id={id} /> : <NotFoundPage />;
}
