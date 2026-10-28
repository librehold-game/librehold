/** How far past the last tick the world clock is extrapolated; a paused world stops after this. */
const MAX_AHEAD_MS = 500;
/** Shorter gaps between ticks say nothing reliable about the world's speed. */
const MIN_RATE_SAMPLE_MS = 100;

/** World time for the army markers between ticks, and the ticks themselves; the player's store or the god view's. */
export interface ArmyClock {
  now(): number;
  subscribe(onTick: () => void): () => void;
}

/** A clock over a source of world time: its last value moved on by the wall time since it changed. */
export function armyClock(read: () => number | null, subscribe: (onTick: () => void) => () => void): ArmyClock {
  const s = { now: Number.NaN, wall: 0, rate: 1 };
  return {
    now() {
      const now = read();
      if (now === null) return 0;
      const wall = typeof performance === 'undefined' ? Date.now() : performance.now();
      if (now !== s.now) {
        const dw = wall - s.wall, dn = now - s.now;
        if (!Number.isNaN(s.now) && dw >= MIN_RATE_SAMPLE_MS) s.rate = dn > 0 ? Math.min(20, dn / dw) : 1;
        s.now = now;
        s.wall = wall;
      }
      return now + Math.min(wall - s.wall, MAX_AHEAD_MS) * s.rate;
    },
    subscribe,
  };
}
