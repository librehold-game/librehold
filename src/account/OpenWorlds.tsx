import { useEffect, useState } from 'react';
import { api, errorText, type Account, type PublicWorldCard } from './api';
import { playNavigation } from './play';
import { Link } from './router';
import { pathFor } from './routes';
import { useLocale, useT } from '../i18n/i18n';

/** Long worlds that take players now, each with a way in that suits who is reading; nothing when there are none. */
export function OpenWorlds({ me }: { me: Account | null | undefined }) {
  const t = useT();
  const locale = useLocale();
  const [worlds, setWorlds] = useState<PublicWorldCard[]>([]);
  const [error, setError] = useState('');
  useEffect(() => {
    api.publicWorlds().then((l) => setWorlds((l?.long ?? []).filter((w) => w.status === 'open' && !w.paused)), () => undefined);
  }, []);
  if (worlds.length === 0) return null;

  const join = async (id: number) => {
    setError('');
    try {
      await api.join(id, 'random');
      playNavigation.assign(`/game/?world=${id}`);
    } catch (err) {
      setError(errorText(err));
    }
  };
  const date = new Intl.DateTimeFormat(locale, { dateStyle: 'medium' });
  const title = t('site.home.worlds.title');
  return (
    <section className="card open-worlds" aria-label={title}>
      <h2>{title}</h2>
      <ul className="world-cards">
        {worlds.map((w) => {
          const label = t('site.home.worlds.joinLabel', { name: w.name });
          let action;
          if (!me) action = <Link className="secondary" aria-label={label} to={`${pathFor('register', locale)}?world=${w.id}`}>{t('site.home.worlds.join')}</Link>;
          else if (me.isGuest) action = <Link className="secondary" aria-label={label} to={pathFor('lobby', locale)}>{t('site.home.worlds.join')}</Link>;
          else action = <button type="button" className="secondary" aria-label={label} onClick={() => void join(w.id)}>{t('site.home.worlds.join')}</button>;
          return (
            <li key={w.id} className="world-card">
              <h3>{w.name}</h3>
              <p className="muted">
                {t('site.home.worlds.speed', { speed: w.speed })} · {t('site.home.worlds.started', { date: date.format(w.startedAt) })}
              </p>
              <p>{t('site.home.worlds.players', { humans: w.humans, bots: w.bots })}</p>
              {action}
            </li>
          );
        })}
      </ul>
      {error && <p className="error" role="alert">{error}</p>}
    </section>
  );
}
