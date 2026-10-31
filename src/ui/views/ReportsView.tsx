import { buildingName, unitName } from '../../i18n/names';
import { PixelIcon } from '../pixelIcons';
import { useEffect, useState } from 'react';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import { reportIcon, reportVillageOf } from '../../engine/reports';
import { reportVillageLabel } from '../../i18n/names';
import { t, useT } from '../../i18n/i18n';
import { BotTag } from '../components/BotTag';
import { RESOURCES, UNIT_IDS, type Report, type ReportType, type UnitId } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { ResourceIcon } from '../components/ResourceIcon';
import { UnitIcon } from '../components/UnitIcon';
import { formatNumber } from '../format';
import { useClock } from '../useClock';
import { humanVillageIds, isHumanVillage } from '../../engine/players';
import { canCancelCommand } from '../../engine/commands';

type Filter = 'all' | ReportType | 'unread';

function UnitsTable({ title, side }: { title: string; side: Report['attacker'] }) {
  const units = UNIT_IDS.filter((u) => (side.units[u] ?? 0) > 0);
  const t = useT();
  return (
    <div>
      <h4 className="text-[13px] font-semibold mb-1">{title}</h4>
      {units.length === 0 ? (
        <p className="text-muted text-[12px] italic">{t('reports.noUnits')}</p>
      ) : (
        <div className="overflow-x-auto"><table className="table-game">
          <thead>
            <tr><th></th>{units.map((u) => <th key={u} className="text-center" title={unitName(u)}><UnitIcon unit={u} info /></th>)}</tr>
          </thead>
          <tbody>
            <tr><td>{t('reports.count')}</td>{units.map((u) => <td key={u} className="text-center tabular-nums">{formatNumber(side.units[u] ?? 0)}</td>)}</tr>
            <tr><td>{t('reports.losses')}</td>{units.map((u) => <td key={u} className={`text-center tabular-nums ${(side.losses[u] ?? 0) > 0 ? 'text-bad font-semibold' : ''}`}>{formatNumber(side.losses[u] ?? 0)}</td>)}</tr>
          </tbody>
        </table></div>
      )}
    </div>
  );
}

function LuckBar({ luck }: { luck: number }) {
  const pct = ((luck + 0.25) / 0.5) * 100;
  return (
    <div className="flex items-center gap-2 text-[12px]">
      <span className="text-muted">−25 %</span>
      <div className="relative h-3 w-48 border border-wood-border rounded-[2px] bg-gradient-to-r from-[#a33a2a] via-[#c8a54b] to-[#4c8f2f]">
        <div className="absolute top-[-3px] w-[2px] h-[16px] bg-ink" style={{ left: `calc(${pct}% - 1px)` }} />
      </div>
      <span className="text-muted">+25 %</span>
      <strong className={luck < 0 ? 'text-bad' : 'text-ok'}>{luck >= 0 ? '+' : ''}{(luck * 100).toFixed(1)} %</strong>
    </div>
  );
}

const fieldPlace = (r: Report) => `${r.field!.x.toFixed(1)}|${r.field!.y.toFixed(1)}`;

/** Where a report's troops went: a village, or a place in the field when they found no army there. */
function targetLabel(r: Report): string {
  return r.field && !r.targetId ? t('reports.field.place', { at: fieldPlace(r) }) : reportVillageLabel(r.target);
}

function resultText(r: Report): string {
  if (r.field?.missed) return t('reports.field.missed');
  if (r.type === 'conquest') return t('reports.conquered');
  if (r.type === 'scout' || r.type === 'fieldScout') return r.outcome === 'attackerWon' ? t('reports.scoutOk') : t('reports.scoutFail');
  return r.outcome === 'attackerWon' ? t('reports.attackerWon') : t('reports.defenderWon');
}

function ReportDetail({ id, onBack }: { id: string; onBack: () => void }) {
  const clock = useClock();
  const json = useGameStore((s) => JSON.stringify(s.game!.reports.find((r) => r.id === id) ?? null));
  const r = JSON.parse(json) as Report | null;
  const openRally = useUiStore((s) => s.openRally);
  const sendTemplate = useGameStore((s) => s.sendTemplate);
  const deleteReport = useGameStore((s) => s.deleteReport);
  const villages = useGameStore((s) => s.game!.villages);
  const cancel = useGameStore((s) => s.cancelCmd);
  // The army this field report is about, when it is the player's own, was hit and may still turn back.
  const turnBackId = useGameStore((s) => {
    const g = s.game!;
    const rep = g.reports.find((x) => x.id === id);
    const c = rep?.field ? g.commands.find((x) => x.id === rep.field!.targetCommandId) : undefined;
    return c && c.fieldHit && humanVillageIds(g).includes(c.originId) && canCancelCommand(g, c) ? c.id : null;
  });
  const t = useT();
  if (!r) return <p className="text-muted italic">{t('reports.missing')}</p>;
  const isField = r.type === 'field' || r.type === 'fieldScout';
  const target = villages[r.targetId];
  const tx = r.target.x;
  const ty = r.target.y;
  const canRetry = !isField && tx !== undefined && ty !== undefined && !(target && isHumanVillage(target));
  const haulSum = r.haul ? r.haul.wood + r.haul.clay + r.haul.iron : 0;

  return (
    <div className="space-y-3 text-[12px]">
      <div className="flex items-center gap-2 flex-wrap">
        <button className="btn" onClick={onBack}>{t('reports.back')}</button>
        <h3 className="text-[15px]"><PixelIcon name={reportIcon(r)} size={18} /> {reportVillageLabel(r.origin)}{r.origin.ownerIsBot && <BotTag />} → {targetLabel(r)}{r.target.ownerIsBot && <BotTag />}</h3>
        <span className="text-muted ml-auto">{clock.at(r.time)}</span>
      </div>
      <div className="flex gap-2 flex-wrap">
        {canRetry && <button className="btn" onClick={() => openRally({ x: tx!, y: ty!, units: r.attacker.units, mode: 'attack' })}>{t('reports.retry')}</button>}
        {canRetry && <button className="btn" onClick={() => sendTemplate('A', tx!, ty!)}>{t('reports.sendA')}</button>}
        {turnBackId && <button className="btn" onClick={() => { void cancel(turnBackId); }}>{t('armyCard.turnBack')}</button>}
        <button className="btn ml-auto" onClick={() => { deleteReport(r.id); onBack(); }}>{t('reports.delete')}</button>
      </div>
      <div className="panel p-2">
        <div className="font-semibold mb-1">{t('reports.result')} {resultText(r)}</div>
        {r.field && <div>{t('reports.field.at', { at: fieldPlace(r) })}</div>}
        {r.type !== 'scout' && r.type !== 'fieldScout' && !r.field?.missed && <LuckBar luck={r.luck} />}
        {r.morale !== 1 && <div>{t('reports.morale', { n: Math.round(r.morale * 100) })}</div>}
      </div>
      <div className="grid md:grid-cols-2 gap-3">
        <UnitsTable title={t('reports.attacker')} side={r.attacker} />
        {!r.field?.missed && <UnitsTable title={t('reports.defender')} side={r.defender} />}
      </div>
      {(r.wallBefore !== r.wallAfter || r.buildingDamage) && (
        <div className="panel p-2">
          {r.wallBefore !== r.wallAfter && <div>{t('reports.wall')} {r.wallBefore} → <strong>{r.wallAfter}</strong></div>}
          {r.buildingDamage && <div>{buildingName(r.buildingDamage.building)}: {r.buildingDamage.from} → <strong>{r.buildingDamage.to}</strong></div>}
        </div>
      )}
      {r.haul && (
        <div className="panel p-2 flex items-center gap-3 flex-wrap">
          <span className="font-semibold">{t('reports.loot')}</span>
          {RESOURCES.map((res) => <span key={res} className="inline-flex items-center gap-0.5 tabular-nums"><ResourceIcon r={res} size={14} />{formatNumber(r.haul![res])}</span>)}
          <span className="text-muted">{formatNumber(haulSum)} / {formatNumber(r.haulCapacity ?? 0)} {r.haulCapacity && haulSum >= r.haulCapacity ? t('reports.fullLoad') : t('reports.partialLoot')}</span>
        </div>
      )}
      {r.loyaltyBefore !== undefined && (
        <div className="panel p-2">{t('reports.loyalty')} {r.loyaltyBefore} → <strong>{r.loyaltyAfter}</strong>{r.conquered ? t('reports.villageTaken') : ''}</div>
      )}
      {r.scout && r.type === 'fieldScout' && (
        <div className="panel p-2 flex flex-wrap gap-2">
          <span className="font-semibold">{t('reports.field.scouted')}</span>
          {UNIT_IDS.filter((u) => (r.scout!.units[u] ?? 0) > 0).map((u) => <span key={u} className="inline-flex items-center gap-0.5"><UnitIcon unit={u} size={14} />{formatNumber(r.scout!.units[u] ?? 0)}</span>)}
        </div>
      )}
      {r.scout && r.type !== 'fieldScout' && (
        <div className="panel p-2 space-y-2">
          <div className="flex items-center gap-3 flex-wrap">
            <span className="font-semibold">{t('reports.resources')}</span>
            {RESOURCES.map((res) => <span key={res} className="inline-flex items-center gap-0.5 tabular-nums"><ResourceIcon r={res} size={14} />{formatNumber(r.scout!.resources[res])}</span>)}
            <span className="ml-auto">{t('reports.loyalty')} <strong>{r.scout.loyalty}</strong></span>
          </div>
          <div className="flex flex-wrap gap-x-3 gap-y-0.5">
            {BUILDING_ORDER.filter((b) => (r.scout!.buildings[b] ?? 0) > 0).map((b) => <span key={b}>{buildingName(b)} <strong>{r.scout!.buildings[b]}</strong></span>)}
          </div>
          <div className="flex flex-wrap gap-2">
            <span className="font-semibold">{t('reports.troops')}</span>
            {UNIT_IDS.filter((u) => (r.scout!.units[u] ?? 0) > 0).map((u) => <span key={u} className="inline-flex items-center gap-0.5"><UnitIcon unit={u} size={14} />{formatNumber(r.scout!.units[u] ?? 0)}</span>)}
            {UNIT_IDS.every((u) => !(r.scout!.units[u] ?? 0)) && <span className="text-muted">{t('reports.none')}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

export function ReportsView() {
  const clock = useClock();
  const [filter, setFilter] = useState<Filter>('all');
  const [selected, setSelected] = useState<string | null>(null);
  const villageFilter = useUiStore((s) => s.reportsVillageFilter);
  const setVillageFilter = useUiStore((s) => s.setReportsVillageFilter);
  const villageFilterLabel = useGameStore((s) => (villageFilter ? reportVillageLabel(reportVillageOf(s.game!, villageFilter)) : ''));
  const reportToOpen = useUiStore((s) => s.reportToOpen);
  const consumeReportToOpen = useUiStore((s) => s.consumeReportToOpen);
  const list = useGameStore((s) => JSON.stringify(
    s.game!.reports
      .filter((r) => !villageFilter || r.targetId === villageFilter || r.originId === villageFilter)
      .filter((r) => filter === 'all' || (filter === 'unread' ? !r.read : r.type === filter))
      .map((r) => ({ id: r.id, time: r.time, type: r.type, read: r.read, icon: reportIcon(r), o: reportVillageLabel(r.origin), t: targetLabel(r), haul: r.haul ? r.haul.wood + r.haul.clay + r.haul.iron : null, cap: r.haulCapacity ?? null }))
      .reverse(),
  ));
  const rows = JSON.parse(list) as { id: string; time: number; type: ReportType; read: boolean; icon: string; o: string; t: string; haul: number | null; cap: number | null }[];
  const markRead = useGameStore((s) => s.markReportRead);
  const markAll = useGameStore((s) => s.markAllReportsRead);
  const deleteRead = useGameStore((s) => s.deleteReadReports);
  const total = useGameStore((s) => s.game!.reports.length);
  const t = useT();

  useEffect(() => {
    const id = consumeReportToOpen();
    if (id) { markRead(id); setSelected(id); }
  }, [reportToOpen, consumeReportToOpen, markRead]);
  useEffect(() => () => setVillageFilter(null), [setVillageFilter]);

  if (selected) return <ReportDetail id={selected} onBack={() => setSelected(null)} />;

  return (
    <div>
      <div className="flex items-center gap-2 flex-wrap mb-2 text-[12px]">
        <h2 className="text-lg mr-2">{t('reports.title')} <span className="text-muted font-body text-[12px]">({total}/500)</span></h2>
        <select className="px-1 border border-wood-border rounded bg-white/50" value={filter} onChange={(e) => setFilter(e.target.value as Filter)}>
          {(['all', 'unread', 'attack', 'scout', 'conquest', 'support'] as const).map((f) => <option key={f} value={f}>{t(`reports.filter.${f}`)}</option>)}
        </select>
        {villageFilter && (
          <span className="inline-flex items-center gap-1 px-1.5 py-0.5 border border-wood-border rounded bg-white/40">
            {t('reports.villageFilter')} <strong>{villageFilterLabel}</strong>
            <button className="ml-1 font-bold" title={t('reports.allVillages')} onClick={() => setVillageFilter(null)}>✕</button>
          </span>
        )}
        <button className="btn ml-auto" onClick={markAll}>{t('reports.markAll')}</button>
        <button className="btn" onClick={deleteRead}>{t('reports.deleteRead')}</button>
      </div>
      {rows.length === 0 ? (
        <p className="text-muted italic text-[12px]">{t('reports.empty')}</p>
      ) : (
        <table className="table-game table-stack">
          <thead><tr><th></th><th>{t('reports.colReport')}</th><th>{t('reports.colLoot')}</th><th>{t('reports.colTime')}</th></tr></thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} className={`cursor-pointer ${r.read ? 'text-muted' : 'font-semibold'}`} onClick={() => { markRead(r.id); setSelected(r.id); }}>
                <td className="text-center w-8 stack-inline"><PixelIcon name={r.icon} size={16} /></td>
                <td className="stack-inline">{r.o} → {r.t}</td>
                <td className="tabular-nums" data-label={t('reports.colLoot')}>{r.haul !== null ? `${formatNumber(r.haul)}${r.cap ? ` / ${formatNumber(r.cap)}` : ''}` : '—'}</td>
                <td className="tabular-nums whitespace-nowrap" data-label={t('reports.colTime')}>{clock.at(r.time)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
