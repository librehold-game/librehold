import { useEffect, useState } from 'react';
import { api, type LiveStrip as Live } from './api';
import { EventRow } from './EventList';
import { agoText } from './live';
import { useT } from '../i18n/i18n';

const ROWS = 5;
const REFRESH_MS = 30_000;

/** Proof that people play: the instant games running now and the latest large battles. Its size is fixed from the
 * first paint, so nothing below it moves when the numbers arrive (or never do). */
export function LiveStrip() {
  const t = useT();
  const [live, setLive] = useState<Live | null>(null);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    let alive = true;
    const load = () => api.publicLive().then((l) => {
      if (!alive || !Array.isArray(l?.events)) return;
      setLive(l);
      setNow(Date.now());
    }, () => undefined);
    void load();
    const timer = window.setInterval(load, REFRESH_MS);
    return () => {
      alive = false;
      window.clearInterval(timer);
    };
  }, []);

  const dash = '—';
  let numbers: string;
  if (!live) numbers = dash;
  else if (live.games === 0) numbers = t('site.live.nothingRunning');
  else {
    numbers = [t('site.live.games', { count: live.games }), t('site.live.humans', { count: live.humans }),
      t('site.live.bots', { count: live.bots }), t('site.live.openSeats', { count: live.openSeats })].join(' · ');
  }
  let last = dash;
  if (live) last = live.lastBattleAt === null ? t('site.live.noBattle') : t('site.live.lastBattle', { ago: agoText(live.lastBattleAt, now) });
  const events = live?.events.slice(0, ROWS) ?? [];

  return (
    <section className="live-strip card" aria-label={t('site.live.label')}>
      <p className="live-numbers">{numbers}</p>
      <p className="live-last muted">{last}</p>
      <ul className="event-list">
        {events.map((e, i) => <EventRow key={`${e.worldId}-${e.at}-${i}`} e={e} now={now} />)}
        {Array.from({ length: ROWS - events.length }, (_, i) => <li key={`empty-${i}`} />)}
      </ul>
    </section>
  );
}
