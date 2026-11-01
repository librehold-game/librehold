import { useState } from 'react';
import { BUILDING_ORDER } from '../../data/buildings.data';
import { buildingName, unitName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { checkBuild, type BuildBlocker } from '../../engine/queues';
import { checkRecruit, maxRecruitable, unitsForBuilding, type RecruitBlocker } from '../../engine/units';
import type { BuildingId, RecruitBuilding, UnitId } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';

const RECRUIT_BUILDINGS: readonly RecruitBuilding[] = ['barracks', 'stable', 'workshop', 'statue'];

function buildBlockerText(blocker: BuildBlocker, t: ReturnType<typeof useT>): string {
  switch (blocker.kind) {
    case 'maxLevel': return t('buildings.maxLevel');
    case 'queueFull': return t('buildings.queueFull', { slots: blocker.slots });
    case 'requirements': return t('buildings.requires', {
      list: blocker.missing.map((m) => `${buildingName(m.building)} ${m.level}`).join(', '),
    });
    case 'resources': return t('recruit.noResources');
    case 'population': return t('buildings.noPopulation', { needed: blocker.needed, free: blocker.free });
  }
}

function recruitBlockerText(blocker: RecruitBlocker, t: ReturnType<typeof useT>): string {
  switch (blocker.kind) {
    case 'requirements': return t('recruit.requires', {
      list: blocker.missing.map((m) => `${buildingName(m.building)} ${m.level}`).join(', '),
    });
    case 'research': return t('recruit.needsResearch');
    case 'resources': return t('recruit.noResources');
    case 'population': return t('recruit.noPopulation', { needed: blocker.needed, free: blocker.free });
    case 'limit': return t('recruit.limit', { max: blocker.max });
    case 'coins': return t('recruit.coins', { have: blocker.have, needed: blocker.needed });
    case 'count': return t('recruit.count');
  }
}

/** Compact build and recruitment controls for the own-village map popup. */
export function VillageQuickActions({ villageId }: { villageId: string }) {
  const t = useT();
  const game = useGameStore((s) => s.game);
  const build = useGameStore((s) => s.build);
  const recruit = useGameStore((s) => s.recruit);
  const [buildingChoice, setBuildingChoice] = useState<BuildingId>('wood');
  const [unitChoice, setUnitChoice] = useState<UnitId>('spear');
  const [count, setCount] = useState(1);

  const village = game?.villages[villageId];
  if (!game || !village) return null;

  // Match the village building table: unavailable buildings stay hidden until their requirements are met.
  const buildings = BUILDING_ORDER.filter((id) => {
    const blockers = checkBuild(game, village, id).blockers;
    return !blockers.some((b) => b.kind === 'maxLevel' || (village.buildings[id] === 0 && b.kind === 'requirements'));
  });
  const building = buildings.includes(buildingChoice) ? buildingChoice : buildings[0];
  const buildCheck = building ? checkBuild(game, village, building) : null;

  // Only show units for recruitment buildings that have actually been built; checkRecruit handles unlocks and limits.
  const units = RECRUIT_BUILDINGS
    .filter((id) => village.buildings[id] > 0)
    .flatMap((id) => unitsForBuilding(id));
  const unit = units.includes(unitChoice) ? unitChoice : units[0];
  const recruitCheck = unit ? checkRecruit(game, village, unit, count) : null;
  const recruitMax = unit ? maxRecruitable(village, unit, game) : 0;

  return (
    <div className="mt-1.5 space-y-1 border-t border-wood-border/60 pt-1">
      <details>
        <summary className="cursor-pointer font-semibold">{t('map.quickBuild')}</summary>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {building ? (
            <>
              <select
                aria-label={t('map.chooseBuilding')}
                className="min-w-0 max-w-full px-1 border border-wood-border rounded bg-white/50"
                value={building}
                onChange={(e) => setBuildingChoice(e.target.value as BuildingId)}
              >
                {buildings.map((id) => <option key={id} value={id}>{buildingName(id)}</option>)}
              </select>
              <button
                className="btn"
                disabled={!buildCheck || buildCheck.blockers.length > 0}
                title={buildCheck?.blockers.map((b) => buildBlockerText(b, t)).join('\n')}
                onClick={() => { void build(villageId, building); }}
              >
                {t('buildings.upgradeTo', { n: buildCheck?.targetLevel ?? 1 })}
              </button>
              {buildCheck && buildCheck.blockers.length > 0 && (
                <p className="w-full text-[11px] text-bad">{buildCheck.blockers.map((b) => buildBlockerText(b, t)).join(' · ')}</p>
              )}
            </>
          ) : (
            <p className="text-[11px] text-muted">{t('map.noBuildAvailable')}</p>
          )}
        </div>
      </details>
      <details>
        <summary className="cursor-pointer font-semibold">{t('map.quickRecruit')}</summary>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {unit && recruitCheck ? (
            <>
              <select
                aria-label={t('map.chooseUnit')}
                className="min-w-0 max-w-full px-1 border border-wood-border rounded bg-white/50"
                value={unit}
                onChange={(e) => setUnitChoice(e.target.value as UnitId)}
              >
                {units.map((id) => <option key={id} value={id}>{unitName(id)}</option>)}
              </select>
              <input
                aria-label={t('map.count')}
                type="number"
                min={1}
                max={Math.max(1, recruitMax)}
                value={count}
                className="w-14 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
                onChange={(e) => setCount(Math.max(1, Math.floor(Number(e.target.value) || 1)))}
              />
              <button
                className="btn !px-1.5"
                title={t('recruit.max', { n: recruitMax })}
                onClick={() => setCount(Math.max(1, recruitMax))}
              >
                {t('recruit.max', { n: recruitMax })}
              </button>
              <button
                className="btn"
                disabled={recruitCheck.blockers.length > 0}
                title={recruitCheck.blockers.map((b) => recruitBlockerText(b, t)).join('\n')}
                onClick={() => { void recruit(villageId, unit, count); }}
              >
                {t('recruit.recruit')}
              </button>
              {recruitCheck.blockers.length > 0 && (
                <p className="w-full text-[11px] text-bad">{recruitCheck.blockers.map((b) => recruitBlockerText(b, t)).join(' · ')}</p>
              )}
            </>
          ) : (
            <p className="text-[11px] text-muted">{t('map.noRecruitBuildings')}</p>
          )}
        </div>
      </details>
    </div>
  );
}
