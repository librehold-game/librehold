import { getLocale, t } from './i18n';

const MS_PER_MINUTE = 60_000;
const MS_PER_HOUR = 3_600_000;
const MS_PER_DAY = 86_400_000;

const numberFormats = new Map<string, Intl.NumberFormat>();

export function formatNumber(n: number): string {
  const locale = getLocale();
  let f = numberFormats.get(locale);
  if (!f) numberFormats.set(locale, (f = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })));
  return f.format(Math.floor(n));
}

const decimalFormats = new Map<string, Intl.NumberFormat>();

/** A number with up to two decimals, for factors such as ×1.5. */
export function formatDecimal(n: number): string {
  const locale = getLocale();
  let f = decimalFormats.get(locale);
  if (!f) decimalFormats.set(locale, (f = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 })));
  return f.format(n);
}

/** H:MM:SS (hours may exceed 24). */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms)) return '∞';
  const total = Math.max(0, Math.ceil(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (x: number) => x.toString().padStart(2, '0');
  return `${h}:${pad(m)}:${pad(s)}`;
}

/** Short form such as "7 h 20 min" or "3 d 4 h". */
export function formatDurationHuman(ms: number): string {
  if (!Number.isFinite(ms)) return '∞';
  const d = Math.floor(ms / MS_PER_DAY);
  const h = Math.floor((ms % MS_PER_DAY) / MS_PER_HOUR);
  const m = Math.floor((ms % MS_PER_HOUR) / MS_PER_MINUTE);
  if (d > 0) return t('time.dh', { d, h });
  if (h > 0) return t('time.hm', { h, m });
  if (m > 0) return t('time.m', { m });
  return t('time.s', { s: Math.floor(ms / 1000) });
}

/*
 * The world clock is local wall time: world moment `t` is `origin + t`, where `origin` is `state.clock.originLocalMs`
 * (epoch ms already shifted by the zone offset, which is why everything below reads UTC fields).
 */
const pad2 = (x: number) => String(x).padStart(2, '0');
const localDate = (origin: number, at: number) => new Date(origin + at);
const dayIndex = (origin: number, at: number) => Math.floor((origin + at) / MS_PER_DAY);

function clockText(d: Date, withMs: boolean): string {
  const hms = `${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}:${pad2(d.getUTCSeconds())}`;
  return withMs ? `${hms}:${String(d.getUTCMilliseconds()).padStart(3, '0')}` : hms;
}

function dateText(d: Date, withYear: boolean): string {
  const day = pad2(d.getUTCDate());
  const month = pad2(d.getUTCMonth() + 1);
  if (getLocale() === 'en') return withYear ? `${d.getUTCFullYear()}-${month}-${day}` : `${month}/${day}`;
  return withYear ? `${day}.${month}.${d.getUTCFullYear()}` : `${day}.${month}.`;
}

/** Server time for the header: "09:14:05 19.10.2026". */
export function formatServerTime(origin: number, now: number): string {
  const d = localDate(origin, now);
  return `${clockText(d, false)} ${dateText(d, true)}`;
}

/** "today" / "tomorrow" / "yesterday" / a short date — the day of moment `at` relative to the day of `now`. */
export function formatDay(origin: number, at: number, now: number): string {
  const diff = dayIndex(origin, at) - dayIndex(origin, now);
  if (diff === 0) return t('time.today');
  if (diff === 1) return t('time.tomorrow');
  if (diff === -1) return t('time.yesterday');
  return dateText(localDate(origin, at), false);
}

/** "today at 14:05:30", "on 21.10. at …"; `withMs` appends ":250". */
export function formatAt(origin: number, at: number, now: number, withMs = false): string {
  const diff = dayIndex(origin, at) - dayIndex(origin, now);
  const time = clockText(localDate(origin, at), withMs);
  if (Math.abs(diff) <= 1) return t('time.dayAt', { day: formatDay(origin, at, now), time });
  return t('time.dateAt', { date: formatDay(origin, at, now), time });
}

/** Which day of the world it is — the small label next to the clock. */
export function worldDayLabel(now: number): string {
  return t('time.worldDay', { n: Math.floor(now / MS_PER_DAY) + 1 });
}

/** Today's date as an `<input type="date">` value ("2026-10-19"). */
export function localDateInput(origin: number, now: number): string {
  return localDate(origin, now).toISOString().slice(0, 10);
}

/** An `<input type="date">` value (empty = today) plus "14:05:30.250" → world millisecond; `null` on a bad format. */
export function parseLocalTime(origin: number, now: number, date: string, time: string): number | null {
  const dm = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim() || localDateInput(origin, now));
  const tm = /^(\d{1,2}):(\d{2})(?::(\d{2})(?:[.:](\d{1,3}))?)?$/.exec(time.trim());
  if (!dm || !tm) return null;
  const [h, min, sec, ms] = [Number(tm[1]), Number(tm[2]), Number(tm[3] ?? 0), Number((tm[4] ?? '0').padEnd(3, '0'))];
  if (h > 23 || min > 59 || sec > 59) return null;
  return Date.UTC(Number(dm[1]), Number(dm[2]) - 1, Number(dm[3]), h, min, sec, ms) - origin;
}

export function formatCoords(x: number, y: number): string {
  return `${x}|${y}`;
}
