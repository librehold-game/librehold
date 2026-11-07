import { buildingName, resourceName, unitName } from '../../i18n/names';
import { PixelIcon } from '../pixelIcons';
import { useT } from '../../i18n/i18n';
import { COMMAND_ICON, commandEnds, villageLabelOf } from '../villageName';
import { useState } from 'react';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import { villageFarmCapacity, villagePopulationUsed, villageProductionPerHour, villageStorageCapacity } from '../../engine/resources';
import { RESOURCES, UNIT_IDS, type BuildingId, type Command, type Resource, type UnitId } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { Countdown } from '../components/Countdown';
import { ResourceIcon } from '../components/ResourceIcon';
import { ResourceList } from '../components/ResourceList';
import { UnitCountList } from '../components/UnitCountList';
import { UnitIcon } from '../components/UnitIcon';
import { formatCoords, formatNumber } from '../format';
import { useClock } from '../useClock';
import { humanPlayer, humanVillageIds } from '../../engine/players';
import { NumberInput } from '../../shared/NumberInput';
import { isMovementOf } from './MovementsPanel';
import { CommandsTable, commandRows, type CommandRow } from '../components/CommandsTable';

type Tab = 'build' | 'units' | 'recruit' | 'resources' | 'trading' | 'commands' | 'groups';

function useVillageIds(groupId: string): string[] {
  return useGameStore((s) => {
    const g = s.game!;
    if (groupId === 'all') return humanVillageIds(g).join(',');
    return (g.villageGroups.find((x) => x.id === groupId)?.villageIds ?? []).filter((id) => g.villages[id]).join(',');
  }).split(',').filter(Boolean);
}

function BuildTab({ ids }: { ids: string[] }) {
  const [building, setBuilding] = useState<BuildingId>('wood');
  const massBuild = useGameStore((s) => s.massBuild);
  const build = useGameStore((s) => s.build);
  const rows = useGameStore((s) => JSON.stringify(ids.map((id) => {
    const v = s.game!.villages[id]!;
    return { id, name: villageLabelOf(s.game!, v), x: v.x, y: v.y, queue: v.buildQueue.map((j) => `${buildingName(j.building)} ${j.targetLevel}`), plan: v.buildPlan?.length ?? 0, level: v.buildings[building], slots: humanPlayer(s.game!).buildQueueSlots };
  })));
  const list = JSON.parse(rows) as { id: string; name: string; x: number; y: number; queue: string[]; plan: number; level: number; slots: number }[];
  const t = useT();
  return (
    <div>
      <div className="flex items-center gap-2 mb-2">
        <select className="px-1 border border-wood-border rounded bg-white/50" value={building} onChange={(e) => setBuilding(e.target.value as BuildingId)}>{BUILDING_ORDER.map((b) => <option key={b} value={b}>{buildingName(b)}</option>)}</select>
        <button className="btn" onClick={() => massBuild(ids, building)}>{t('overview.addEverywhere', { building: buildingName(building), n: ids.length })}</button>
      </div>
      <table className="table-game table-stack">
        <thead><tr><th>{t('overview.village')}</th><th>{buildingName(building)}</th><th>{t('overview.queue')}</th><th>{t('overview.plan')}</th><th></th></tr></thead>
        <tbody>
          {list.map((r) => (
            <tr key={r.id}>
              <td>{r.name} <span className="text-muted">{formatCoords(r.x, r.y)}</span></td>
              <td className="tabular-nums" data-label={t('overview.level')}>{r.level}</td>
              <td data-label={t('overview.queue')}>{r.queue.length ? r.queue.join(', ') : <span className="text-muted">{t('overview.empty')}</span>} <span className="text-muted">({r.queue.length}/{r.slots})</span></td>
              <td className="tabular-nums" data-label={t('overview.plan')}>{r.plan ? t('overview.steps', { count: r.plan }) : '—'}</td>
              <td className="stack-inline"><button className="btn" onClick={() => build(r.id, building)}>+1</button></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function UnitsTab({ ids }: { ids: string[] }) {
  const rows = useGameStore((s) => JSON.stringify(ids.map((id) => { const v = s.game!.villages[id]!; return { id, name: villageLabelOf(s.game!, v), x: v.x, y: v.y, units: v.units }; })));
  const t = useT();
  const list = JSON.parse(rows) as { id: string; name: string; x: number; y: number; units: Record<UnitId, number> }[];
  const cols = UNIT_IDS.filter((u) => list.some((r) => r.units[u] > 0));
  const sums = Object.fromEntries(cols.map((u) => [u, list.reduce((s, r) => s + r.units[u], 0)])) as Record<UnitId, number>;
  return (
    <div className="overflow-x-auto">
    <table className="table-game">
      <thead><tr><th>{t('overview.village')}</th>{cols.map((u) => <th key={u} className="text-center" title={unitName(u)}><UnitIcon unit={u} /></th>)}</tr></thead>
      <tbody>
        {list.map((r) => <tr key={r.id}><td>{r.name} <span className="text-muted">{formatCoords(r.x, r.y)}</span></td>{cols.map((u) => <td key={u} className="text-center tabular-nums">{r.units[u] || ''}</td>)}</tr>)}
        <tr className="font-semibold"><td>{t('overview.total')}</td>{cols.map((u) => <td key={u} className="text-center tabular-nums">{formatNumber(sums[u])}</td>)}</tr>
      </tbody>
    </table>
    </div>
  );
}

function RecruitTab({ ids }: { ids: string[] }) {
  const [unit, setUnit] = useState<UnitId>('spear');
  const [upTo, setUpTo] = useState(100);
  const massRecruit = useGameStore((s) => s.massRecruit);
  const [msg, setMsg] = useState('');
  const t = useT();
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 flex-wrap">
        <select className="px-1 border border-wood-border rounded bg-white/50" value={unit} onChange={(e) => setUnit(e.target.value as UnitId)}>{UNIT_IDS.filter((u) => u !== 'noble').map((u) => <option key={u} value={u}>{unitName(u)}</option>)}</select>
        <label>{t('overview.recruitUpTo')} <NumberInput className="w-20 px-1 border border-wood-border rounded bg-white/50" min={0} value={upTo} onValue={setUpTo} /> {t('overview.eachVillage')}</label>
        <button className="btn" onClick={async () => setMsg(t('overview.ordered', { n: await massRecruit(ids, unit, upTo) }))}>{t('overview.massRecruit', { n: ids.length })}</button>
        {msg && <span className="text-muted">{msg}</span>}
      </div>
      <p className="text-muted text-[11px]">{t('overview.recruitHint')}</p>
    </div>
  );
}

function ResourcesTab({ ids }: { ids: string[] }) {
  const rows = useGameStore((s) => JSON.stringify(ids.map((id) => {
    const v = s.game!.villages[id]!;
    const cap = villageStorageCapacity(v);
    return { id, name: villageLabelOf(s.game!, v), x: v.x, y: v.y, res: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) }, cap, prod: RESOURCES.map((r) => Math.round(villageProductionPerHour(v, r, s.game!.config))), pop: villagePopulationUsed(v, s.game!), popCap: villageFarmCapacity(v) };
  })));
  const list = JSON.parse(rows) as { id: string; name: string; x: number; y: number; res: Record<'wood' | 'clay' | 'iron', number>; cap: number; prod: number[]; pop: number; popCap: number }[];
  const t = useT();
  return (
    <table className="table-game table-stack">
      <thead><tr><th>{t('overview.village')}</th>{RESOURCES.map((r) => <th key={r}><ResourceIcon r={r} size={14} /></th>)}<th>{t('overview.storage')}</th><th>{t('overview.production')}</th><th>{t('overview.population')}</th></tr></thead>
      <tbody>
        {list.map((r) => {
          const full = RESOURCES.some((res) => r.res[res] >= r.cap * 0.9);
          return (
            <tr key={r.id} className={full ? 'text-bad' : ''}>
              <td>{r.name} <span className="text-muted">{formatCoords(r.x, r.y)}</span>{full && <> <PixelIcon name="warning" size={14} /></>}</td>
              {RESOURCES.map((res) => <td key={res} className="tabular-nums" data-label={resourceName(res)}>{formatNumber(r.res[res])}</td>)}
              <td className="tabular-nums" data-label={t('overview.storage')}>{formatNumber(r.cap)}</td>
              <td className="tabular-nums" data-label={t('overview.production')}>{r.prod.join(' / ')}</td>
              <td className="tabular-nums" data-label={t('overview.population')}>{formatNumber(r.pop)}/{formatNumber(r.popCap)}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

/** Troop movements only. Merchant shipments have their own tab, so they are not listed twice. */
function CommandsTab({ ids }: { ids: string[] }) {
  const t = useT();
  const json = useGameStore((s) => {
    const g = s.game!;
    return JSON.stringify(commandRows(g, g.commands.filter((c) => c.type !== 'trade' && (ids.includes(c.originId) || ids.includes(c.targetId))), () => false));
  });
  const rows = JSON.parse(json) as CommandRow[];
  if (rows.length === 0) return <p className="text-muted italic">{t('overview.noMoves')}</p>;
  return <CommandsTable rows={rows} />;
}

/** Every merchant shipment the villages in view have on the road, in the table the Commands tab uses. */
function TradingTab({ ids }: { ids: string[] }) {
  const t = useT();
  // The panel's own predicate, so a shipment another player is bringing to one of these villages is listed too.
  const json = useGameStore((s) => {
    const g = s.game!;
    const shown = new Set(ids);
    const own = new Set(humanVillageIds(g));
    return JSON.stringify(commandRows(g, g.commands.filter((c) => isMovementOf(c, shown, own, 'trade')), () => false));
  });
  const rows = JSON.parse(json) as CommandRow[];
  if (rows.length === 0) return <p className="text-muted italic">{t('overview.noTrade')}</p>;
  return <CommandsTable rows={rows} />;
}

function GroupsTab() {
  const groups = useGameStore((s) => JSON.stringify(s.game!.villageGroups));
  const villages = useGameStore((s) => JSON.stringify(humanVillageIds(s.game!).map((id) => { const v = s.game!.villages[id]!; return { id, label: `${villageLabelOf(s.game!, v)} ${formatCoords(v.x, v.y)}` }; })));
  const t = useT();
  const save = useGameStore((s) => s.saveGroup);
  const del = useGameStore((s) => s.deleteGroup);
  const list = JSON.parse(groups) as { id: string; name: string; villageIds: string[] }[];
  const all = JSON.parse(villages) as { id: string; label: string }[];
  const [name, setName] = useState('');
  return (
    <div className="space-y-3">
      <form className="flex items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim()) { save({ id: `grp_${Date.now().toString(36)}`, name: name.trim(), villageIds: [] }); setName(''); } }}>
        <input className="px-1 border border-wood-border rounded bg-white/50" placeholder={t('overview.groupName')} value={name} onChange={(e) => setName(e.target.value)} />
        <button className="btn" type="submit">{t('overview.addGroup')}</button>
      </form>
      {list.map((g) => (
        <div key={g.id} className="panel p-2">
          <div className="flex items-center gap-2 mb-1"><strong>{g.name}</strong><span className="text-muted">({g.villageIds.length})</span><button className="btn ml-auto" onClick={() => del(g.id)}>{t('overview.delete')}</button></div>
          <div className="flex flex-wrap gap-2">
            {all.map((v) => (
              <label key={v.id} className="inline-flex items-center gap-1">
                <input type="checkbox" checked={g.villageIds.includes(v.id)} onChange={(e) => save({ ...g, villageIds: e.target.checked ? [...g.villageIds, v.id] : g.villageIds.filter((x) => x !== v.id) })} /> {v.label}
              </label>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

export function OverviewView() {
  const [tab, setTab] = useState<Tab>('build');
  const [groupId, setGroupId] = useState('all');
  const groups = useGameStore((s) => JSON.stringify(s.game!.villageGroups.map((g) => ({ id: g.id, name: g.name }))));
  const ids = useVillageIds(groupId);
  const groupList = JSON.parse(groups) as { id: string; name: string }[];
  const t = useT();
  return (
    <div className="text-[12px]">
      <div className="flex items-center gap-2 flex-wrap mb-3">
        <h2 className="text-lg mr-2">{t('overview.title')}</h2>
        {(['build', 'units', 'recruit', 'resources', 'trading', 'commands', 'groups'] as const).map((id) => (
          <button key={id} className={`btn ${tab === id ? '!bg-panel-dark' : ''}`} onClick={() => setTab(id)}>{t(`overview.tab.${id}`)}</button>
        ))}
        <select className="ml-auto px-1 border border-wood-border rounded bg-white/50" value={groupId} onChange={(e) => setGroupId(e.target.value)}>
          <option value="all">{t('overview.allVillages')}</option>
          {groupList.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
        </select>
      </div>
      {tab === 'build' && <BuildTab ids={ids} />}
      {tab === 'units' && <UnitsTab ids={ids} />}
      {tab === 'recruit' && <RecruitTab ids={ids} />}
      {tab === 'resources' && <ResourcesTab ids={ids} />}
      {tab === 'trading' && <TradingTab ids={ids} />}
      {tab === 'commands' && <CommandsTab ids={ids} />}
      {tab === 'groups' && <GroupsTab />}
    </div>
  );
}
