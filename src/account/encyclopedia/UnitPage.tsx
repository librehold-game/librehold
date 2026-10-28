import { CONQUEST_BALANCE } from '../../data/conquest.data';
import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link } from '../router';
import { idForSlug } from '../routes';
import { breadcrumbList } from '../structuredData';
import { formatDuration, formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { UnitId } from '../../engine/types';
import { buildingPath, neighbours, trail, UNIT_ORDER, unitPath } from './entries';
import { unitFacts } from './tables';
import { UnitSprite } from './UnitsPage';

export function UnitPage({ id }: { id: UnitId }) {
  const t = useT();
  const locale = useLocale();
  const f = unitFacts(id);
  const name = t(`unit.${id}`);
  const items = trail(locale, 'units', { name, path: unitPath(id, locale) });
  const n = formatNumber;
  const rows: [string, React.ReactNode][] = [
    ['wood', n(f.cost.wood)], ['clay', n(f.cost.clay)], ['iron', n(f.cost.iron)], ['pop', n(f.pop)],
    ['attack', n(f.attack)], ['defense', n(f.defense)], ['defenseCavalry', n(f.defenseCavalry)],
    ['defenseArcher', n(f.defenseArcher)], ['speed', n(f.minutesPerField)], ['carry', n(f.carry)],
    ['recruitedIn', <Link to={buildingPath(f.building, locale)}>{t(`building.${f.building}`)}</Link>],
    ['recruitTime', formatDuration(f.recruitSec * 1000)],
    ['requirements', f.requirements.length === 0 ? t('encyclopedia.label.none') : f.requirements.map((r, i) => (
      <span key={r.building}>{i > 0 && ', '}<Link to={buildingPath(r.building, locale)}>{t(`building.${r.building}`)} {r.level}</Link></span>
    ))],
  ];
  if (id === 'noble') {
    const c = CONQUEST_BALANCE.coinBaseCost;
    rows.push(['coinCost', `${n(c.wood)} / ${n(c.clay)} / ${n(c.iron)}`],
      ['maxDistance', t('encyclopedia.label.fields', { n: CONQUEST_BALANCE.nobleMaxDistance })]);
  }
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
        <p>{t(`encyclopedia.unit.${id}.text`)}</p>
        <h2>{t('encyclopedia.label.stats')}</h2>
        <table className="stats">
          <tbody>
            {rows.map(([k, v]) => <tr key={k}><th scope="row">{t(`encyclopedia.label.${k}`)}</th><td>{v}</td></tr>)}
          </tbody>
        </table>
        <p className="muted">{t('encyclopedia.label.unitCaption', { building: t(`building.${f.building}`) })}</p>
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
