import { useEffect, useState } from 'react';
import { formatDuration } from '../i18n/format';
import { useT } from '../i18n/i18n';
import type { InstantList } from './api';

const TICK_MS = 15_000;

/** The running instant games to pick from; the player's own is first, and a player can be in one game at a time. */
export function GameList({ list, busy, waitMs, onJoin, onStartNew, onLeave }: {
  list: InstantList; busy: boolean; waitMs: number | null;
  onJoin: (id: number) => void; onStartNew: () => void; onLeave: (id: number) => void;
}) {
  const t = useT();
  const [now, setNow] = useState(() => Date.now());
  const [confirming, setConfirming] = useState<number | null>(null);
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);
  const inGame = list.games.some((g) => g.mine);
  return (
    <div className="game-list">
      <h2>{t('play.pick')}</h2>
      <table aria-label={t('play.pick')}>
        <thead>
          <tr><th>{t('play.game')}</th><th>{t('play.timeLeft')}</th><th>{t('play.players')}</th><th>{t('play.leader')}</th><th /></tr>
        </thead>
        <tbody>
          {list.games.map((g) => {
            const full = g.humans >= g.seats;
            return (
              <tr key={g.id} className={g.mine ? 'mine' : full ? 'full' : undefined}>
                <td data-label={t('play.game')}>
                  {t('world.gameName', { n: g.gameNumber })}{g.mine && <> <strong>{t('play.yours')}</strong></>}
                </td>
                <td data-label={t('play.timeLeft')}>{t('play.minutes', { n: Math.max(1, Math.ceil((g.endsAt - now) / 60_000)) })}</td>
                <td data-label={t('play.players')}>{g.humans} / {g.seats}</td>
                <td data-label={t('play.leader')}>{g.leader ? `${g.leader} · ${g.leaderPoints}` : '–'}</td>
                <td className="actions">
                  {g.mine ? (
                    <>
                      <button type="button" onClick={() => onJoin(g.id)} disabled={busy}>{t('play.resume')}</button>
                      <button type="button" className="secondary" disabled={busy}
                        onClick={() => (confirming === g.id ? (setConfirming(null), onLeave(g.id)) : setConfirming(g.id))}>
                        {t(confirming === g.id ? 'play.leaveConfirm' : 'play.leave')}
                      </button>
                    </>
                  ) : full ? <span className="muted">{t('play.fullRow')}</span>
                    : !inGame && <button type="button" onClick={() => onJoin(g.id)} disabled={busy}>{t('play.join')}</button>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {inGame && <p className="muted">{t('play.inAnotherGame')}</p>}
      {list.canStart && <button type="button" onClick={onStartNew} disabled={busy}>{t('play.startNew')}</button>}
      {waitMs !== null && <p className="muted" role="status">{t('play.full', { time: formatDuration(waitMs) })}</p>}
    </div>
  );
}
