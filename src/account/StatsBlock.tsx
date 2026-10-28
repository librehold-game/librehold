import { useEffect, useState } from 'react';
import { formatNumber } from '../i18n/format';
import { useT } from '../i18n/i18n';
import { api, type PublicStats } from './api';

const ITEMS = ['games', 'players', 'conquests', 'unitsKilled'] as const;

/** Real totals from finished instant games; hidden until there are enough of them to mean something. */
export function StatsBlock() {
  const t = useT();
  const [stats, setStats] = useState<PublicStats | null>(null);
  useEffect(() => {
    api.publicStats().then(setStats, () => setStats(null));
  }, []);
  if (!stats?.visible) return null;
  return (
    <section className="card stats" aria-label={t('stats.label')}>
      <h2>{t('stats.label')}</h2>
      <dl className="stat-grid">
        {ITEMS.map((k) => (
          <div key={k}>
            <dt>{t(`stats.${k}`)}</dt>
            <dd>{formatNumber(stats.allTime[k])}</dd>
          </div>
        ))}
      </dl>
      <p className="muted">{t('stats.last24h', { games: formatNumber(stats.last24h.games), conquests: formatNumber(stats.last24h.conquests) })}</p>
    </section>
  );
}
