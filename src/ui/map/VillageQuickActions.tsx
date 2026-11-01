import { useState } from 'react';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { meetsRequirements } from '../../engine/buildings';
import { msUntilAmount } from '../../engine/resources';
import { checkBuild, effectiveBuildingLevels } from '../../engine/queues';
import { checkRecruit, maxRecruitable, unitsForBuilding, type RecruitBlocker } from '../../engine/units';
import { RESOURCES, type BuildingId, type RecruitBuilding, type UnitId } from '../../engine/types';
import { buildingName, unitName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { selectConfig } from '../../state/selectors';
import { useGameStore, type GameStore } from '../../state/gameStore';
import { blockerText as buildBlockerText, type BlockerInfo } from '../views/BuildingsTable';
import { blockerText as recruitBlockerText } from '../views/RecruitScreen';

const RECRUIT_BUILDINGS: readonly RecruitBuilding[] = ['barracks', 'stable', 'workshop', 'statue'];

interface BuildModel {
  targetLevel: number;
  blockers: BlockerInfo[];
}

interface RecruitModel {
  blockers: RecruitBlocker[];
  max: number;
}

/** Subscribe to one selected building's check, not the entire game object. */
function makeBuildSelector(villageId: string, building: BuildingId | undefined) {
  return (s: GameStore): string => {
    const game = s.game;
    const village = game?.villages[villageId];
    if (!game || !village || !building) return '';

    const check = checkBuild(game, village, building);
    const config = selectConfig(s);
    const blockers: BlockerInfo[] = check.blockers.map((blocker) => {
      if (blocker.kind !== 'resources') return blocker;
      let eta = 0;
      for (const resource of RESOURCES) {
        if (blocker.missing[resource] > 0) {
          eta = Math.max(eta, msUntilAmount(village, resource, check.cost[resource], config));
        }
      }
      // JSON cannot carry Infinity; match BuildingsTable's "never" representation.
      return { ...blocker, etaMs: eta === Infinity ? -1 : Math.ceil(eta / 60_000) * 60_000 };
    });

    return JSON.stringify({ targetLevel: check.targetLevel, blockers } satisfies BuildModel);
  };
}

/** Subscribe to one selected unit's check, not the entire game object. */
function makeRecruitSelector(villageId: string, unit: UnitId | undefined, count: number) {
  return (s: GameStore): string => {
    const game = s.game;
    const village = game?.villages[villageId];
    if (!game || !village || !unit) return '';
    const check = checkRecruit(game, village, unit, count);
    return JSON.stringify({
      blockers: check.blockers,
      max: maxRecruitable(village, unit, game),
    } satisfies RecruitModel);
  };
}

/** Compact build and recruitment controls for the own-village map popup. */
export function VillageQuickActions({ villageId }: { villageId: string }) {
  const t = useT();
  const build = useGameStore((s) => s.build);
  const recruit = useGameStore((s) => s.recruit);
  const [buildingChoice, setBuildingChoice] = useState<BuildingId>('wood');
  const [unitChoice, setUnitChoice] = useState<UnitId>('spear');
  const [count, setCount] = useState(1);

  // Only the village's building levels and queued target levels affect these option lists.
  const buildingStateJson = useGameStore((s) => {
    const village = s.game?.villages[villageId];
    if (!village) return '';
    return JSON.stringify({
      levels: village.buildings,
      effectiveLevels: effectiveBuildingLevels(village),
    });
  });
  const buildingState = buildingStateJson
    ? JSON.parse(buildingStateJson) as {
      levels: Record<BuildingId, number>;
      effectiveLevels: Record<BuildingId, number>;
    }
    : null;

  // Match the village building table: hide maxed buildings and buildings whose requirements are not met.
  const buildings = buildingState
    ? BUILDING_ORDER.filter((id) =>
      buildingState.effectiveLevels[id] < BUILDINGS[id].maxLevel
      && (buildingState.levels[id] > 0 || meetsRequirements(buildingState.effectiveLevels, id)))
    : [];
  const building = buildings.includes(buildingChoice) ? buildingChoice : buildings[0];
  const buildJson = useGameStore(makeBuildSelector(villageId, building));
  const buildModel = buildJson ? JSON.parse(buildJson) as BuildModel : null;

  // Only show units for recruitment buildings that have actually been built.
  const units = buildingState
    ? RECRUIT_BUILDINGS
      .filter((id) => buildingState.levels[id] > 0)
      .flatMap((id) => unitsForBuilding(id))
    : [];
  const unit = units.includes(unitChoice) ? unitChoice : units[0];
  const recruitJson = useGameStore(makeRecruitSelector(villageId, unit, count));
  const recruitModel = recruitJson ? JSON.parse(recruitJson) as RecruitModel : null;
  const recruitMax = recruitModel?.max ?? 0;

  return (
    <div className="mt-1.5 space-y-1 border-t border-wood-border/60 pt-1">
      <details>
        <summary className="cursor-pointer font-semibold">{t('map.quickBuild')}</summary>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {building && buildModel ? (
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
                disabled={buildModel.blockers.length > 0}
                title={buildModel.blockers.map(buildBlockerText).join('\n')}
                onClick={() => { void build(villageId, building); }}
              >
                {t('buildings.upgradeTo', { n: buildModel.targetLevel })}
              </button>
              {buildModel.blockers.length > 0 && (
                <p className="w-full text-[11px] text-bad">{buildModel.blockers.map(buildBlockerText).join(' · ')}</p>
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
          {unit && recruitModel ? (
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
                disabled={recruitModel.blockers.length > 0}
                title={recruitModel.blockers.map(recruitBlockerText).join('\n')}
                onClick={() => { void recruit(villageId, unit, count); }}
              >
                {t('recruit.recruit')}
              </button>
              {recruitModel.blockers.length > 0 && (
                <p className="w-full text-[11px] text-bad">{recruitModel.blockers.map(recruitBlockerText).join(' · ')}</p>
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
