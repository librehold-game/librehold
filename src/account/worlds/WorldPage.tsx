import { useEffect, useState } from 'react';
import { ApiError, api, errorText, type PublicMap, type PublicWorld } from '../api';
import { EventList } from '../EventList';
import { Head } from '../Head';
import { playersAndBots } from '../live';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { formatNumber } from '../../i18n/format';
import { loadHeadCatalog } from '../../i18n/headCatalog';
import { useT } from '../../i18n/i18n';
import { WorldMap } from './WorldMap';

loadHeadCatalog();

/** A long world's public page: its settings, the ten best, the map and the latest large battles. */
export function WorldPage({ id }: { id: number }) {
  const t = useT();
  const [world, setWorld] = useState<PublicWorld | null>(null);
  const [map, setMap] = useState<PublicMap | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    api.publicWorld(id).then(setWorld, (err) => {
      if (err instanceof ApiError && err.status === 404) setMissing(true);
      else setError(errorText(err));
    });
    api.publicMap(id).then(setMap, () => undefined);
  }, [id]);

  if (missing) return <NotFoundPage />;
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!world) return <p className="muted">{t('loading')}</p>;
  const s = world.settings;
  const settings: [string, string][] = [
    ['speed', `×${formatNumber(s.worldSpeed)}`],
    ['unitSpeed', `×${formatNumber(s.unitSpeed)}`],
    ['mapSize', `${s.mapSize}×${s.mapSize}`],
    ['protection', t('site.worlds.days', { count: s.beginnerProtectionDays })],
    ['lastVillage', t(`site.worlds.lastVillage.${s.lastVillage}`)],
    ['fieldBattles', t(s.fieldBattles ? 'site.worlds.yes' : 'site.worlds.no')],
  ];
  const status = world.paused ? 'paused' : world.status;
  return (
    <div className="world-page">
      <Head
        title={t('head.world.title', { name: world.name, day: world.day ?? 1, players: t('head.count.players', { count: world.humans }),
          bots: t('head.count.bots', { count: world.bots }) })}
        description={t('head.world.description')}
      />
      <section className="card">
        <h1>{world.name}</h1>
        <p className="muted">
          {t(`site.worlds.status.${status}`)} · {t('site.worlds.day', { day: world.day ?? 1 })} · {playersAndBots(world)}
        </p>
      </section>
      <div className="world-columns">
        <section className="card" aria-label={t('site.worlds.settings')}>
          <h2>{t('site.worlds.settings')}</h2>
          <dl className="settings-list">
            {settings.map(([k, v]) => (
              <div key={k}><dt>{t(`site.worlds.setting.${k}`)}</dt><dd>{v}</dd></div>
            ))}
          </dl>
        </section>
        {map && (
          <section className="card">
            <h2>{t('site.worlds.map')}</h2>
            <WorldMap map={map} name={world.name} />
          </section>
        )}
      </div>
      <section className="card">
        <h2>{t('site.worlds.top')}</h2>
        <table>
          <thead>
            <tr><th>#</th><th>{t('site.worlds.player')}</th><th>{t('site.worlds.points')}</th><th>{t('site.worlds.villages')}</th></tr>
          </thead>
          <tbody>
            {world.top.map((r) => (
              <tr key={r.rank}>
                <td>{r.rank}</td>
                <td>{r.name}{r.isBot && <span className="bot-tag">{t('archive.bot')}</span>}</td>
                <td>{formatNumber(r.points)}</td>
                <td>{formatNumber(r.villages)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="card">
        <h2>{t('site.worlds.events')}</h2>
        {world.events.length === 0 ? <p className="muted">{t('site.live.noEvents')}</p> : <EventList events={world.events} now={Date.now()} />}
      </section>
    </div>
  );
}

export default function WorldRoute({ params }: PageProps) {
  const id = Number(params.id);
  return /^\d+$/.test(params.id ?? '') && id > 0 ? <WorldPage id={id} /> : <NotFoundPage />;
}
