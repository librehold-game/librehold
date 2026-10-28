import { useState, type ReactNode, type SubmitEvent } from 'react';
import { useT } from '../../i18n/i18n';
import { errorText, type Account, type HostedCard, type HostForm as Form, type HostLimits } from '../api';
import { hostApi } from './hostApi';
import { DIFFICULTIES, DOMINATION, DURATIONS, LATE_JOIN, STANCES, defaultHostForm } from './hostDefaults';

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {children}
    </div>
  );
}

/** Every setting of a hosted game, opening on the instant game's; the server holds each to its range. */
export function HostForm({ limits, me, onCreated }: { limits: HostLimits; me: Account; onCreated: (card: HostedCard) => void }) {
  const t = useT();
  const [form, setForm] = useState<Form>(() => defaultHostForm(limits, me.username));
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const set = <K extends keyof Form>(key: K, value: Form[K]) => setForm((f) => ({ ...f, [key]: value }));
  const num = (key: keyof Form) => (e: { target: { value: string } }) => set(key, Number(e.target.value) as never);
  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      onCreated(await hostApi.createHosted({ ...form, name: form.name.trim() }));
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  const select = (key: keyof Form, values: (string | number)[], label: (v: string | number) => string) => (
    <select id={`host-${key}`} value={String(form[key])}
      onChange={(e) => set(key, (typeof form[key] === 'number' ? Number(e.target.value) : e.target.value) as never)}>
      {values.map((v) => <option key={v} value={v}>{label(v)}</option>)}
    </select>
  );
  const check = (key: 'fieldBattles' | 'registeredOnly') => (
    <div className="field">
      <label htmlFor={`host-${key}`}>
        <input id={`host-${key}`} type="checkbox" checked={form[key]} onChange={(e) => set(key, e.target.checked)} />
        {' '}{t(`host.field.${key}`)}
      </label>
    </div>
  );
  return (
    <form className="host-form" onSubmit={submit}>
      <p className="muted">{t('host.intro')}</p>
      <div className="grid">
        <Field id="host-name" label={t('host.field.name')}>
          <input id="host-name" value={form.name} maxLength={40} onChange={(e) => set('name', e.target.value)} />
        </Field>
        <Field id="host-durationMin" label={t('host.field.duration')}>
          {select('durationMin', DURATIONS, (n) => t('host.minutes', { n }))}
        </Field>
        <Field id="host-seats" label={t('host.field.seats')}>
          <input id="host-seats" type="number" min={2} max={limits.maxSeats} value={form.seats} onChange={num('seats')} />
        </Field>
        <Field id="host-bots" label={t('host.field.bots')}>
          <input id="host-bots" type="number" min={0} max={limits.maxBots} value={form.bots} onChange={num('bots')} />
        </Field>
        <Field id="host-worldSpeed" label={t('host.field.worldSpeed')}>
          <input id="host-worldSpeed" type="number" min={50} max={200} step={10} value={form.worldSpeed} onChange={num('worldSpeed')} />
        </Field>
        <Field id="host-unitSpeed" label={t('host.field.unitSpeed')}>
          <input id="host-unitSpeed" type="number" min={50} max={200} step={10} value={form.unitSpeed} onChange={num('unitSpeed')} />
        </Field>
        <Field id="host-startVillage" label={t('host.field.startVillage')}>
          {select('startVillage', ['grown', 'fresh'], (v) => t(`host.startVillage.${v}`))}
        </Field>
        <Field id="host-barbarians" label={t('host.field.barbarians')}>
          {select('barbarians', ['few', 'normal', 'many'], (v) => t(`host.barbarians.${v}`))}
        </Field>
        <Field id="host-difficulty" label={t('host.field.difficulty')}>
          {select('difficulty', DIFFICULTIES, (v) => t(`admin.choice.difficulty.${v}`))}
        </Field>
        <Field id="host-botStance" label={t('host.field.botStance')}>
          {select('botStance', STANCES, (v) => t(`admin.choice.botStance.${v}`))}
        </Field>
        <Field id="host-sightRadius" label={t('host.field.sightRadius')}>
          <input id="host-sightRadius" type="number" min={0} max={10} value={form.sightRadius} onChange={num('sightRadius')} />
        </Field>
        <Field id="host-protectionMin" label={t('host.field.protectionMin')}>
          <input id="host-protectionMin" type="number" min={0} max={15} value={form.protectionMin} onChange={num('protectionMin')} />
        </Field>
        <Field id="host-dominationPct" label={t('host.field.domination')}>
          {select('dominationPct', DOMINATION, (n) => (n === 0 ? t('host.dominationOff') : t('host.dominationPct', { n })))}
        </Field>
        <Field id="host-lateJoinMin" label={t('host.field.lateJoin')}>
          {select('lateJoinMin', LATE_JOIN, (n) => (n === 0 ? t('host.lateJoinNone') : t('host.lateJoinUntil', { n })))}
        </Field>
        {check('fieldBattles')}
        {check('registeredOnly')}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      <button type="submit" disabled={busy || form.name.trim() === ''}>{t('host.create')}</button>
    </form>
  );
}
