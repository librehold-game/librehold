import { useEffect, useState } from 'react';
import { useT } from '../../i18n/i18n';
import type { HostedView } from '../../net/types';
import { useGameStore } from '../../state/gameStore';
import { formatDuration } from '../format';
import { Overlay } from './Overlay';

/** A player world before its host starts it: who is here, and the whole game still ahead. */
export function Waiting() {
  const t = useT();
  const hosted = useGameStore((s) => s.hosted);
  const left = useGameStore((s) => (s.instant && s.game ? Math.max(0, s.instant.endsAt - s.game.now) : null));
  if (!hosted?.waiting) return null;
  return (
    <Overlay titleId="waiting-title">
      <div className="panel-header" id="waiting-title">{t('waiting.title')}</div>
      <div className="p-3 flex flex-col gap-2">
        <p>{t('waiting.text', { host: hosted.hostName, count: hosted.humans })}</p>
        {left !== null && <p className="text-muted">{t('waiting.left')} <strong role="timer">{formatDuration(left)}</strong></p>}
      </div>
    </Overlay>
  );
}

/** The host's pause: by whom, and when the game goes on by itself. */
export function HostPause({ hosted }: { hosted: HostedView }) {
  const t = useT();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  return (
    <div role="status" className="panel mt-2 px-3 py-2 text-center font-semibold text-bad">
      {t('waiting.hostPause', { host: hosted.hostName, time: formatDuration(Math.max(0, (hosted.resumesAt ?? now) - now)) })}
    </div>
  );
}
