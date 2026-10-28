import { useMemo } from 'react';
import { MS_PER_DAY } from '../engine/types';
import { useGameStore } from '../state/gameStore';
import { formatAt, formatDay } from './format';

/** World moments as local time ("today at 14:05:30"). The component refreshes when the date changes, not on every clock tick. */
export function useClock() {
  const origin = useGameStore((s) => s.game!.clock.originLocalMs);
  const today = useGameStore((s) => Math.floor((s.game!.clock.originLocalMs + s.game!.now) / MS_PER_DAY));
  return useMemo(() => {
    const ref = today * MS_PER_DAY - origin;
    return {
      at: (t: number, withMs = false) => formatAt(origin, t, ref, withMs),
      day: (t: number) => formatDay(origin, t, ref),
    };
  }, [origin, today]);
}
