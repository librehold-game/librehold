import { BUILDING_BALANCE, BUILDINGS } from '../../data/buildings.data';
import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link } from '../router';
import { idForSlug } from '../routes';
import { breadcrumbList } from '../structuredData';
import { formatDuration, formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { BuildingId } from '../../engine/types';
import { BuildingSprite } from './BuildingsPage';
import { BUILDING_ORDER, buildingPath, neighbours, trail } from './entries';
import { buildingEffect, buildingLevels } from './tables';

export function BuildingPage({ id }: { id: BuildingId }) {
  const t = useT();
  const locale = useLocale();
  const def = BUILDINGS[id];
  const name = t(`building.${id}`);
  const items = trail(locale, 'buildings', { name, path: buildingPath(id, locale) });
  const levels = buildingLevels(id);
  const effect = buildingEffect(id);
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
        <p>{t(`encyclopedia.building.${id}.text`)}</p>
        <table className="stats">
          <tbody>
            <tr><th scope="row">{t('encyclopedia.label.maxLevel')}</th><td>{def.maxLevel}</td></tr>
            <tr>
              <th scope="row">{t('encyclopedia.label.requirements')}</th>
              <td>
                {def.requirements.length === 0 ? t('encyclopedia.label.none') : def.requirements.map((r, i) => (
                  <span key={r.building}>{i > 0 && ', '}<Link to={buildingPath(r.building, locale)}>{t(`building.${r.building}`)} {r.level}</Link></span>
                ))}
              </td>
            </tr>
          </tbody>
        </table>
        <h2>{t('encyclopedia.label.levels')}</h2>
        <div className="table-scroll">
          <table className="levels">
            <thead>
              <tr>
                {['level', 'wood', 'clay', 'iron', 'time', 'popTotal', 'pointsTotal'].map((k) => <th key={k}>{t(`encyclopedia.label.${k}`)}</th>)}
                {effect && <th>{t(`encyclopedia.label.effect.${effect}`)}</th>}
              </tr>
            </thead>
            <tbody>
              {levels.map((r) => (
                <tr key={r.level}>
                  <td>{r.level}</td><td>{n(r.cost.wood)}</td><td>{n(r.cost.clay)}</td><td>{n(r.cost.iron)}</td>
                  <td>{formatDuration(r.timeMs)}</td><td>{n(r.pop)}</td><td>{n(r.points)}</td>
                  {effect && <td>{effect === 'wall' ? t('encyclopedia.label.percent', { n: r.effect! }) : n(r.effect!)}</td>}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="muted">{t('encyclopedia.label.levelsCaption', { factor: BUILDING_BALANCE.mainTimeFactor })}</p>
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
