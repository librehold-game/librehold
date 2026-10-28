import { useCallback, useEffect, useState } from 'react';
import '../i18n/hostCatalog';
import { useLocale, useT } from '../i18n/i18n';
import { worldDisplayName } from '../i18n/names';
import { api, errorText, type Account, type HostLimits, type LobbyView, type Quadrant, type WorldCard } from './api';
import { ClaimForm } from './ClaimForm';
import { HostForm } from './host/HostForm';
import { HostPanel } from './host/HostPanel';
import type { PageProps } from './pages';
import { PlayButton } from './PlayButton';
import { pathFor } from './routes';

const QUADRANTS: Quadrant[] = ['random', 'nw', 'ne', 'sw', 'se'];

function Summary({ w }: { w: WorldCard }) {
  const t = useT();
  return (
    <span className="muted">
      {t('lobby.summary', { humans: w.humans, bots: w.bots, speed: w.settings.worldSpeed })}
      {w.paused && <> · <strong>{t('lobby.paused')}</strong></>}
    </span>
  );
}

function OpenWorld({ w, onJoined }: { w: WorldCard; onJoined: () => void }) {
  const t = useT();
  const [quadrant, setQuadrant] = useState<Quadrant>('random');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const join = async () => {
    setBusy(true);
    setError('');
    try {
      await api.join(w.id, quadrant);
      onJoined();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <li className="world">
      <div><strong>{worldDisplayName(w)}</strong><br /><Summary w={w} /></div>
      <div className="row">
        <label htmlFor={`quadrant-${w.id}`}>{t('lobby.direction')}</label>
        <select id={`quadrant-${w.id}`} value={quadrant} onChange={(e) => setQuadrant(e.target.value as Quadrant)}>
          {QUADRANTS.map((q) => <option key={q} value={q}>{t(`lobby.quadrant.${q}`)}</option>)}
        </select>
        <button onClick={join} disabled={busy}>{t('lobby.join')}</button>
      </div>
      {error && <p className="error" role="alert">{error}</p>}
    </li>
  );
}

function EnterButton({ id }: { id: number }) {
  const t = useT();
  return <button onClick={() => { window.location.href = `/game/?world=${id}`; }}>{t('lobby.enter')}</button>;
}

/** Hosting a game: the form behind a button, or the server-full notice. */
function HostCard({ limits, me, onCreated }: { limits: HostLimits; me: Account; onCreated: Parameters<typeof HostForm>[0]['onCreated'] }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <section className="card">
      <h2>{t('host.open')}</h2>
      {limits.full ? <p className="muted">{t('host.full')}</p>
        : open ? <HostForm limits={limits} me={me} onCreated={onCreated} />
          : <button onClick={() => setOpen(true)}>{t('host.open')}</button>}
    </section>
  );
}

function HostedLists({ lobby }: { lobby: LobbyView }) {
  const t = useT();
  const locale = useLocale();
  const playing = lobby.playing ?? [];
  const past = lobby.pastHosted ?? [];
  return (
    <>
      {playing.length > 0 && (
        <section className="card">
          <h2>{t('host.playing')}</h2>
          <ul className="worlds">
            {playing.map((w) => (
              <li key={w.id} className="world">
                <div><strong>{worldDisplayName(w)}</strong><br /><Summary w={w} /></div>
                <EnterButton id={w.id} />
              </li>
            ))}
          </ul>
        </section>
      )}
      {past.length > 0 && (
        <section className="card">
          <h2>{t('host.past')}</h2>
          <ul className="worlds">
            {past.map((g) => (
              <li key={g.worldId}><a href={pathFor('game', locale, { id: g.code ?? String(g.worldId) })}>{worldDisplayName(g)}</a></li>
            ))}
          </ul>
        </section>
      )}
    </>
  );
}

function InstantCard() {
  const t = useT();
  return (
    <section className="card instant">
      <h2>{t('lobby.instant.title')}</h2>
      <p>{t('lobby.instant.text')}</p>
      <PlayButton signedIn />
    </section>
  );
}

/** A guest sees the instant game and how to keep their progress; a player also sees the long worlds. */
export function LobbyPage({ me, onClaimed }: { me: Account; onClaimed: (a: Account) => void }) {
  const t = useT();
  const [lobby, setLobby] = useState<LobbyView | null>(null);
  const [error, setError] = useState('');
  const load = useCallback(() => {
    api.lobby().then(setLobby, (err) => setError(errorText(err)));
  }, []);
  useEffect(() => {
    if (!me.isGuest) load();
  }, [me.isGuest, load]);

  if (me.isGuest) {
    return (
      <div className="stack">
        <InstantCard />
        <ClaimForm me={me} onClaimed={onClaimed} />
      </div>
    );
  }
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!lobby) return <p className="muted">{t('loading')}</p>;
  const hosted = lobby.hosted ?? null;
  return (
    <div className="stack">
      <InstantCard />
      {hosted && <HostPanel card={hosted} onChange={(c) => (c ? setLobby({ ...lobby, hosted: c }) : load())} />}
      {lobby.host?.canHost && (!hosted || hosted.status === 'ended') && (
        <HostCard limits={lobby.host} me={me} onCreated={(c) => setLobby({ ...lobby, hosted: c })} />
      )}
      <HostedLists lobby={lobby} />
      <section className="card">
        <h2>{t('lobby.mine')}</h2>
        {lobby.mine.length === 0 ? <p className="muted">{t('lobby.mineEmpty')}</p> : (
          <ul className="worlds">
            {lobby.mine.map((w) => (
              <li key={w.id} className="world">
                <div><strong>{worldDisplayName(w)}</strong><br /><Summary w={w} /></div>
                <EnterButton id={w.id} />
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="card">
        <h2>{t('lobby.open')}</h2>
        {lobby.open.length === 0 ? <p className="muted">{t('lobby.openEmpty')}</p> : (
          <ul className="worlds">{lobby.open.map((w) => <OpenWorld key={w.id} w={w} onJoined={load} />)}</ul>
        )}
      </section>
    </div>
  );
}

export default function LobbyRoute({ me, onClaimed }: PageProps) {
  return me ? <LobbyPage me={me} onClaimed={onClaimed} /> : null;
}
