import { useCallback, useEffect, useState, type SubmitEvent } from 'react';
import '../../i18n/hostCatalog';
import { getLocale, useT } from '../../i18n/i18n';
import { errorText, type AdminAccount, type AdminWorld, type HostSettings } from '../api';
import { adminApi } from './adminApi';
import { hostApi } from '../host/hostApi';
import { DEFAULT_CONFIG, type WorldConfig } from '../worldConfig';
import { ReportsTab } from './ReportsTab';
import { liveChanges, WorldForm } from './WorldForm';

const when = (iso: string | null) => (iso ? new Date(iso).toLocaleString(getLocale()) : '');

/** Loads a list and reloads it after every action; errors of either show above the tab. */
function useList<T>(load: () => Promise<T[]>) {
  const [items, setItems] = useState<T[] | null>(null);
  const [error, setError] = useState('');
  const reload = useCallback(() => {
    load().then((v) => { setItems(v); setError(''); }, (err) => setError(errorText(err)));
  }, [load]);
  useEffect(reload, [reload]);
  const act = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      reload();
    } catch (err) {
      setError(errorText(err));
    }
  };
  return { items, error, act };
}

function PasswordReset({ account, onReset }: { account: AdminAccount; onReset: (pw: string) => void }) {
  const t = useT();
  const [pw, setPw] = useState('');
  return (
    <form className="row" onSubmit={(e) => { e.preventDefault(); onReset(pw); setPw(''); }}>
      <label htmlFor={`pw-${account.id}`} className="sr-only">{t('admin.newPasswordFor', { name: account.username })}</label>
      <input id={`pw-${account.id}`} type="password" placeholder={t('admin.newPassword')} value={pw} autoComplete="new-password"
        onChange={(e) => setPw(e.target.value)} />
      <button type="submit" disabled={pw.length === 0}>{t('admin.setPassword')}</button>
    </form>
  );
}

function AccountsTab() {
  const t = useT();
  const { items, error, act } = useList(adminApi.accounts);
  const [done, setDone] = useState('');
  return (
    <section className="stack">
      {error && <p className="error" role="alert">{error}</p>}
      {done && <p className="fresh">{done}</p>}
      <table>
        <thead><tr><th>{t('admin.col.name')}</th><th>{t('admin.col.role')}</th><th>{t('admin.col.created')}</th><th>{t('admin.col.password')}</th></tr></thead>
        <tbody>
          {(items ?? []).map((a) => (
            <tr key={a.id}>
              <td data-label={t('admin.col.name')}>{a.username}</td>
              <td data-label={t('admin.col.role')}>{t(a.isAdmin ? 'admin.role.admin' : a.isGuest ? 'admin.role.guest' : 'admin.role.player')}</td>
              <td data-label={t('admin.col.created')}>{when(a.createdAt)}</td>
              <td data-label={t('admin.col.password')}>
                <PasswordReset account={a} onReset={(pw) => act(async () => {
                  await adminApi.resetPassword(a.id, pw);
                  setDone(t('admin.passwordDone', { name: a.username }));
                })} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function EditWorld({ world, onSave }: { world: AdminWorld; onSave: (patch: Partial<WorldConfig>) => void }) {
  const t = useT();
  const [cfg, setCfg] = useState(world.settings);
  const patch = liveChanges(world.settings, cfg);
  return (
    <form className="card" onSubmit={(e) => { e.preventDefault(); onSave(patch); }}>
      <h3>{t('admin.settingsOf', { name: world.name })}</h3>
      <WorldForm idPrefix={`edit-${world.id}`} value={cfg} onChange={setCfg} mode="edit" />
      <button type="submit" disabled={Object.keys(patch).length === 0}>{t('admin.save')}</button>
    </form>
  );
}

function WorldsTab() {
  const t = useT();
  const { items, error, act } = useList(adminApi.worlds);
  const [name, setName] = useState('');
  const [cfg, setCfg] = useState<WorldConfig>(DEFAULT_CONFIG);
  const [editing, setEditing] = useState<number | null>(null);
  const create = (e: SubmitEvent) => {
    e.preventDefault();
    void act(async () => {
      await adminApi.createWorld(name, cfg);
      setName('');
    });
  };
  const edited = (items ?? []).find((w) => w.id === editing);
  return (
    <section className="stack">
      {error && <p className="error" role="alert">{error}</p>}
      <table>
        <thead><tr>{['name', 'status', 'players', 'bots', 'state', 'actions'].map((c) => <th key={c}>{t(`admin.col.${c}`)}</th>)}</tr></thead>
        <tbody>
          {(items ?? []).map((w) => (
            <tr key={w.id}>
              <td data-label={t('admin.col.name')}>{w.name}</td>
              <td data-label={t('admin.col.status')}>{t(`admin.status.${w.ended ? 'ended' : w.status}`)}</td>
              <td data-label={t('admin.col.players')}>{w.humans}</td>
              <td data-label={t('admin.col.bots')}>{w.bots}</td>
              <td data-label={t('admin.col.state')}>
                {w.ended
                  ? t('admin.state.ended', { reason: t(`admin.endReason.${w.ended.reason}`), at: new Date(w.ended.at).toLocaleString(getLocale()) })
                  : t(w.paused ? 'admin.state.paused' : 'admin.state.running')}
              </td>
              <td data-label={t('admin.col.actions')} className="row">
                {!w.ended && (
                  <>
                    <button onClick={() => act(() => adminApi.updateWorld(w.id, { status: w.status === 'open' ? 'closed' : 'open' }))}>
                      {t(w.status === 'open' ? 'admin.action.close' : 'admin.action.open')}
                    </button>
                    <button onClick={() => act(() => (w.paused ? adminApi.resumeWorld(w.id) : adminApi.pauseWorld(w.id)))}>
                      {t(w.paused ? 'admin.action.resume' : 'admin.action.pause')}
                    </button>
                  </>
                )}
                <button onClick={() => setEditing(editing === w.id ? null : w.id)}>{t('admin.action.settings')}</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {edited && <EditWorld key={edited.id} world={edited} onSave={(patch) => act(() => adminApi.updateWorld(edited.id, { settings: patch }))} />}
      <form className="card" onSubmit={create}>
        <h3>{t('admin.newWorld')}</h3>
        <div className="field">
          <label htmlFor="world-name">{t('admin.worldName')}</label>
          <input id="world-name" value={name} maxLength={40} onChange={(e) => setName(e.target.value)} />
        </div>
        <WorldForm idPrefix="new" value={cfg} onChange={setCfg} mode="create" />
        <button type="submit" disabled={name.trim() === ''}>{t('admin.create')}</button>
      </form>
    </section>
  );
}

/** The limits of the worlds players host: seats 2–30, bots 0–30; worlds already made keep theirs. */
function HostingTab() {
  const t = useT();
  const [value, setValue] = useState<HostSettings | null>(null);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    hostApi.hostSettings().then(setValue, (err) => setError(errorText(err)));
  }, []);
  const save = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!value) return;
    setSaved(false);
    setError('');
    try {
      setValue(await hostApi.saveHostSettings(value));
      setSaved(true);
    } catch (err) {
      setError(errorText(err));
    }
  };
  const field = (key: keyof HostSettings, min: number) => (
    <div className="field">
      <label htmlFor={`host-limit-${key}`}>{t(key === 'maxSeats' ? 'host.settings.seats' : 'host.settings.bots')}</label>
      <input id={`host-limit-${key}`} type="number" min={min} max={30} value={value?.[key] ?? ''} disabled={!value}
        onChange={(e) => value && setValue({ ...value, [key]: Number(e.target.value) })} />
    </div>
  );
  return (
    <form className="stack" onSubmit={save}>
      <p className="muted">{t('host.settings.text')}</p>
      {error && <p className="error" role="alert">{error}</p>}
      {field('maxSeats', 2)}
      {field('maxBots', 0)}
      <button type="submit" disabled={!value}>{t('host.settings.save')}</button>
      {saved && <p className="fresh" role="status">{t('host.settings.saved')}</p>}
    </form>
  );
}

const TABS = ['accounts', 'worlds', 'hosting', 'reports'] as const;
const tabLabel = (id: (typeof TABS)[number]) => (id === 'hosting' ? 'host.settings.title' : `admin.tab.${id}`);

export function AdminPage() {
  const t = useT();
  const [tab, setTab] = useState<(typeof TABS)[number]>('accounts');
  return (
    <div className="card">
      <h1>{t('admin.title')}</h1>
      <div role="tablist" className="row">
        {TABS.map((id) => (
          <button key={id} role="tab" aria-selected={tab === id} onClick={() => setTab(id)}>{t(tabLabel(id))}</button>
        ))}
      </div>
      {tab === 'accounts' && <AccountsTab />}
      {tab === 'worlds' && <WorldsTab />}
      {tab === 'hosting' && <HostingTab />}
      {tab === 'reports' && <ReportsTab />}
    </div>
  );
}

export default AdminPage;
