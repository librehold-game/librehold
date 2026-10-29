import { useEffect, useRef } from 'react';
import { formatDuration } from '../i18n/format';
import { useT } from '../i18n/i18n';
import { GameList } from './GameList';
import { usePlay } from './play';

/**
 * The front door: one click to the running instant games, as a guest when nobody is signed in. `waiting` holds it
 * while the account is not known yet, so a signed-in player never gets a second, guest account. Opening the page
 * with `?play` clicks it once.
 */
export function PlayButton({ signedIn, continueGame = false, waiting = false }: {
  signedIn: boolean; continueGame?: boolean; waiting?: boolean;
}) {
  const t = useT();
  const play = usePlay(signedIn);
  const auto = useRef(false);
  const { start } = play;
  useEffect(() => {
    if (waiting || auto.current || !new URLSearchParams(window.location.search).has('play')) return;
    auto.current = true;
    void start();
  }, [waiting, start]);
  return (
    <div className="play">
      <button type="button" className="cta" onClick={() => void play.start()} disabled={play.busy || play.counting || waiting}>{t(continueGame ? 'play.continue' : 'play.button')}</button>
      {play.list && (
        <GameList list={play.list} busy={play.busy} waitMs={play.waitMs} onJoin={(id) => void play.join(id)}
          onStartNew={() => void play.startNew()} onLeave={(id) => void play.leave(id)} />
      )}
      {!play.list && play.waitMs !== null && <p className="muted" role="status">{t('play.full', { time: formatDuration(play.waitMs) })}</p>}
      {play.error && <p className="error" role="alert">{play.error}</p>}
    </div>
  );
}
