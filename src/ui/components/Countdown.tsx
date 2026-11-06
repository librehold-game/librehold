import { useGameStore } from '../../state/gameStore';
import { selectNow } from '../../state/selectors';
import { formatDuration, formatDurationHuman } from '../format';

const MS_PER_DAY = 86_400_000;

/** Countdown to `until` (world time). Subscribes to `now` rounded to seconds, so it re-renders once a second.
 * Under a day it ticks as H:MM:SS; from a full day on it shows a day counter ("3 d 4 h") so long expiries stay readable. */
export function Countdown({ until }: { until: number }) {
  const secondsLeft = useGameStore((s) => Math.max(0, Math.ceil((until - selectNow(s)) / 1000)));
  const ms = secondsLeft * 1000;
  return <span className="tabular-nums">{ms >= MS_PER_DAY ? formatDurationHuman(ms) : formatDuration(ms)}</span>;
}
