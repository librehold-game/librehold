import { useGameStore } from '../../state/gameStore';
import { selectNow } from '../../state/selectors';
import { formatDuration } from '../format';

/** Countdown to `until` (world time). Subscribes to `now` rounded to seconds, so it re-renders once a second. */
export function Countdown({ until }: { until: number }) {
  const secondsLeft = useGameStore((s) => Math.max(0, Math.ceil((until - selectNow(s)) / 1000)));
  return <span className="tabular-nums">{formatDuration(secondsLeft * 1000)}</span>;
}
