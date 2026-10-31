import { useEffect, useMemo, useState } from 'react';
import { ApiError, api, errorText, type PublicMap, type PublicWorld } from '../api';
import { EventList } from '../EventList';
import { Head } from '../Head';
import { playersAndBots } from '../live';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link, navigate } from '../router';
import { pathFor } from '../routes';
import { loadHeadCatalog } from '../../i18n/headCatalog';
import { useLocale, useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { RankingTable, type RankingData, type RankingHost } from '../../shared/ranking/RankingTable';
import type { ConquestRow, DailyRow, PlayerRow } from '../../shared/ranking/types';
import { WorldMap } from './WorldMap';
import { WorldSettings } from './WorldSettings';
import './catalog';

loadHeadCatalog();

/** A world's public page: its settings, the full ranking, the map and the latest large battles. */
export function WorldPage({ id }: { id: number }) {
  const t = useT();
  const locale = useLocale();
  const [world, setWorld] = useState<PublicWorld | null>(null);
  const [map, setMap] = useState<PublicMap | null>(null);
  const [ranking, setRanking] = useState<RankingData | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    api.publicWorld(id).then(setWorld, (err) => {
      if (!(err instanceof ApiError && err.status === 404)) {
        setError(errorText(err));
        return;
      }
      api.publicGame(String(id)).then(() => navigate(pathFor('game', locale, { id: String(id) }), true), () => setMissing(true));
    });
    api.publicMap(id).then(setMap, () => undefined);
    Promise.all([api.publicRanking(id, 'points'), api.publicRanking(id, 'daily'), api.publicRanking(id, 'conquests')]).then(
      ([players, daily, conquests]) => setRanking({ players: players as PlayerRow[], daily: daily as DailyRow[], conquests: conquests as ConquestRow[] }),
      () => undefined,
    );
  }, [id, locale]);

  const host = useMemo<RankingHost>(() => {
    const when = new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' });
    return {
      renderPlayer: (p) => <Link to={pathFor('worldPlayer', locale, { id: String(id), pid: p.id })}>{p.name}</Link>,
      isMe: () => false,
      formatTime: (ms) => when.format(ms),
      refreshNote: t('site.worlds.rankingNote'),
    };
  }, [id, locale, t]);

  if (missing) return <NotFoundPage />;
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!world) return <p className="muted">{t('loading')}</p>;
  const status = world.paused ? 'paused' : world.status;
  const name = worldDisplayName(world);
  return (
    <div className="world-page">
      <Head
        title={world.instant
          ? t('head.instantWorld.title', { name, players: t('head.count.players', { count: world.humans }), bots: t('head.count.bots', { count: world.bots }) })
          : t('head.world.title', { name, day: world.day ?? 1, players: t('head.count.players', { count: world.humans }), bots: t('head.count.bots', { count: world.bots }) })}
        description={t(world.instant ? 'head.instantWorld.description' : 'head.world.description')}
        noIndex={world.instant}
      />
      <section className="card">
        <h1>{name}</h1>
        <p className="muted">
          {t(`site.worlds.status.${status}`)} · {world.instant ? '' : `${t('site.worlds.day', { day: world.day ?? 1 })} · `}{playersAndBots(world)}
        </p>
      </section>
      <div className="world-columns">
        <section className="card" aria-label={t('site.worlds.settings')}>
          <h2>{t('site.worlds.settings')}</h2>
          <WorldSettings settings={world.settings} instant={world.instant} />
        </section>
        {map && (
          <section className="card">
            <h2>{t('site.worlds.map')}</h2>
            <WorldMap map={map} name={name} />
          </section>
        )}
      </div>
      {ranking && (
        <section className="card" aria-label={t('ranking.title')}>
          <RankingTable data={ranking} host={host} />
        </section>
      )}
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
