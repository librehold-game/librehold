/** How long the whole game takes to replay, in seconds. */
export const LENGTHS = [30, 60, 120] as const;
export type Length = (typeof LENGTHS)[number];

export interface PlaybackState { playing: boolean; length: Length; t: number }

/** Moves the replay on by realMs of wall time; it stops at the end. */
export function advance(p: PlaybackState, realMs: number, endAt: number): PlaybackState {
  const t = Math.min(endAt, p.t + (realMs * endAt) / (p.length * 1000));
  return { ...p, t, playing: p.playing && t < endAt };
}

/** How long, in world time, a conquest flashes and a battle line shows: 0.6 s of the replay. */
export function effectMsFor(length: Length, endAt: number): number {
  return (endAt * 0.6) / length;
}

export function initialPlayback(reducedMotion: boolean): PlaybackState {
  return { playing: !reducedMotion, length: 60, t: 0 };
}
