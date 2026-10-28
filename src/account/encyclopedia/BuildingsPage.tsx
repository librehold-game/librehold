import { BUILDINGS } from '../../data/buildings.data';
import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { Link } from '../router';
import { breadcrumbList } from '../structuredData';
import { formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { BuildingId } from '../../engine/types';
import { BUILDING_ORDER, buildingPath, trail } from './entries';
import { BUILDING_SPRITES } from './sprites';
import { buildingLevels } from './tables';

export function BuildingSprite({ id }: { id: BuildingId }) {
  if (id === 'wall') return <span className="building-sprite wall-sprite" style={{ backgroundImage: `url(${BUILDING_SPRITES.wall})` }} />;
  return <img className="building-sprite" src={BUILDING_SPRITES[id]} alt="" loading="lazy" />;
}

export function BuildingsPage() {
  const t = useT();
  const locale = useLocale();
  const items = trail(locale, 'buildings');
  return (
    <div className="encyclopedia">
      <Head title={t('encyclopedia.buildings.title')} description={t('encyclopedia.buildings.description')} jsonLd={[breadcrumbList(items)]} />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <section className="card">
        <h1>{t('encyclopedia.buildings.heading')}</h1>
        <p>{t('encyclopedia.buildings.intro')}</p>
        <ul className="entry-cards">
          {BUILDING_ORDER.map((id) => {
            const top = buildingLevels(id).at(-1)!;
            return (
              <li key={id}>
                <Link to={buildingPath(id, locale)} className="entry-card">
                  <BuildingSprite id={id} />
                  <strong>{t(`building.${id}`)}</strong>
                  <span className="muted">{t('encyclopedia.label.card.building', { level: BUILDINGS[id].maxLevel, points: formatNumber(top.points) })}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </section>
    </div>
  );
}

export default BuildingsPage;
