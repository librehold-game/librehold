import { useGameStore } from '../../state/gameStore';
import { PixelIcon } from '../pixelIcons';
import { selectNow, selectConfig } from '../../state/selectors';
import { formatServerTime, worldDayLabel } from '../format';
import { useT } from '../../i18n/i18n';

/** A separate component: it subscribes to `now`, so only it re-renders (4×/s), not the whole tree. */
export function WorldClock() {
  const t = useT();
  const now = useGameStore(selectNow);
  const origin = useGameStore((s) => s.game!.clock.originLocalMs);
  const speed = useGameStore((s) => selectConfig(s).worldSpeed);
  const paused = useGameStore((s) => selectConfig(s).paused);
  return (
    <span className="flex items-center gap-1 ml-auto text-muted tabular-nums" title={t('clock.title')}>
      <PixelIcon name="clock" size={16} />
      {formatServerTime(origin, now)}
      <span className="text-[10px]">{worldDayLabel(now)}</span>
      {speed !== 1 && <span className="text-gold font-semibold">×{speed}</span>}
      {paused && <span className="text-bad font-semibold">{t('clock.paused')}</span>}
    </span>
  );
}
