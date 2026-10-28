import { buildingName, resourceName } from '../../i18n/names';
import { t, useT } from '../../i18n/i18n';
import { bonusLabel } from '../map/villageInfo';
import { villageLabelOf } from '../villageName';
import { useEffect, useState } from 'react';
import { BUILDINGS } from '../../data/buildings.data';
import { villageProductionPerHour, villageStorageCapacity, villageFarmCapacity, villagePopulationUsed } from '../../engine/resources';
import { loyaltyAt } from '../../engine/conquest';
import { MS_PER_DAY } from '../../engine/types';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatCoords } from '../format';
import { RESOURCES, type BuildingId, type Resource } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { selectConfig } from '../../state/selectors';
import { useUiStore } from '../../state/uiStore';
import { VillageScene } from '../scene/VillageScene';
import { formatDurationHuman, formatNumber } from '../format';
import { BuildQueuePanel } from './BuildQueuePanel';
import { BuildingScreen } from './BuildingScreen';
import { BuildingsTable } from './BuildingsTable';
import { UnitsPanel } from './UnitsPanel';
import { IncomingList } from '../components/IncomingList';
import { BuildPlanPanel } from './BuildPlanPanel';
import { humanPlayer, humanVillageIds } from '../../engine/players';


const RESOURCE_BUILDING: Record<Resource, BuildingId> = { wood: 'wood', clay: 'clay', iron: 'iron' };

/** Panel with a header and a collapse button. */
function SidePanel({ title, children, defaultOpen = true, right }: { title: string; children: React.ReactNode; defaultOpen?: boolean; right?: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen);
  useT();
  return (
    <section className="panel overflow-hidden">
      <div className="panel-header flex items-center justify-between !py-1 !px-2">
        <span className="italic font-bold">{title}</span>
        <span className="flex items-center gap-1">
          {right}
          <button className="btn !px-1.5 !py-0 !text-[11px] leading-4" onClick={() => setOpen((o) => !o)} title={open ? t('village.collapse') : t('village.expand')}>{open ? '−' : '+'}</button>
        </span>
      </div>
      {open && children}
    </section>
  );
}

function ProductionPanel() {
  const json = useGameStore((s) => {
    const v = selectActiveVillage(s);
    const cfg = selectConfig(s);
    const out: Record<Resource, number> = { wood: 0, clay: 0, iron: 0 };
    for (const r of RESOURCES) out[r] = Math.round(villageProductionPerHour(v, r, cfg));
    return JSON.stringify({ out, levels: { wood: v.buildings.wood, clay: v.buildings.clay, iron: v.buildings.iron } });
  });
  const { out, levels } = JSON.parse(json) as { out: Record<Resource, number>; levels: Record<Resource, number> };
  const setFocus = useUiStore((s) => s.setFocusBuilding);
  const lowest = RESOURCES.reduce((a, b) => (levels[b] < levels[a] ? b : a));
  useT();
  return (
    <SidePanel title={t('village.production')}>
      <table className="w-full text-[12px]">
        <tbody className="divide-y divide-wood-border/40">
          {RESOURCES.map((r) => (
            <tr key={r} className="cursor-pointer hover:bg-white/30" onClick={() => setFocus(RESOURCE_BUILDING[r])} title={t('village.levelTitle', { building: buildingName(RESOURCE_BUILDING[r]), n: levels[r] })}>
              <td className="w-6 py-[3px] pl-2"><ResourceIcon r={r} size={15} /></td>
              <td className="py-[3px]">{resourceName(r)}</td>
              <td className="py-[3px] pr-2 text-right tabular-nums"><strong>{formatNumber(out[r])}</strong> <span className="text-muted">{t('village.perHour')}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
      <button className="px-2 pb-1.5 pt-1 font-bold text-ink hover:underline text-[12px]" onClick={() => setFocus(RESOURCE_BUILDING[lowest])}>{t('village.boostProduction')}</button>
    </SidePanel>
  );
}

function ParamsPanel() {
  const json = useGameStore((s) => {
    const g = s.game!;
    const v = selectActiveVillage(s);
    return JSON.stringify({
      worldSpeed: g.config.worldSpeed, unitSpeed: g.config.unitSpeed,
      pop: villagePopulationUsed(v, g), popMax: villageFarmCapacity(v), storage: villageStorageCapacity(v),
      loyalty: Math.round(loyaltyAt(v, g.now, g.config.worldSpeed)), bonus: v.bonus?.kind ?? null,
      protectedMs: Math.max(0, humanPlayer(g).protectedUntil - g.now), villages: humanVillageIds(g).length, coords: [v.x, v.y] as [number, number],
      day: Math.floor(g.now / MS_PER_DAY) + 1, lossMode: g.config.battleLosses,
    });
  });
  useT();
  const p = JSON.parse(json) as { worldSpeed: number; unitSpeed: number; pop: number; popMax: number; storage: number; loyalty: number; bonus: string | null; protectedMs: number; villages: number; coords: [number, number]; day: number; lossMode: string };
  const rows: [string, string][] = [
    [t('village.worldSpeed'), `×${p.worldSpeed}${p.unitSpeed !== 1 ? t('village.unitsSpeed', { n: p.unitSpeed }) : ''}`],
    [t('village.population'), `${formatNumber(p.pop)} / ${formatNumber(p.popMax)}`],
    [t('village.storage'), formatNumber(p.storage)],
  ];
  if (p.loyalty < 100) rows.push([t('village.loyalty'), t('village.loyaltyRising', { n: p.loyalty })]);
  if (p.bonus) rows.push([t('village.bonus'), bonusLabel(p.bonus)]);
  if (p.protectedMs > 0) rows.push([t('village.protection'), t('village.protectionLeft', { time: formatDurationHuman(p.protectedMs) })]);
  if (p.lossMode === 'reduced') rows.push([t('village.battleLosses'), t('village.reduced')]);
  rows.push([t('village.villages'), `${p.villages} · ${formatCoords(p.coords[0], p.coords[1])}`]);
  return (
    <SidePanel title={t('village.params')}>
      <table className="w-full text-[12px]">
        <tbody className="divide-y divide-wood-border/40">
          {rows.map(([k, v]) => <tr key={k}><td className="py-[3px] pl-2 text-muted">{k}</td><td className="py-[3px] pr-2 text-right font-semibold tabular-nums">{v}</td></tr>)}
        </tbody>
      </table>
    </SidePanel>
  );
}

function ScenePanel() {
  const name = useGameStore((s) => villageLabelOf(s.game!, selectActiveVillage(s)));
  useT();
  const points = useGameStore((s) => selectActiveVillage(s).points);
  const show = useUiStore((s) => s.prefs.showScene);
  const setPrefs = useUiStore((s) => s.setPrefs);
  return (
    <section className="panel overflow-hidden mb-3">
      <div className="panel-header flex items-center justify-between !py-1 !px-2">
        <span className="italic font-bold">{name} <span className="font-normal">({t('village.points', { count: points })})</span></span>
        <button className="btn !px-1.5 !py-0 !text-[11px] leading-4" onClick={() => setPrefs({ showScene: !show })} title={show ? t('village.hideScene') : t('village.showScene')}>{show ? '−' : '+'}</button>
      </div>
      <VillageScene />
    </section>
  );
}

export function VillageView() {
  const activeId = useGameStore((s) => selectActiveVillage(s).id);
  const opened = useUiStore((s) => s.buildingScreen);
  const openBuilding = useUiStore((s) => s.openBuilding);
  const closeBuilding = useUiStore((s) => s.closeBuilding);
  const screen = opened?.villageId === activeId ? opened.building : null;
  const focus = useUiStore((s) => s.focusBuilding);
  const setFocus = useUiStore((s) => s.setFocusBuilding);
  useEffect(() => {
    if (!focus) return;
    document.getElementById(`building-row-${focus}`)?.scrollIntoView({ block: 'center', behavior: 'smooth' });
    const t = window.setTimeout(() => setFocus(null), 2500);
    return () => window.clearTimeout(t);
  }, [focus, setFocus]);
  if (screen) return <BuildingScreen building={screen} onBack={closeBuilding} />;
  const open = (b: BuildingId) => openBuilding(b, activeId);
  return (
    <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_270px] gap-3 items-start">
      <div className="min-w-0">
        <ScenePanel />
        <IncomingList villageId={activeId} />
        <BuildQueuePanel />
        <BuildingsTable onOpen={open} focus={focus} />
        <BuildPlanPanel />
      </div>
      <aside className="min-w-0 flex flex-col gap-3 lg:sticky lg:top-2">
        <ProductionPanel />
        <SidePanel title={t('village.units')}><UnitsPanel onRecruit={() => open('barracks')} /></SidePanel>
        <ParamsPanel />
      </aside>
    </div>
  );
}
