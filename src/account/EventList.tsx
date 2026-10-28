import type { PublicEvent } from './api';
import { agoText, eventMessage, worldPath } from './live';
import { Link } from './router';
import { useLocale, useT } from '../i18n/i18n';
import { worldDisplayName } from '../i18n/names';

export function EventRow({ e, now }: { e: PublicEvent; now: number }) {
  const t = useT();
  const locale = useLocale();
  const { key, params } = eventMessage(e);
  const [before, after = ''] = t(key, params).split('{world}');
  return (
    <li>
      {before}
      <Link to={worldPath(e, locale)}>{worldDisplayName({ name: e.worldName, gameNumber: e.gameNumber })}</Link>
      {after} <span className="muted">· {agoText(e.at, now)}</span>
    </li>
  );
}

/** Public battles and conquests, newest first, each linking to its world. */
export function EventList({ events, now }: { events: PublicEvent[]; now: number }) {
  return (
    <ul className="event-list">
      {events.map((e, i) => <EventRow key={`${e.worldId}-${e.at}-${i}`} e={e} now={now} />)}
    </ul>
  );
}
