import { useState } from 'react';
import { MS_PER_DAY, RESOURCES } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { useUiStore, type AnimationMode } from '../../state/uiStore';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatDurationHuman, formatNumber, formatServerTime, worldDayLabel } from '../format';
import { playAlarm, playBuildDone } from '../sound';
import { humanCoins, humanPoints, humanVillageIds } from '../../engine/players';
import { t, useLocale, useT } from '../../i18n/i18n';
import { pathFor } from '../../account/routes';
import { LanguageSwitch } from '../../i18n/LanguageSwitch';
import { RichText } from '../components/RichText';
import { AUTHOR, SOURCE_URL } from '../../account/maker';

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="panel">
      <div className="panel-header">{title}</div>
      <div className="p-3 space-y-2 text-[12px]">{children}</div>
    </section>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="flex items-center gap-3 flex-wrap">
      <span className="w-44 font-semibold">{label}</span>
      {children}
      {hint && <span className="text-muted">{hint}</span>}
    </label>
  );
}

const selectCls = 'px-1 border border-wood-border rounded bg-white/60';

function InterfaceSettings() {
  const prefs = useUiStore((s) => s.prefs);
  const setPrefs = useUiStore((s) => s.setPrefs);
  const setLocale = useGameStore((s) => s.setLocale);
  useT();
  return (
    <Section title={t('settings.interface')}>
      <Field label={t('settings.language')}>
        <LanguageSwitch buttonClassName="btn !px-1.5 !py-0 opacity-70" onChange={setLocale} />
      </Field>
      <Field label={t('settings.animations')}>
        <select className={selectCls} value={prefs.animations} onChange={(e) => setPrefs({ animations: e.target.value as AnimationMode })}>
          <option value="full">{t('settings.animFull')}</option>
          <option value="limited">{t('settings.animLimited')}</option>
          <option value="off">{t('settings.animOff')}</option>
        </select>
        <span className="text-muted">{t('settings.reducedMotion')}</span>
      </Field>
      <Field label={t('settings.scene')}>
        <input type="checkbox" checked={prefs.showScene} onChange={(e) => setPrefs({ showScene: e.target.checked })} />
      </Field>
      <Field label={t('settings.sound')}>
        <input type="checkbox" checked={prefs.sound} onChange={(e) => setPrefs({ sound: e.target.checked })} />
        <button className="btn" onClick={playBuildDone}>{t('settings.testBuild')}</button>
        <button className="btn" onClick={playAlarm}>{t('settings.testAlarm')}</button>
      </Field>
      <p className="text-muted"><RichText text={t('settings.shortcuts')} /></p>
      <p className="text-muted">{t('settings.credits')}</p>
      <p className="text-muted">
        {t('settings.legal', { author: AUTHOR.name })}{' '}
        <a href={SOURCE_URL}>{t('settings.source')}</a>{' · '}<a href={`${SOURCE_URL}/blob/main/LICENSE`}>{t('settings.licence')}</a>
      </p>
    </Section>
  );
}

function PointsChart({ history }: { history: [number, number][] }) {
  if (history.length < 2) return <p className="text-muted italic">{t('settings.chartEmpty')}</p>;
  const W = 640, H = 180, padL = 48, padB = 22, padT = 8;
  const t0 = history[0]![0], t1 = Math.max(history[history.length - 1]![0], t0 + MS_PER_DAY);
  const maxP = Math.max(10, ...history.map((h) => h[1]));
  const x = (t: number) => padL + ((t - t0) / (t1 - t0)) * (W - padL - 8);
  const y = (p: number) => padT + (1 - p / maxP) * (H - padT - padB);
  const pts = history.map((h) => `${x(h[0]).toFixed(1)},${y(h[1]).toFixed(1)}`).join(' ');
  const days = Math.ceil((t1 - t0) / MS_PER_DAY);
  const step = days <= 10 ? 1 : days <= 40 ? 5 : 10;
  const ticks: number[] = [];
  for (let d = 0; d <= days; d += step) ticks.push(d);
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => Math.round(maxP * f));
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full max-w-[640px] border border-wood-border bg-white/40 rounded">
      {yTicks.map((p) => <g key={p}><line x1={padL} x2={W - 8} y1={y(p)} y2={y(p)} stroke="#7d510f" strokeOpacity="0.2" /><text x={padL - 4} y={y(p) + 3} textAnchor="end" fontSize="9" fill="#6b563a">{formatNumber(p)}</text></g>)}
      {ticks.map((d) => <text key={d} x={x(t0 + d * MS_PER_DAY)} y={H - 6} textAnchor="middle" fontSize="9" fill="#6b563a">d{d}</text>)}
      <polyline points={pts} fill="none" stroke="#4c8f2f" strokeWidth="2" />
      <polygon points={`${x(t0)},${y(0)} ${pts} ${x(history[history.length - 1]![0])},${y(0)}`} fill="#4c8f2f" opacity="0.12" />
    </svg>
  );
}

function Stats() {
  const json = useGameStore((s) => {
    const g = s.game!;
    return JSON.stringify({ stats: g.stats, points: humanPoints(g), villages: humanVillageIds(g).length, now: g.now, origin: g.clock.originLocalMs, coins: g.config.noCoins ? null : humanCoins(g), reports: g.reports.length });
  });
  const { stats, points, villages, now, origin, coins } = JSON.parse(json) as { stats: import('../../engine/types').PlayerStats; points: number; villages: number; now: number; origin: number; coins: number | null; reports: number };
  const lootedSum = stats.resourcesLooted.wood + stats.resourcesLooted.clay + stats.resourcesLooted.iron;
  useT();
  const Row = ({ k, v }: { k: string; v: React.ReactNode }) => <tr><td className="text-muted pr-4">{k}</td><td className="tabular-nums font-semibold">{v}</td></tr>;
  return (
    <Section title={t('settings.stats')}>
      <PointsChart history={stats.pointsHistory} />
      <div className="grid md:grid-cols-2 gap-4">
        <table><tbody>
          <Row k={t('settings.points')} v={formatNumber(points)} />
          <Row k={t('settings.villages')} v={villages} />
          <Row k={t('settings.worldTime')} v={`${formatServerTime(origin, now)} · ${worldDayLabel(now)}`} />
          <Row k={t('settings.worldAge')} v={formatDurationHuman(now)} />
          {coins !== null && <Row k={t('settings.coins')} v={t('settings.coinsValue', { coins, minted: stats.coinsMinted })} />}
          <Row k={t('settings.conquered')} v={stats.villagesConquered} />
          <Row k={t('settings.research')} v={stats.researchCompleted} />
        </tbody></table>
        <table><tbody>
          <Row k={t('settings.attacks')} v={formatNumber(stats.attacksSent)} />
          <Row k={t('settings.battlesWon')} v={formatNumber(stats.battlesWon)} />
          <Row k={t('settings.scoutsEtc')} v={`${stats.scoutsSent} / ${stats.supportsSent} / ${stats.tradesSent}`} />
          <Row k={t('settings.killed')} v={formatNumber(stats.unitsKilled)} />
          <Row k={t('settings.lost')} v={formatNumber(stats.unitsLost)} />
          <Row k={t('settings.looted')} v={<span className="inline-flex items-center gap-2">{formatNumber(lootedSum)} {RESOURCES.map((r) => <span key={r} className="inline-flex items-center gap-0.5 font-normal text-muted"><ResourceIcon r={r} size={12} />{formatNumber(stats.resourcesLooted[r])}</span>)}</span>} />
          <Row k={t('settings.today')} v={`${stats.attacksToday.count} / ${formatNumber(stats.lootedToday.total)}`} />
        </tbody></table>
      </div>
    </Section>
  );
}

type Tab = 'ui' | 'stats';

export function SettingsView() {
  const [tab, setTab] = useState<Tab>('ui');
  useT();
  const locale = useLocale();
  const tabs: { id: Tab; label: string }[] = [
    { id: 'ui', label: t('settings.interface') }, { id: 'stats', label: t('settings.stats') },
  ];
  const logout = useGameStore((s) => s.logout);
  const openReport = useUiStore((s) => s.setReportOpen);
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        <h2 className="text-lg mr-2">{t('settings.title')}</h2>
        <a className="btn" href={pathFor('lobby', locale)}>{t('settings.lobby')}</a>
        <button className="btn" onClick={() => void logout()}>{t('settings.logout')}</button>
        <button className="btn" onClick={() => openReport(true)}>{t('report.button')}</button>
        {tabs.map((x) => <button key={x.id} className={`btn ${tab === x.id ? '!bg-gold/40' : ''}`} onClick={() => setTab(x.id)}>{x.label}</button>)}
      </div>
      {tab === 'ui' && <InterfaceSettings />}
      {tab === 'stats' && <Stats />}
    </div>
  );
}
