import { useState } from 'react';
import { useT } from '../../i18n/i18n';
import { useGameStore } from '../../state/gameStore';
import { Overlay } from './Overlay';

function Field({ id, label, value, onChange, type = 'text', autoComplete }: {
  id: string; label: string; value: string; onChange: (v: string) => void; type?: string; autoComplete: string;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <label htmlFor={id}>{label}</label>
      <input id={id} type={type} value={value} autoComplete={autoComplete} onChange={(e) => onChange(e.target.value)}
        className="px-1.5 py-1 border border-wood-border rounded bg-white/60" />
    </div>
  );
}

/** Turns the guest into a player mid-game: same account, same village, same game. */
export function ClaimDialog() {
  const open = useGameStore((s) => s.claimOpen);
  const name = useGameStore((s) => s.account?.username ?? '');
  if (!open) return null;
  return <ClaimForm initial={name} />;
}

function ClaimForm({ initial }: { initial: string }) {
  const t = useT();
  const claim = useGameStore((s) => s.claimAccount);
  const close = useGameStore((s) => s.closeClaim);
  const [username, setUsername] = useState(initial);
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  return (
    <Overlay titleId="claim-title">
      <div className="panel-header" id="claim-title">{t('claimDialog.title')}</div>
      <form className="p-3 flex flex-col gap-2" onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError(await claim(username, password, confirm));
        setBusy(false);
      }}>
        <p className="text-muted">{t('claimDialog.text')}</p>
        <Field id="claim-name" label={t('claimDialog.username')} value={username} onChange={setUsername} autoComplete="username" />
        <Field id="claim-password" label={t('claimDialog.password')} type="password" value={password} onChange={setPassword} autoComplete="new-password" />
        <Field id="claim-confirm" label={t('claimDialog.confirm')} type="password" value={confirm} onChange={setConfirm} autoComplete="new-password" />
        {error && <p role="alert" className="text-bad font-semibold">{error}</p>}
        <div className="flex gap-2 justify-end">
          <button type="button" className="btn" onClick={close}>{t('claimDialog.later')}</button>
          <button type="submit" className="btn" disabled={busy}>{t('claimDialog.submit')}</button>
        </div>
      </form>
    </Overlay>
  );
}
