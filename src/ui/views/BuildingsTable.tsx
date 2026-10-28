import { buildingName } from '../../i18n/names';
import { t, useT } from '../../i18n/i18n';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { storageCapacity, farmCapacity, hideCapacity } from '../../engine/buildings';
import { canDemolish, checkBuild, effectiveBuildingLevels, type BuildBlocker } from '../../engine/queues';
import { msUntilAmount, villageProductionPerHour } from '../../engine/resources';
import { RESOURCES, type BuildingId, type ResourceSet, type Village, type WorldConfig } from '../../engine/types';
import { selectActiveVillage, useGameStore, type GameStore } from '../../state/gameStore';
import { selectConfig } from '../../state/selectors';
import { CostRow } from '../components/CostRow';
import { formatDuration, formatDurationHuman, formatNumber } from '../format';

/** Effect of the current level; production as in the Production panel — with world speed, bonus and the side's multiplier. */
function effectLabel(id: BuildingId, v: Village, cfg: WorldConfig): string {
  const level = v.buildings[id];
  if (level === 0) return t('buildings.notBuilt');
  switch (id) {
    case 'wood': case 'clay': case 'iron': return t('buildings.perHour', { n: formatNumber(villageProductionPerHour(v, id, cfg)) });
    case 'storage': return t('buildings.capacity', { n: formatNumber(storageCapacity(level)) });
    case 'farm': return t('buildings.population', { n: formatNumber(farmCapacity(level)) });
    case 'hide': return t('buildings.hides', { n: formatNumber(hideCapacity(level)) });
    case 'main': return t('buildings.buildTime', { n: Math.pow(1.05, -(level - 1)).toFixed(2) });
    case 'wall': return t('buildings.defence', { n: Math.pow(1.037, level).toFixed(2) });
    case 'market': return t('buildings.merchants', { n: level });
    case 'academy': return t('buildings.nobles', { n: level });
    default: return '';
  }
}

function blockerText(b: BlockerInfo): string {
  switch (b.kind) {
    case 'maxLevel': return t('buildings.maxLevel');
    case 'queueFull': return t('buildings.queueFull', { slots: b.slots });
    case 'requirements':
      return t('buildings.requires', { list: b.missing.map((m) => `${buildingName(m.building)} ${m.level}`).join(', ') });
    case 'resources': {
      const parts = RESOURCES.filter((r) => b.missing[r] > 0).map((r) => t(`resourceAmount.${r}`, { n: formatNumber(b.missing[r]) }));
      const eta = b.etaMs < 0 ? t('buildings.never') : t('buildings.in', { time: formatDurationHuman(b.etaMs) });
      return t('buildings.missing', { list: parts.join(', '), eta });
    }
    case 'population': return t('buildings.noPopulation', { needed: b.needed, free: b.free });
  }
}

type BlockerInfo = Exclude<BuildBlocker, { kind: 'resources' }> | { kind: 'resources'; missing: ResourceSet; etaMs: number };

interface RowModel {
  level: number;
  effect: string;
  effectiveLevel: number;
  targetLevel: number;
  cost: ResourceSet;
  popDelta: number;
  durationMs: number;
  blockers: BlockerInfo[];
  canDemolish: boolean;
  have: ResourceSet;
}

function makeRowSelector(id: BuildingId) {
  return (s: GameStore): string => {
    const g = s.game;
    if (!g) return '';
    const v = selectActiveVillage(s);
    const check = checkBuild(g, v, id);
    const blockers: BlockerInfo[] = check.blockers.map((b) => {
      if (b.kind !== 'resources') return b;
      let eta = 0;
      for (const r of RESOURCES) {
        if (b.missing[r] > 0) eta = Math.max(eta, msUntilAmount(v, r, check.cost[r], selectConfig(s)));
      }
      // JSON can't carry Infinity — "never" is encoded as -1.
      return { ...b, etaMs: eta === Infinity ? -1 : Math.ceil(eta / 60_000) * 60_000 };
    });
    const model: RowModel = {
      level: v.buildings[id],
      effect: effectLabel(id, v, selectConfig(s)),
      effectiveLevel: effectiveBuildingLevels(v)[id],
      targetLevel: check.targetLevel,
      cost: check.cost,
      popDelta: check.popDelta,
      durationMs: check.durationMs,
      blockers,
      canDemolish: canDemolish(g, v, id),
      have: {
        wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron),
      },
    };
    // Serialised so the selector returns a primitive — re-render only when something actually changes.
    return JSON.stringify(model);
  };
}

function BuildingRow({ id, onOpen, focused }: { id: BuildingId; onOpen: (b: BuildingId) => void; focused: boolean }) {
  const json = useGameStore(makeRowSelector(id));
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const build = useGameStore((s) => s.build);
  const demolish = useGameStore((s) => s.demolish);
  useT();
  if (!json) return null;
  const m = JSON.parse(json) as RowModel;
  const def = BUILDINGS[id];
  // A building whose requirements aren't met is hidden from the table — it appears once unlocked.
  if (m.level === 0 && m.effectiveLevel === 0 && m.blockers.some((b) => b.kind === 'requirements')) return null;
  const maxed = m.blockers.some((b) => b.kind === 'maxLevel');
  const resBlock = m.blockers.find((b): b is Extract<BlockerInfo, { kind: 'resources' }> => b.kind === 'resources');
  const disabled = m.blockers.length > 0;
  const tooltip = m.blockers.map(blockerText).join('\n');
  const queued = m.effectiveLevel !== m.level;

  return (
    <tr id={`building-row-${id}`} className={`${m.level === 0 && !queued ? 'text-muted' : ''} ${focused ? 'outline-2 outline-gold bg-gold/20' : ''}`}>
      <td>
        {m.level > 0 ? (
          <button className="font-semibold underline decoration-dotted cursor-pointer" onClick={() => onOpen(id)}>
            {buildingName(id)}
          </button>
        ) : (
          <div className="font-semibold">{buildingName(id)}</div>
        )}
        <div className="text-[11px] text-muted">{m.effect}</div>
      </td>
      <td className="tabular-nums" data-label={t('buildings.colLevel')}>
        {m.level}
        {queued && <span className="text-muted"> (→{m.effectiveLevel})</span>}
        <span className="text-muted"> / {def.maxLevel}</span>
      </td>
      <td data-label={t('buildings.colCost')}>
        {maxed ? <span className="text-muted">—</span> : <CostRow cost={m.cost} have={m.have} />}
        {!maxed && resBlock && (
          <div className="text-[11px] text-bad whitespace-nowrap" title={blockerText(resBlock)}>
            {resBlock.etaMs < 0 ? t('buildings.storageTooSmall') : t('buildings.resourcesIn', { time: formatDurationHuman(resBlock.etaMs) })}
          </div>
        )}
      </td>
      <td className="tabular-nums" data-label={t('buildings.colPopulation')}>{maxed ? '—' : `+${m.popDelta}`}</td>
      <td className="tabular-nums" data-label={t('buildings.colTime')}>{maxed ? '—' : formatDuration(m.durationMs)}</td>
      <td className="whitespace-nowrap stack-inline">
        <button
          className="btn"
          disabled={disabled}
          title={tooltip || t('buildings.upgradeTitle', { building: buildingName(id), n: m.targetLevel })}
          onClick={() => build(villageId, id)}
        >
          {maxed ? t('buildings.max') : t('buildings.upgradeTo', { n: m.targetLevel })}
        </button>
        {m.canDemolish && (
          <button
            className="btn ml-1 !px-1.5"
            title={t('buildings.demolishTitle', { n: m.effectiveLevel })}
            onClick={() => demolish(villageId, id)}
          >
            ▾
          </button>
        )}
      </td>
    </tr>
  );
}

/** Buildings table with upgrades — in the village overview and on the Headquarters page. A building's name opens its page. */
export function BuildingsTable({ onOpen, focus }: { onOpen: (b: BuildingId) => void; focus?: string | null }) {
  const t = useT();
  return (
    <>
      <h2 className="text-lg mb-2 mt-3">{t('buildings.title')}</h2>
      <div className="overflow-x-auto">
        <table className="table-game table-stack">
          <thead>
            <tr>
              <th>{t('buildings.colBuilding')}</th>
              <th>{t('buildings.colLevel')}</th>
              <th>{t('buildings.colCost')}</th>
              <th>{t('buildings.colPop')}</th>
              <th>{t('buildings.colTime')}</th>
              <th>{t('buildings.colAction')}</th>
            </tr>
          </thead>
          <tbody>
            {BUILDING_ORDER.map((id) => <BuildingRow key={id} id={id} onOpen={onOpen} focused={focus === id} />)}
          </tbody>
        </table>
      </div>
    </>
  );
}
