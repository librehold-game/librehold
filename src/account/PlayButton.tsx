import { formatDuration } from '../i18n/format';
import { useT } from '../i18n/i18n';
import { usePlay } from './play';

/**
 * The front door: one click into an instant game, as a guest when nobody is signed in. `waiting` holds it while the
 * account is not known yet, so a signed-in player never gets a second, guest account.
 */
export function PlayButton({ signedIn, continueGame = false, waiting = false }: {
  signedIn: boolean; continueGame?: boolean; waiting?: boolean;
}) {
  const t = useT();
  const play = usePlay(signedIn);
  return (
    <div className="play">
      <button type="button" className="cta" onClick={() => void play.start()} disabled={play.busy || waiting}>{t(continueGame ? 'play.continue' : 'play.button')}</button>
      {play.waitMs !== null && <p className="muted" role="status">{t('play.full', { time: formatDuration(play.waitMs) })}</p>}
      {play.error && <p className="error" role="alert">{play.error}</p>}
    </div>
  );
}
