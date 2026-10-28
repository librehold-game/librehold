import { useState, type SubmitEvent } from 'react';
import { useLocale, useT } from '../i18n/i18n';
import { Hero } from './Hero';
import { ApiError, api, errorText, type Account } from './api';
import { TextField } from './LoginPage';
import { playNavigation } from './play';
import { Link } from './router';
import { pathFor } from './routes';

type Field = 'username' | 'password' | 'passwordConfirm';

export function RegisterPage({ onLogin }: { onLogin: (a: Account) => void }) {
  const t = useT();
  const locale = useLocale();
  const [form, setForm] = useState<Record<Field, string>>({ username: '', password: '', passwordConfirm: '' });
  const [error, setError] = useState<{ field?: string; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (f: Field) => (v: string) => setForm({ ...form, [f]: v });

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const { account } = await api.register(form.username, form.password, form.passwordConfirm);
      // Arriving from a world's Join, the new account goes straight into that world; failing that, to the lobby.
      const world = Number(new URLSearchParams(window.location.search).get('world'));
      if (world > 0) {
        try {
          await api.join(world, 'random');
          playNavigation.assign(`/game/?world=${world}`);
          return;
        } catch {
          // The lobby shows the world, and why it could not be joined.
        }
      }
      onLogin(account);
    } catch (err) {
      setError({ field: err instanceof ApiError ? err.field : undefined, text: errorText(err) });
    } finally {
      setBusy(false);
    }
  };

  const fieldError = (f: Field) => error?.field === f && <p className="error" role="alert">{error.text}</p>;

  return (
    <>
      <Hero />
      <form className="card narrow" onSubmit={submit}>
        <h1>{t('register.title')}</h1>
        <TextField id="reg-name" label={t('register.username')} value={form.username} onChange={set('username')} autoComplete="username" />
        {fieldError('username')}
        <TextField id="reg-password" label={t('register.password')} type="password" value={form.password} onChange={set('password')} autoComplete="new-password" />
        {fieldError('password')}
        <TextField id="reg-confirm" label={t('register.confirm')} type="password" value={form.passwordConfirm} onChange={set('passwordConfirm')} autoComplete="new-password" />
        {fieldError('passwordConfirm')}
        {error && !['username', 'password', 'passwordConfirm'].includes(error.field ?? '') && (
          <p className="error" role="alert">{error.text}</p>
        )}
        <button type="submit" disabled={busy}>{t('register.submit')}</button>
        <p className="muted">{t('register.haveAccount')} <Link to={pathFor('login', locale)}>{t('register.login')}</Link></p>
      </form>
    </>
  );
}

export default RegisterPage;
