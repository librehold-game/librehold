import { useEffect, useId, useState } from 'react';
import { PixelIcon } from '../pixelIcons';
import { autoFarmUnlocked, type FarmTarget } from '../../engine/farm';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { UnitCountList } from '../components/UnitCountList';
import { FARM_BALANCE } from '../../data/world.config';
import { formatCoords, formatDurationHuman, formatNumber } from '../format';
import { humanVillageIds } from '../../engine/players';
import { useT } from '../../i18n/i18n';
import { RichText } from '../components/RichText';


/** A target's state at a glance: a square pixel in the state's colour, or the hourglass while an attack is on its way. */
const STATUS_COLOR: Record<Exclude<FarmTarget['status'], 'inbound'>, string> = { untouched: '#e8e0cc', full: '#2f7d32', partial: '#d9a51e', losses: '#b3261e' };

function StatusMark({ status }: { status: FarmTarget['status'] }) {
  if (status === 'inbound') return <PixelIcon name="hourglass" size={16} />;
  return <span data-status={status} className="inline-block w-2.5 h-2.5 border border-[#2e1a05]" style={{ background: STATUS_COLOR[status] }} />;
}

type SortKey = 'distance' | 'lastAttackAt' | 'estimatedLoot' | 'points';

export function FarmAssistantView() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const [sort, setSort] = useState<SortKey>('distance');
  const [sel, setSel] = useState(0);
  // Targets come from the server (it knows the map); they are fetched again with every new state.
  const stateRev = useGameStore((s) => s.stateRev);
  const loadTargets = useGameStore((s) => s.loadFarmTargets);
  const [targets, setTargets] = useState<FarmTarget[]>([]);
  useEffect(() => {
    let live = true;
    void loadTargets(villageId).then((t) => { if (live && t) setTargets(t); });
    return () => { live = false; };
  }, [villageId, stateRev, loadTargets]);
  const json = useGameStore((s) => {
    const g = s.game!;
    return JSON.stringify({
      filters: g.farmAssistant.filters, templates: g.farmAssistant.templates,
      today: g.stats.attacksToday, looted: g.stats.lootedToday, now: g.now,
      autoUnlocked: autoFarmUnlocked(g), autoUnlockAt: FARM_BALANCE.autoFarmUnlockVillages, autoEnabled: g.farmAssistant.autoFarmEnabled, autoVillages: g.farmAssistant.autoFarmVillages,
      units: selectActiveVillage(s).units, villages: humanVillageIds(g).length,
    });
  });
  const m = { targets, ...JSON.parse(json) } as {
    targets: FarmTarget[]; filters: { radius: number; minMinutesSinceAttack: number; maxWall: number; hideLosses: boolean; hideInbound: boolean };
    templates: Record<'A' | 'B' | 'C' | 'D', { units: Record<string, number> }>; today: { count: number }; looted: { total: number }; now: number;
    autoUnlocked: boolean; autoUnlockAt: number; autoEnabled: boolean; autoVillages: string[]; units: Record<string, number>; villages: number;
  };
  const sendTemplate = useGameStore((s) => s.sendTemplate);
  const sendC = useGameStore((s) => s.sendTemplateC);
  const setFilters = useGameStore((s) => s.setFarmFilters);
  const setAutoFarm = useGameStore((s) => s.setAutoFarm);
  const setView = useUiStore((s) => s.setView);
  const t = useT();
  const autoHintId = useId();
  const autoHint = m.autoUnlocked ? t('farm.autoOn') : t('farm.autoLocked', { at: m.autoUnlockAt, n: m.villages });

  const sorted = [...m.targets].sort((a, b) => {
    if (sort === 'lastAttackAt') return (a.lastAttackAt ?? -1) - (b.lastAttackAt ?? -1);
    if (sort === 'estimatedLoot') return b.estimatedLoot - a.estimatedLoot;
    if (sort === 'points') return a.points - b.points;
    return a.distance - b.distance;
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLSelectElement) return;
      if (e.key === 'ArrowDown') { setSel((i) => Math.min(sorted.length - 1, i + 1)); e.preventDefault(); }
      if (e.key === 'ArrowUp') { setSel((i) => Math.max(0, i - 1)); e.preventDefault(); }
      const row = sorted[sel];
      if (!row) return;
      if (e.key === 'a' || e.key === 'A') sendTemplate('A', row.x, row.y);
      if (e.key === 'b' || e.key === 'B') sendTemplate('B', row.x, row.y);
      if (e.key === 'c' || e.key === 'C') sendC(row.x, row.y);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sorted, sel, sendTemplate, sendC]);

  const tplEmpty = (k: 'A' | 'B' | 'C') => Object.keys(m.templates[k].units).length === 0;
  const canSend = (k: 'A' | 'B' | 'C') => !tplEmpty(k) && Object.entries(m.templates[k].units).every(([u, n]) => (m.units[u] ?? 0) >= n);

  return (
    <div className="text-[12px]">
      <div className="flex items-center gap-3 flex-wrap mb-2">
        <h2 className="text-lg">{t('farm.title')}</h2>
        <span><RichText text={t('farm.today', { count: m.today.count, looted: formatNumber(m.looted.total) })} /></span>
        <span className="text-muted">{t('farm.templates')} A <UnitCountList units={m.templates.A.units} size={12} /> · B <UnitCountList units={m.templates.B.units} size={12} /> · C <UnitCountList units={m.templates.C.units} size={12} /> <button className="underline" onClick={() => setView('rally')}>{t('farm.edit')}</button></span>
      </div>
      <div className="flex flex-wrap items-center gap-3 mb-2 panel p-2">
        <label>{t('farm.radius')} <input type="number" className="w-14 px-1 border border-wood-border rounded bg-white/50" value={m.filters.radius} onChange={(e) => setFilters({ radius: Math.max(1, Math.min(40, Number(e.target.value) || 1)) })} /></label>
        <label>{t('farm.hideAttacked')} <input type="number" className="w-14 px-1 border border-wood-border rounded bg-white/50" value={m.filters.minMinutesSinceAttack} onChange={(e) => setFilters({ minMinutesSinceAttack: Math.max(0, Number(e.target.value) || 0) })} /> {t('farm.min')}</label>
        <label>{t('farm.wall')} <input type="number" className="w-12 px-1 border border-wood-border rounded bg-white/50" value={m.filters.maxWall} onChange={(e) => setFilters({ maxWall: Math.max(0, Number(e.target.value) || 0) })} /></label>
        <label className="inline-flex items-center gap-1"><input type="checkbox" checked={m.filters.hideLosses} onChange={(e) => setFilters({ hideLosses: e.target.checked })} /> {t('farm.hideLosses')}</label>
        <label className="inline-flex items-center gap-1"><input type="checkbox" checked={m.filters.hideInbound} onChange={(e) => setFilters({ hideInbound: e.target.checked })} /> {t('farm.hideInbound')}</label>
        <label>{t('farm.sort')} <select className="px-1 border border-wood-border rounded bg-white/50" value={sort} onChange={(e) => setSort(e.target.value as SortKey)}>{(['distance', 'lastAttackAt', 'estimatedLoot', 'points'] as const).map((k) => <option key={k} value={k}>{t(`farm.sortBy.${k}`)}</option>)}</select></label>
        <span className="ml-auto">
          {/* A disabled input swallows the hover, so it lets the pointer through to the label and its tooltip. */}
          <label className={`inline-flex items-center gap-1${m.autoUnlocked ? '' : ' cursor-help'}`} title={autoHint}>
            <input type="checkbox" disabled={!m.autoUnlocked} aria-describedby={m.autoUnlocked ? undefined : autoHintId}
              className={m.autoUnlocked ? undefined : 'pointer-events-none'}
              checked={m.autoEnabled && m.autoVillages.includes(villageId)} onChange={(e) => {
                const ids = new Set(m.autoVillages);
                if (e.target.checked) ids.add(villageId); else ids.delete(villageId);
                setAutoFarm(ids.size > 0, [...ids]);
              }} /> {t('farm.auto')} {!m.autoUnlocked && <PixelIcon name="lock" size={16} />}
            {!m.autoUnlocked && <span id={autoHintId} className="sr-only">{autoHint}</span>}
          </label>
        </span>
      </div>
      <p className="text-muted mb-1">{t('farm.keys')}</p>
      {sorted.length === 0 ? (
        <p className="text-muted italic">{t('farm.none', { n: m.filters.radius })}</p>
      ) : (
        <table className="table-game table-stack">
          <thead><tr><th></th><th>{t('farm.col.coords')}</th><th>{t('farm.col.distance')}</th><th>{t('farm.col.pts')}</th><th>{t('farm.col.lastAttack')}</th><th>{t('farm.col.lastLoot')}</th><th>{t('farm.col.estLoot')}</th><th title={t('farm.col.defenceTitle')}>{t('farm.col.defence')}</th><th>A</th><th>B</th><th>C</th></tr></thead>
          <tbody>
            {sorted.map((row, i) => (
              <tr key={row.id} className={i === sel ? 'outline outline-2 outline-gold' : ''} onClick={() => setSel(i)}>
                <td title={t(`farm.status.${row.status}`)} data-label={t('farm.col.status')}><StatusMark status={row.status} /></td>
                <td className="tabular-nums" data-label={t('farm.col.target')}>{formatCoords(row.x, row.y)}</td>
                <td className="tabular-nums" data-label={t('farm.col.distance')}>{row.distance.toFixed(1)}</td>
                <td className="tabular-nums" data-label={t('farm.col.points')}>{formatNumber(row.points)}</td>
                <td className="tabular-nums" data-label={t('farm.col.lastAttack')}>{row.lastAttackAt === null ? t('farm.never') : t('farm.ago', { time: formatDurationHuman(m.now - row.lastAttackAt) })}</td>
                <td className="tabular-nums" data-label={t('farm.col.lastLoot')}>{row.lastHaul === null ? '—' : <span className={row.status === 'full' ? 'text-gold font-semibold' : 'text-muted'}>{formatNumber(row.lastHaul)}{row.lastHaulCapacity ? `/${formatNumber(row.lastHaulCapacity)}` : ''}</span>}</td>
                <td className="tabular-nums" data-label={t('farm.col.estLoot')}>{row.estimatedLoot ? formatNumber(row.estimatedLoot) : '—'}</td>
                <td className="tabular-nums whitespace-nowrap" data-label={t('farm.col.defence')}>
                  {row.defenders === null ? '?' : <span className={row.defenders > 0 ? 'font-semibold' : 'text-muted'}>{formatNumber(row.defenders)}</span>}
                  <span className="text-muted">{t('farm.wallValue', { n: row.wall === null ? '?' : row.wall })}</span>
                </td>
                <td className="stack-inline"><button className="btn !px-2" disabled={!canSend('A')} title={t(tplEmpty('A') ? 'farm.defineTemplate' : 'farm.sendTemplate', { k: 'A' })} onClick={(e) => { e.stopPropagation(); sendTemplate('A', row.x, row.y); }}>A</button></td>
                <td className="stack-inline"><button className="btn !px-2" disabled={!canSend('B')} title={t(tplEmpty('B') ? 'farm.defineTemplate' : 'farm.sendTemplate', { k: 'B' })} onClick={(e) => { e.stopPropagation(); sendTemplate('B', row.x, row.y); }}>B</button></td>
                <td className="stack-inline"><button className="btn !px-2" disabled={tplEmpty('C')} title={tplEmpty('C') ? t('farm.defineC') : t('farm.sendC', { n: formatNumber(row.estimatedLoot) })} onClick={(e) => { e.stopPropagation(); sendC(row.x, row.y); }}>C</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
