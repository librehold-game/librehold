import { useEffect, useState, type ReactNode } from 'react';
import { api, errorText, type PublicWorldCard, type PublicWorldList } from '../api';
import { Head } from '../Head';
import { playersAndBots } from '../live';
import { Link } from '../router';
import { pathFor } from '../routes';
import { loadHeadCatalog } from '../../i18n/headCatalog';
import { formatDurationHuman } from '../../i18n/format';
import { useLocale, useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';

loadHeadCatalog();

function Section({ title, empty, children }: { title: string; empty: string; children: ReactNode[] }) {
  return (
    <section className="card worlds-section" aria-label={title}>
      <h2>{title}</h2>
      {children.length === 0 ? <p className="muted">{empty}</p> : <ul className="world-cards">{children}</ul>}
    </section>
  );
}

/** Every world anyone may look at: instant games running now, long worlds, and the latest finished games. */
export function WorldsPage() {
  const t = useT();
  const locale = useLocale();
  const [list, setList] = useState<PublicWorldList | null>(null);
  const [error, setError] = useState('');
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    api.publicWorlds().then((l) => {
      setList(l);
      setNow(Date.now());
    }, (err) => setError(errorText(err)));
  }, []);
  const date = new Intl.DateTimeFormat(locale, { dateStyle: 'medium' });
  const status = (w: PublicWorldCard) => t(w.paused ? 'site.worlds.status.paused' : `site.worlds.status.${w.status}`);

  return (
    <div className="worlds-page">
      <Head title={t('head.worlds.title')} description={t('head.worlds.description')} />
      <h1>{t('site.worlds.title')}</h1>
      {error && <p className="error" role="alert">{error}</p>}
      {!list && !error && <p className="muted">{t('loading')}</p>}
      {list && (
        <>
          <Section title={t('site.worlds.running')} empty={t('site.worlds.noRunning')}>
            {list.running.map((w) => (
              <li key={w.id} className="world-card">
                <h3><Link to={pathFor('game', locale, { id: String(w.id) })}>{worldDisplayName(w)}</Link></h3>
                <p>{playersAndBots(w)}</p>
                <p className="muted">{t('site.worlds.timeLeft', { time: formatDurationHuman(Math.max(0, (w.endsAt ?? now) - now)) })}</p>
              </li>
            ))}
          </Section>
          <Section title={t('site.worlds.long')} empty={t('site.worlds.noLong')}>
            {list.long.map((w) => (
              <li key={w.id} className="world-card">
                <h3><Link to={pathFor('world', locale, { id: String(w.id) })}>{w.name}</Link></h3>
                <p className="muted">{status(w)} · {t('site.home.worlds.speed', { speed: w.speed })} · {t('site.worlds.day', { day: w.day ?? 1 })}</p>
                <p>{playersAndBots(w)}</p>
              </li>
            ))}
          </Section>
          <Section title={t('site.worlds.finished')} empty={t('site.worlds.noFinished')}>
            {list.finished.map((g) => (
              <li key={g.worldId} className="world-card">
                <h3><Link to={pathFor('game', locale, { id: String(g.worldId) })}>{worldDisplayName(g)}</Link></h3>
                <p className="muted">{t('site.worlds.ended', { date: date.format(g.endedAt) })}</p>
              </li>
            ))}
          </Section>
        </>
      )}
    </div>
  );
}

export default WorldsPage;
