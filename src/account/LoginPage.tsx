import { useState, type SubmitEvent } from 'react';
import { useLocale, useT } from '../i18n/i18n';
import { api, errorText, type Account } from './api';
import { Link } from './router';
import { pathFor } from './routes';

export function TextField({ id, label, value, onChange, type = 'text', autoComplete }: {
  id: string; label: string; value: string; onChange: (v: string) => void; type?: string; autoComplete?: string;
}) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input id={id} type={type} value={value} autoComplete={autoComplete} onChange={(e) => onChange(e.target.value)} />
    </div>
  );
}

export function LoginForm({ onLogin }: { onLogin: (a: Account) => void }) {
  const t = useT();
  const locale = useLocale();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const { account } = await api.login(username, password);
      onLogin(account);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="card narrow" onSubmit={submit}>
      <h1>{t('login.title')}</h1>
      <TextField id="login-name" label={t('login.username')} value={username} onChange={setUsername} autoComplete="username" />
      <TextField id="login-password" label={t('login.password')} type="password" value={password} onChange={setPassword} autoComplete="current-password" />
      {error && <p className="error" role="alert">{error}</p>}
      <button type="submit" disabled={busy}>{t('login.submit')}</button>
      <p className="muted">{t('login.noAccount')} <Link to={pathFor('register', locale)}>{t('login.register')}</Link></p>
    </form>
  );
}

export default LoginForm;
