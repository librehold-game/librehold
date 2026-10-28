import { MS_PER_DAY, MS_PER_MINUTE, type GameState } from './types';

/**
 * The player's local wall clock. World time runs 1:1 with real time, so world moment `t` is
 * `clock.originLocalMs + t` of local time (ms since epoch, already shifted by the time zone).
 */

export function localDayIndex(state: GameState, t: number): number {
  return Math.floor((state.clock.originLocalMs + t) / MS_PER_DAY);
}

export function localMinuteOfDay(state: GameState, t: number): number {
  const ms = (state.clock.originLocalMs + t) % MS_PER_DAY;
  return Math.floor((ms < 0 ? ms + MS_PER_DAY : ms) / MS_PER_MINUTE);
}

/** Saturday or Sunday of the local date. */
export function isWeekend(state: GameState, t: number): boolean {
  return isWeekendDay(localDayIndex(state, t));
}

export function isWeekendDay(dayIndex: number): boolean {
  const dow = new Date(dayIndex * MS_PER_DAY).getUTCDay();
  return dow === 0 || dow === 6;
}

/** World moment for minute `min` of local day `dayIndex` (`min` may exceed 1440). */
export function worldTimeOf(state: GameState, dayIndex: number, min: number): number {
  return dayIndex * MS_PER_DAY + min * MS_PER_MINUTE - state.clock.originLocalMs;
}
