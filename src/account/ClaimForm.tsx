import { useState, type SubmitEvent } from 'react';
import { useT } from '../i18n/i18n';
import { api, errorText, type Account } from './api';
import { TextField } from './LoginPage';

/** Turns a guest into a player: a name and a password on the same account, so games and tally carry on. */
export function ClaimForm({ me, onClaimed }: { me: Account; onClaimed: (a: Account) => void }) {
  const t = useT();
  const [username, setUsername] = useState(me.username);
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      onClaimed(await api.claim(username, password, confirm));
    } catch (err) {
      setError(errorText(err));
      setBusy(false);
    }
  };
  return (
    <form className="card narrow" onSubmit={submit}>
      <h2>{t('claim.title')}</h2>
      <p className="muted">{t('claim.text')}</p>
      <TextField id="claim-name" label={t('register.username')} value={username} onChange={setUsername} autoComplete="username" />
      <TextField id="claim-password" label={t('register.password')} type="password" value={password} onChange={setPassword} autoComplete="new-password" />
      <TextField id="claim-confirm" label={t('register.confirm')} type="password" value={confirm} onChange={setConfirm} autoComplete="new-password" />
      {error && <p className="error" role="alert">{error}</p>}
      <button type="submit" disabled={busy}>{t('claim.submit')}</button>
    </form>
  );
}
