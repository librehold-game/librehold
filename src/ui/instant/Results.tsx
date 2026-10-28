import { useEffect, useState } from 'react';
import type { ArchivedGame } from '../../account/api';
import { usePlay } from '../../account/play';
import { pathFor } from '../../account/routes';
import { useLocale, useT } from '../../i18n/i18n';
import { api, navigation } from '../../net/api';
import { useGameStore } from '../../state/gameStore';
import { BotTag } from '../components/BotTag';
import { formatDuration, formatNumber } from '../format';
import { Overlay } from './Overlay';

/** The end of an instant game: who won, the final leaderboard, and what to do next. */
export function Results() {
  const t = useT();
  const locale = useLocale();
  const ended = useGameStore((s) => s.instant?.ended ?? null);
  const worldId = useGameStore((s) => s.worldId);
  const guest = useGameStore((s) => !!s.account?.isGuest);
  const openClaim = useGameStore((s) => s.openClaim);
  const hidden = useGameStore((s) => s.resultsHidden);
  const hide = useGameStore((s) => s.setResultsHidden);
  const play = usePlay(true);
  const [game, setGame] = useState<ArchivedGame | null>(null);
  const endedAt = ended?.at;
  const ref = ended?.resultsRef ?? (worldId === null ? null : String(worldId));
  useEffect(() => {
    if (endedAt === undefined || ref === null) return;
    api.get<ArchivedGame>(`/api/v1/public/games/${ref}`).then(setGame, () => setGame(null));
  }, [endedAt, ref]);
  if (!ended || hidden) return null;
  return (
    <Overlay titleId="results-title" wide>
      <div className="panel-header" id="results-title">{t('results.title')}</div>
      <div className="p-3 flex flex-col gap-3">
        <p>{t(`results.reason.${ended.reason}`, { winner: ended.winnerName })}{ended.winnerIsBot && <BotTag />}</p>
        {game && (
          <table aria-label={t('results.leaderboard')} className="w-full text-[13px]">
            <thead>
              <tr className="text-left text-muted">
                <th>#</th><th>{t('results.col.player')}</th><th>{t('results.col.points')}</th><th>{t('results.col.villages')}</th>
                <th>{t('results.col.conquered')}</th>
              </tr>
            </thead>
            <tbody>
              {game.rows.map((r) => (
                <tr key={r.rank}>
                  <td>{r.rank}</td>
                  <td>{r.name || t('player.former')}{r.isBot && <BotTag />}</td>
                  <td>{formatNumber(r.points)}</td><td>{r.villages}</td><td>{r.conquered}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {play.waitMs !== null && <p role="status" className="text-muted">{t('play.full', { time: formatDuration(play.waitMs) })}</p>}
        {play.error && <p role="alert" className="text-bad">{play.error}</p>}
        <div className="flex flex-wrap items-center gap-2">
          <a href={pathFor('game', locale, { id: ref ?? '' })}>{t('results.details')}</a>
          <a href={`${pathFor('game', locale, { id: ref ?? '' })}#watch`}>{t('results.watch')}</a>
          <span className="ml-auto flex flex-wrap gap-2">
            <button className="btn" onClick={() => hide(true)}>{t('results.close')}</button>
            {guest
              ? <button className="btn" onClick={openClaim}>{t('results.createAccount')}</button>
              : <button className="btn" onClick={() => navigation.assign(pathFor('lobby', locale))}>{t('results.longWorld')}</button>}
            <button className="btn" onClick={() => void play.start()} disabled={play.busy}>{t('results.playAgain')}</button>
          </span>
        </div>
      </div>
    </Overlay>
  );
}
