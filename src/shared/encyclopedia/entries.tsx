import { BUILDING_BALANCE, BUILDINGS } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import { useT } from '../../i18n/i18n';
import { unitNeedsResearch } from '../../engine/research';
import type { BuildingId, UnitId } from '../../engine/types';
import { BASE_TIMING, requirementChain, type Timing } from './facts';
import { LevelsTable, RequirementChain, ResearchTable, UnitStatsTable, UnlocksList } from './sections';

/** The village the viewer is looking from; absent on the public site. */
export interface VillageProgress {
  levels: Record<BuildingId, number>;
  research: Record<UnitId, number>;
  unitCount?: number;
}

export function BuildingEntry({ id, timing, progress, levelsOpen }: { id: BuildingId; timing: Timing; progress?: VillageProgress; levelsOpen?: boolean }) {
  const t = useT();
  const def = BUILDINGS[id];
  const built = (progress?.levels[id] ?? 0) > 0;
  return (
    <>
      <p>{t(`encyclopedia.building.${id}.text`)}</p>
      <table className="stats">
        <tbody><tr><th scope="row">{t('encyclopedia.label.maxLevel')}</th><td>{def.maxLevel}</td></tr></tbody>
      </table>
      {!built && (
        <>
          <h2>{t(progress ? 'encyclopedia.label.toUnlock' : 'encyclopedia.label.requirements')}</h2>
          <RequirementChain nodes={requirementChain(def.requirements, progress?.levels)} />
        </>
      )}
      <UnlocksList building={id} />
      <details open={levelsOpen}>
        <summary><h2 style={{ display: 'inline' }}>{t('encyclopedia.label.levels')}</h2></summary>
        <LevelsTable id={id} timing={timing} />
        <p className="muted">
          {timing === BASE_TIMING ? t('encyclopedia.label.levelsCaption', { factor: BUILDING_BALANCE.mainTimeFactor }) : t('encyclopedia.label.worldCaption', { speed: timing.worldSpeed })}
        </p>
      </details>
    </>
  );
}

export function UnitEntry({ id, timing, progress }: { id: UnitId; timing: Timing; progress?: VillageProgress }) {
  const t = useT();
  const needsResearch = unitNeedsResearch(id);
  return (
    <>
      <p>{t(`encyclopedia.unit.${id}.text`)}</p>
      <h2>{t(progress ? 'encyclopedia.label.toUnlock' : 'encyclopedia.label.requirements')}</h2>
      <RequirementChain
        nodes={requirementChain(UNITS[id].requirements, progress?.levels)}
        research={{ needed: needsResearch, done: progress ? progress.research[id] >= 1 : undefined }}
      />
      <ResearchTable unit={id} timing={timing} current={progress ? (needsResearch ? progress.research[id] : Math.max(1, progress.research[id])) : undefined} />
      <UnitStatsTable unit={id} timing={timing} count={progress?.unitCount} />
      <p className="muted">
        {timing === BASE_TIMING
          ? t('encyclopedia.label.unitCaption', { building: t(`building.${UNITS[id].building}`) })
          : t('encyclopedia.label.worldCaption', { speed: timing.worldSpeed })}
      </p>
    </>
  );
}
