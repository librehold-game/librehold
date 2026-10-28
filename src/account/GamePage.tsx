import { Suspense, lazy, useEffect, useState } from 'react';
import { formatDurationHuman, formatNumber } from '../i18n/format';
import { useLocale, useT } from '../i18n/i18n';
import { worldDisplayName } from '../i18n/names';
import { ApiError, api, errorText, type ArchivedGame, type LeaderRow } from './api';
import { Head } from './Head';
import { loadHeadCatalog } from '../i18n/headCatalog';
import '../i18n/hostCatalog';
import '../i18n/timelapseCatalog';
import { NotFoundPage } from './NotFoundPage';
import type { PageProps } from './pages';
import { formatDate } from './ProfilePage';
import { Link } from './router';
import { pathFor } from './routes';

loadHeadCatalog();

const TimelapsePlayer = lazy(() => import('./timelapse/TimelapsePlayer').then((m) => ({ default: m.TimelapsePlayer })));

function BotMark() {
  const t = useT();
  return <span className="bot-tag">{t('archive.bot')}</span>;
}

function PlayerName({ row }: { row: LeaderRow }) {
  const t = useT();
  const locale = useLocale();
  if (row.isBot) return <>{row.name} <BotMark /></>;
  if (!row.name) return <span className="muted">{t('archive.formerPlayer')}</span>;
  return <Link to={pathFor('player', locale, { name: row.name })}>{row.name}</Link>;
}

/** A finished game, by its id or a player world's results code: how it ended and its final leaderboard. */
export function GamePage({ gameRef }: { gameRef: string }) {
  const t = useT();
  const [game, setGame] = useState<ArchivedGame | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const [watching, setWatching] = useState(() => typeof location !== 'undefined' && location.hash === '#watch');
  useEffect(() => {
    setGame(null);
    setFailure(null);
    api.publicGame(gameRef).then(setGame, setFailure);
  }, [gameRef]);

  if (failure) {
    if (failure instanceof ApiError && failure.status === 404) return <NotFoundPage />;
    return <p className="error" role="alert">{errorText(failure)}</p>;
  }
  if (!game) return <p className="muted">{t('loading')}</p>;
  const s = game.summary;
  const params = {
    name: worldDisplayName(game),
    winner: game.winnerName || t('head.formerPlayer'),
    players: t('head.count.players', { count: s.humans }),
    conquests: t('head.count.conquests', { count: s.conquestsHumans + s.conquestsBots }),
    bots: t('head.count.bots', { count: s.bots }),
  };
  const head = game.running
    ? <Head title={t('head.game.running.title', params)} description={t('head.game.running.description', params)} />
    : <Head title={t(game.winnerIsBot ? 'head.game.ended.titleBot' : 'head.game.ended.title', params)}
      description={t('head.game.ended.description', params)} />;
  return (
    <div className="stack">
      {head}
      <section className="card">
        <h1>{params.name}</h1>
        {game.running ? (
          <p>{t('archive.running', { time: formatDurationHuman(Math.max(0, (game.endsAt ?? 0) - Date.now())) })}</p>
        ) : (
          <>
            <p className="muted">{formatDate(game.endedAt)}</p>
            <p>
              {t(`archive.reason.${s.endReason}`, { winner: game.winnerName || t('archive.formerPlayer') })}
              {game.winnerIsBot && <> <BotMark /></>}
            </p>
          </>
        )}
        <p className="muted">{t('archive.summary', {
          humans: s.humans, bots: s.bots, conquests: formatNumber(s.conquestsHumans + s.conquestsBots),
          attacks: formatNumber(s.attacksSent), killed: formatNumber(s.humanUnitsKilled),
        })}</p>
      </section>
      {!game.running && (
        <section className="card">
          {watching ? (
            <Suspense fallback={<p className="muted">{t('timelapse.loading')}</p>}>
              <TimelapsePlayer gameRef={gameRef} game={game} />
            </Suspense>
          ) : (
            <button className="btn" onClick={() => setWatching(true)}>{t('timelapse.watch')}</button>
          )}
        </section>
      )}
      <section className="card">
        <table aria-label={t('archive.leaderboard')}>
          <thead>
            <tr>
              <th>#</th><th>{t('archive.col.player')}</th><th>{t('archive.col.points')}</th><th>{t('archive.col.villages')}</th>
              <th>{t('archive.col.conquered')}</th><th>{t('archive.col.killed')}</th>
            </tr>
          </thead>
          <tbody>
            {game.rows.map((r) => (
              <tr key={r.rank}>
                <td data-label="#">{r.rank}</td>
                <td data-label={t('archive.col.player')}><PlayerName row={r} /></td>
                <td data-label={t('archive.col.points')}>{formatNumber(r.points)}</td>
                <td data-label={t('archive.col.villages')}>{r.villages}</td>
                <td data-label={t('archive.col.conquered')}>{r.conquered}</td>
                <td data-label={t('archive.col.killed')}>{formatNumber(r.killed)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}

export default function GameRoute({ params }: PageProps) {
  return <GamePage gameRef={params.id ?? ''} />;
}
