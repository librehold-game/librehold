import { useEffect, useState } from 'react';
import { formatNumber } from '../i18n/format';
import { getLocale, useLocale, useT } from '../i18n/i18n';
import { worldDisplayName } from '../i18n/names';
import { ApiError, api, errorText, type PublicProfile } from './api';
import { Head } from './Head';
import { loadHeadCatalog } from '../i18n/headCatalog';
import { NotFoundPage } from './NotFoundPage';
import type { PageProps } from './pages';
import { Link } from './router';
import { pathFor } from './routes';
import '../i18n/hostCatalog';

loadHeadCatalog();

export function formatDate(ms: number): string {
  return new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(ms);
}

const TALLY = ['games', 'wins', 'conquered', 'killed'] as const;

/** A player's public page: the tally of their instant games and the games themselves. */
export function ProfilePage({ username }: { username: string }) {
  const t = useT();
  const locale = useLocale();
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  useEffect(() => {
    setProfile(null);
    setFailure(null);
    api.publicProfile(username).then(setProfile, setFailure);
  }, [username]);

  if (failure) {
    if (failure instanceof ApiError && failure.status === 404) return <NotFoundPage />;
    return <p className="error" role="alert">{errorText(failure)}</p>;
  }
  if (!profile) return <p className="muted">{t('loading')}</p>;
  const counts = {
    name: profile.username,
    games: t('head.count.games', { count: profile.tally.games }),
    wins: t('head.count.wins', { count: profile.tally.wins }),
    conquests: t('head.count.conquests', { count: profile.tally.conquered }),
  };
  return (
    <div className="stack">
      <Head title={t('head.player.title', counts)} description={t('head.player.description', counts)} noIndex={profile.isGuest} />
      <section className="card">
        <h1>{profile.username}</h1>
        {profile.isGuest && <p className="muted">{t('publicProfile.guest')}</p>}
      </section>
      <section className="card" aria-label={t('publicProfile.tallyLabel')}>
        <h2>{t('publicProfile.tallyLabel')}</h2>
        <dl className="stat-grid">
          {TALLY.map((k) => (
            <div key={k}>
              <dt>{t(`publicProfile.tally.${k}`)}</dt>
              <dd>{formatNumber(profile.tally[k])}</dd>
            </div>
          ))}
        </dl>
      </section>
      <section className="card">
        <h2>{t('publicProfile.games')}</h2>
        {profile.games.length === 0 ? <p className="muted">{t('publicProfile.noGames')}</p> : (
          <ul className="worlds">
            {profile.games.map((g) => (
              <li key={g.worldId} className="world">
                <div>
                  <Link to={pathFor('game', locale, { id: g.code ?? String(g.worldId) })}>{worldDisplayName(g)}</Link>
                  {g.code && <small className="muted"> · {t('host.privateGame')}</small>}<br />
                  <span className="muted">{formatDate(g.endedAt)}</span>
                </div>
                <span>{t('publicProfile.place', { rank: g.rank, players: g.players, points: formatNumber(g.points) })}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

export default function ProfileRoute({ params }: PageProps) {
  return <ProfilePage username={params.name!} />;
}
