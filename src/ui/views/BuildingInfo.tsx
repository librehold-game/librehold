import { BUILDINGS } from '../../data/buildings.data';
import { COMBAT_BALANCE } from '../../data/units.data';
import { farmCapacity, hideCapacity, storageCapacity } from '../../engine/buildings';
import { msUntilAmount, villagePopulationUsed, villageProductionPerHour } from '../../engine/resources';
import { RESOURCES, type BuildingId, type Resource } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatDurationHuman, formatNumber } from '../format';
import { t, useT } from '../../i18n/i18n';

const RESOURCE_OF: Partial<Record<BuildingId, Resource>> = { wood: 'wood', clay: 'clay', iron: 'iron' };

function Table({ rows }: { rows: [string, React.ReactNode, React.ReactNode?][] }) {
  const withNext = rows.some((r) => r.length > 2);
  useT();
  return (
    <table className="table-game w-auto mb-3">
      <thead><tr><th /><th>{t('info.now')}</th>{withNext && <th>{t('info.next')}</th>}</tr></thead>
      <tbody>
        {rows.map(([k, now, next]) => (
          <tr key={k}>
            <td className="font-semibold pr-4">{k}</td>
            <td className="tabular-nums">{now}</td>
            {withNext && <td className="tabular-nums text-muted">{next ?? '—'}</td>}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** Info on the building page (production, capacity, population, hiding place, wall): current and next level. */
export function BuildingInfo({ building }: { building: BuildingId }) {
  const v = useGameStore(selectActiveVillage);
  const game = useGameStore((s) => s.game!);
  useT();
  const level = v.buildings[building];
  const maxed = level >= BUILDINGS[building].maxLevel;
  const nextLevel = maxed ? null : level + 1;
  const at = <T,>(f: (l: number) => T) => (nextLevel === null ? undefined : f(nextLevel));
  const res = RESOURCE_OF[building];

  if (res) {
    const prod = (l: number) => villageProductionPerHour({ ...v, buildings: { ...v.buildings, [res]: l } }, res, game.config);
    return <Table rows={[[t('info.production'), formatNumber(prod(level)), at((l) => formatNumber(prod(l)))]]} />;
  }
  if (building === 'storage') {
    const cap = storageCapacity(level, v.bonus);
    return (
      <>
        <Table rows={[[t('info.capacity'), formatNumber(cap), at((l) => formatNumber(storageCapacity(l, v.bonus)))]]} />
        <table className="table-game w-auto">
          <thead><tr><th>{t('info.resource')}</th><th>{t('info.inStorage')}</th><th>{t('info.fullIn')}</th></tr></thead>
          <tbody>
            {RESOURCES.map((r) => {
              const ms = msUntilAmount(v, r, cap, game.config);
              return (
                <tr key={r}>
                  <td><ResourceIcon r={r} /></td>
                  <td className="tabular-nums">{formatNumber(v.resources[r])}</td>
                  <td className="tabular-nums">{ms === 0 ? t('info.full') : ms === Infinity ? '—' : formatDurationHuman(ms)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </>
    );
  }
  if (building === 'farm') {
    const used = villagePopulationUsed(v, game);
    const cap = farmCapacity(level, v.bonus);
    return (
      <Table rows={[
        [t('info.maxPopulation'), formatNumber(cap), at((l) => formatNumber(farmCapacity(l, v.bonus)))],
        [t('info.used'), formatNumber(used)],
        [t('info.free'), formatNumber(Math.max(0, cap - used))],
      ]} />
    );
  }
  if (building === 'hide') {
    return <Table rows={[[t('info.hidden'), formatNumber(hideCapacity(level)), at((l) => formatNumber(hideCapacity(l)))]]} />;
  }
  if (building === 'wall') {
    const bonus = (l: number) => `${Math.round((Math.pow(COMBAT_BALANCE.wallDefenseFactor, l) - 1) * 100)} %`;
    const base = (l: number) => formatNumber(COMBAT_BALANCE.villageBaseDefense + COMBAT_BALANCE.villageDefensePerWall * l);
    return <Table rows={[[t('info.defenceBonus'), bonus(level), at(bonus)], [t('info.baseDefence'), base(level), at(base)]]} />;
  }
  return null;
}
