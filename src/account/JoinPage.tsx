import { useEffect, useRef, useState } from 'react';
import { loadHeadCatalog } from '../i18n/headCatalog';
import '../i18n/hostCatalog';
import { useLocale, useT } from '../i18n/i18n';
import { worldDisplayName } from '../i18n/names';
import { ApiError, api, errorText, type Account } from './api';
import { Head } from './Head';
import { hostApi, type InviteInfo } from './host/hostApi';
import type { PageProps } from './pages';
import { playNavigation } from './play';
import { PlayButton } from './PlayButton';
import { Link } from './router';
import { pathFor } from './routes';

loadHeadCatalog();

const REASONS: Record<string, string> = { worldFull: 'join.full', joinClosed: 'join.closed', worldEnded: 'join.ended' };

/** An invite link: straight into the game when it takes players (as a guest if need be), else why not and Play now. */
export function JoinPage({ code, me }: { code: string; me: Account | null | undefined }) {
  const t = useT();
  const locale = useLocale();
  const [info, setInfo] = useState<InviteInfo | 'missing' | null>(null);
  const [error, setError] = useState('');
  const joining = useRef(false);
  useEffect(() => {
    setInfo(null);
    hostApi.inviteInfo(code).then(setInfo, (err) => {
      if (err instanceof ApiError && err.status === 404) setInfo('missing');
      else setError(errorText(err));
    });
  }, [code]);
  const open = info !== null && info !== 'missing' && info.open;
  const signInFirst = open && info.registeredOnly && (!me || me.isGuest);
  useEffect(() => {
    if (!open || signInFirst || me === undefined || joining.current) return;
    joining.current = true;
    void (async () => {
      try {
        if (me === null) await api.guest();
        const r = await hostApi.joinInvite(code);
        playNavigation.assign(`/game/?world=${r.worldId}${r.watch ? '&watch=1' : ''}`);
      } catch (err) {
        setError(errorText(err));
      }
    })();
  }, [open, signInFirst, me, code]);

  if (info === 'missing') {
    return (
      <section className="card">
        <Head title={t('join.nowhere')} description={t('join.nowhere')} noIndex notFound />
        <h1>{t('join.nowhere')}</h1>
        <PlayButton signedIn={!!me} />
      </section>
    );
  }
  if (!info) return error ? <p className="error" role="alert">{error}</p> : <p className="muted">{t('loading')}</p>;
  const params = { name: worldDisplayName(info), host: info.host };
  return (
    <section className="card">
      <Head title={t('head.join.title', params)} description={t('head.join.description', params)} noIndex />
      <h1>{params.name}</h1>
      <p>{t('join.invited', params)} <span className="muted">· {t('host.seatsTaken', { humans: info.humans, seats: info.seats })}</span></p>
      {!info.open ? (
        <>
          <p>{t(REASONS[info.reason ?? ''] ?? 'join.closed')}</p>
          <PlayButton signedIn={!!me} />
        </>
      ) : signInFirst ? (
        <>
          <p>{t('errors.registeredOnly')}</p>
          <p className="row">
            <Link to={pathFor('login', locale)}>{t('login.submit')}</Link>
            <Link to={pathFor('register', locale)}>{t('register.submit')}</Link>
          </p>
        </>
      ) : error ? (
        <>
          <p className="error" role="alert">{error}</p>
          <PlayButton signedIn={!!me} />
        </>
      ) : <p className="muted">{t('join.joining')}</p>}
    </section>
  );
}

export default function JoinRoute({ params, me }: PageProps) {
  return <JoinPage code={params.code ?? ''} me={me} />;
}
