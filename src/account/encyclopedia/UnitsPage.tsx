import type { CSSProperties } from 'react';
import { Breadcrumbs } from '../Breadcrumbs';
import { Head } from '../Head';
import { Link } from '../router';
import { breadcrumbList } from '../structuredData';
import { formatNumber } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import type { UnitId } from '../../engine/types';
import { trail, UNIT_ORDER, unitPath } from './entries';
import { UNIT_SPRITES } from '../../shared/encyclopedia/sprites';
import { unitFacts } from '../../shared/encyclopedia/facts';
import { UnitSprite } from '../../shared/encyclopedia/sections';

export function UnitsPage() {
  const t = useT();
  const locale = useLocale();
  const items = trail(locale, 'units');
  return (
    <div className="encyclopedia">
      <Head title={t('encyclopedia.units.title')} description={t('encyclopedia.units.description')} jsonLd={[breadcrumbList(items)]} />
      <Breadcrumbs items={items} label={t('encyclopedia.label.breadcrumbs')} />
      <section className="card">
        <h1>{t('encyclopedia.units.heading')}</h1>
        <p>{t('encyclopedia.units.intro')}</p>
        <ul className="entry-cards">
          {UNIT_ORDER.map((id) => {
            const f = unitFacts(id);
            return (
              <li key={id}>
                <Link to={unitPath(id, locale)} className="entry-card">
                  <UnitSprite id={id} />
                  <strong>{t(`unit.${id}`)}</strong>
                  <span className="muted">{t('encyclopedia.label.card.unit', { attack: formatNumber(f.attack), defense: formatNumber(f.defense) })}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </section>
    </div>
  );
}

export default UnitsPage;
