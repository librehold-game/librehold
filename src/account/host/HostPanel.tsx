import { useState } from 'react';
import '../../i18n/hostCatalog';
import { useLocale, useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { errorText, type HostAction, type HostedCard } from '../api';
import { hostApi } from './hostApi';
import { pathFor } from '../routes';
import { QrCode } from './QrCode';

const MINUTE = 60_000;

/** The host's own world: the invite link, the controls and the players; the god view shows it too (`watching`). */
export function HostPanel({ card, onChange, watching = false }: {
  card: HostedCard; onChange: (c: HostedCard | null) => void; watching?: boolean;
}) {
  const t = useT();
  const locale = useLocale();
  const [copied, setCopied] = useState(false);
  const [qr, setQr] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const link = `${location.origin}${pathFor('join', locale, { code: card.inviteCode })}`;
  const ended = card.status === 'ended';
  const run = async (fn: () => Promise<HostedCard | null>) => {
    setBusy(true);
    setError('');
    try {
      onChange(await fn());
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  const act = (action: HostAction, body?: unknown) => run(() => hostApi.hostAction(card.worldId, action, body));
  const confirmed = (question: string, fn: () => void) => () => {
    if (window.confirm(question)) fn();
  };
  const copy = () => {
    void navigator.clipboard?.writeText(link).then(() => setCopied(true), () => setCopied(false));
  };
  return (
    <section className="card host-panel">
      <h2>{worldDisplayName(card)}</h2>
      <p>
        <strong>{t(`host.status.${card.status}`)}</strong>
        {' · '}{t('host.seatsTaken', { humans: card.humans, seats: card.seats })}
      </p>
      {!ended && (
        <>
          <div className="field">
            <label htmlFor={`invite-${card.worldId}`}>{t('host.inviteLink')}</label>
            <div className="row">
              <input id={`invite-${card.worldId}`} readOnly value={link} onFocus={(e) => e.target.select()} />
              <button className="btn" onClick={copy}>{t('host.copy')}</button>
              <button className="btn" onClick={() => setQr(!qr)}>{t(qr ? 'host.hideQr' : 'host.showQr')}</button>
              <button className="btn" disabled={busy} onClick={confirmed(t('host.replaceConfirm'), () => void run(async () => {
                const { inviteCode } = await hostApi.replaceInvite(card.worldId);
                return { ...card, inviteCode };
              }))}>{t('host.replace')}</button>
            </div>
            {copied && <small className="fresh" role="status">{t('host.copied')}</small>}
          </div>
          {qr && <QrCode text={link} />}
          <label htmlFor={`registered-${card.worldId}`}>
            <input id={`registered-${card.worldId}`} type="checkbox" checked={card.registeredOnly} disabled={busy}
              onChange={(e) => void run(() => hostApi.setRegisteredOnly(card.worldId, e.target.checked))} />
            {' '}{t('host.field.registeredOnly')}
          </label>
          <div className="row">
            {card.status === 'waiting' && <button className="btn" disabled={busy || card.humans === 0} onClick={() => void act('start')}>{t('host.start')}</button>}
            {card.status === 'running' && (
              <button className="btn" disabled={busy || card.pauseLeftMs <= 0} onClick={() => void act('pause')}>
                {t('host.pause', { min: Math.ceil(card.pauseLeftMs / MINUTE) })}
              </button>
            )}
            {card.status === 'paused' && <button className="btn" disabled={busy} onClick={() => void act('resume')}>{t('host.resume')}</button>}
            <button className="btn" disabled={busy} onClick={confirmed(t(card.status === 'waiting' ? 'host.closeConfirm' : 'host.endConfirm'), () => void act('end'))}>
              {t('host.end')}
            </button>
          </div>
        </>
      )}
      <h3>{t('host.players')}</h3>
      {card.players.length === 0 ? <p className="muted">{t('host.nobody')}</p> : (
        <ul className="host-players" aria-label={t('host.players')}>
          {card.players.map((p) => (
            <li key={p.id} className="row">
              <span>{p.name}{p.isGuest && <small className="muted"> · {t('host.guest')}</small>}</span>
              {!ended && (
                <button className="btn" disabled={busy} onClick={confirmed(t('host.kickConfirm', { name: p.name }), () => void act('kick', { playerId: p.id }))}>
                  {t('host.kick', { name: p.name })}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      <div className="row">
        {!watching && <a href={`/game/?world=${card.worldId}&watch=1`}>{t('host.godView')}</a>}
        {ended && card.resultsCode && <a href={pathFor('game', locale, { id: card.resultsCode })}>{t('host.results')}</a>}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
    </section>
  );
}
