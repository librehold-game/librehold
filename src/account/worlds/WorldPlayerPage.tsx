import { useEffect, useMemo, useState } from 'react';
import { ApiError, api, errorText, type PublicWorld, type PublicWorldPlayer } from '../api';
import { Head } from '../Head';
import { NotFoundPage } from '../NotFoundPage';
import type { PageProps } from '../pages';
import { Link, navigate } from '../router';
import { pathFor } from '../routes';
import { formatCoords } from '../../i18n/format';
import { loadHeadCatalog } from '../../i18n/headCatalog';
import { useLocale, useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { ProfileBody, ProfileHeading, profileViewData, type ProfileHost } from '../../shared/ranking/ProfileBody';
import './catalog';

loadHeadCatalog();

/** One player in one world: rank, points, villages and conquests there, and a way on to their global profile. */
export function WorldPlayerPage({ id, pid }: { id: number; pid: string }) {
  const t = useT();
  const locale = useLocale();
  const [data, setData] = useState<{ world: PublicWorld; player: PublicWorldPlayer } | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    Promise.all([api.publicWorld(id), api.publicWorldPlayer(id, pid)]).then(([world, player]) => setData({ world, player }), (err) => {
      if (!(err instanceof ApiError && err.status === 404)) {
        setError(errorText(err));
        return;
      }
      api.publicGame(String(id)).then(() => navigate(pathFor('game', locale, { id: String(id) }), true), () => setMissing(true));
    });
  }, [id, pid, locale]);
  const host = useMemo<ProfileHost>(() => {
    const when = new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' });
    return { renderCoords: (x, y) => formatCoords(x, y), formatTime: (ms) => when.format(ms) };
  }, [locale]);

  if (missing) return <NotFoundPage />;
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!data) return <p className="muted">{t('loading')}</p>;
  const { world, player } = data;
  const view = profileViewData(player, player.rank, player.conquests);
  const worldName = worldDisplayName(world);
  return (
    <div className="world-page">
      <Head
        title={t('head.worldPlayer.title', { name: view.name, world: worldName, points: player.player.points })}
        description={t('head.worldPlayer.description', { name: view.name, world: worldName })}
        noIndex
      />
      <nav className="card" aria-label={t('site.worlds.backToWorld')}>
        <Link to={pathFor('world', locale, { id: String(id) })}>← {worldName}</Link>
      </nav>
      <section className="card">
        <h1 className="profile-title"><ProfileHeading data={view} /></h1>
        {player.username && (
          <p><Link to={pathFor('player', locale, { name: player.username })}>{t('site.worlds.globalProfile')}</Link></p>
        )}
        <ProfileBody data={view} host={host} />
      </section>
    </div>
  );
}

export default function WorldPlayerRoute({ params }: PageProps) {
  const id = Number(params.id);
  return /^\d+$/.test(params.id ?? '') && id > 0 && params.pid ? <WorldPlayerPage id={id} pid={params.pid} /> : <NotFoundPage />;
}
