import { describe, it, expect } from 'vitest';
import { formatAt, formatDay, formatServerTime, localDateInput, parseLocalTime, worldDayLabel } from '../src/ui/format';
import { MS_PER_DAY, MS_PER_HOUR } from '../src/engine/types';

/** A world founded on 19.10.2026 at 09:00 local time (the origin is already shifted by the time zone, so we count in UTC). */
const ORIGIN = Date.UTC(2026, 9, 19, 9, 0, 0);

describe('world clock = local real time', () => {
  it('a new world shows the founding hour, not midnight', () => {
    expect(formatServerTime(ORIGIN, 0)).toBe('09:00:00 19.10.2026');
    expect(formatServerTime(ORIGIN, 15 * MS_PER_HOUR + 61_500)).toBe('00:01:01 20.10.2026');
  });

  it('moments relative to today\'s date', () => {
    const now = 2 * MS_PER_HOUR; // 11:00 19.10.
    expect(formatAt(ORIGIN, now + 3 * MS_PER_HOUR, now)).toBe('dziś o 14:00:00');
    expect(formatAt(ORIGIN, now + 14 * MS_PER_HOUR, now)).toBe('jutro o 01:00:00');
    expect(formatAt(ORIGIN, now - 12 * MS_PER_HOUR, now)).toBe('wczoraj o 23:00:00');
    expect(formatAt(ORIGIN, now + 2 * MS_PER_DAY + 250, now, true)).toBe('dnia 21.10. o 11:00:00:250');
  });

  it('date only, for rumours', () => {
    const now = MS_PER_DAY;
    expect(formatDay(ORIGIN, now, now)).toBe('dziś');
    expect(formatDay(ORIGIN, now - MS_PER_DAY, now)).toBe('wczoraj');
    expect(formatDay(ORIGIN, now - 3 * MS_PER_DAY, now)).toBe('17.10.');
  });

  it('the world day stays as a label', () => {
    expect(worldDayLabel(0)).toBe('Dzień 1');
    expect(worldDayLabel(MS_PER_DAY * 6 + 5)).toBe('Dzień 7');
  });

  it('date and time from a form → world millisecond', () => {
    const now = 2 * MS_PER_HOUR;
    expect(localDateInput(ORIGIN, now)).toBe('2026-10-19');
    expect(parseLocalTime(ORIGIN, now, '2026-10-19', '14:05:30.250')).toBe(5 * MS_PER_HOUR + 5 * 60_000 + 30_250);
    expect(parseLocalTime(ORIGIN, now, '', '14:05')).toBe(5 * MS_PER_HOUR + 5 * 60_000);
    expect(parseLocalTime(ORIGIN, now, '2026-10-20', '08:00:00')).toBe(23 * MS_PER_HOUR);
    expect(parseLocalTime(ORIGIN, now, '2026-10-19', '25:00')).toBeNull();
    expect(parseLocalTime(ORIGIN, now, 'jutro', '08:00')).toBeNull();
  });
});
