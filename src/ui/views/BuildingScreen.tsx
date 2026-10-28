import { buildingName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import type { BuildingId, RecruitBuilding } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { RecruitScreen } from './RecruitScreen';
import { ResearchScreen } from './ResearchScreen';
import { AcademyScreen } from './AcademyScreen';
import { MarketScreen } from './MarketScreen';
import { BuildingInfo, buildingDescription } from './BuildingInfo';
import { BuildingsTable } from './BuildingsTable';
import { BuildPlanPanel } from './BuildPlanPanel';
import { BuildQueuePanel } from './BuildQueuePanel';
import { useUiStore } from '../../state/uiStore';

const RECRUIT: readonly BuildingId[] = ['barracks', 'stable', 'workshop', 'statue'];

/** Building page: Headquarters handles village upgrades, military buildings recruitment, the rest show their stats. */
export function BuildingScreen({ building, onBack }: { building: BuildingId; onBack: () => void }) {
  const level = useGameStore((s) => selectActiveVillage(s).buildings[building]);
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const openBuilding = useUiStore((s) => s.openBuilding);
  const t = useT();
  let body: React.ReactNode;
  if (level === 0) {
    body = <p className="text-muted italic">{t('buildingScreen.notBuilt')}</p>;
  } else if (RECRUIT.includes(building)) {
    body = <RecruitScreen building={building as RecruitBuilding} />;
  } else if (building === 'smithy') {
    body = <ResearchScreen />;
  } else if (building === 'academy') {
    body = <AcademyScreen />;
  } else if (building === 'market') {
    body = <MarketScreen />;
  } else if (building === 'main') {
    body = (
      <>
        <BuildQueuePanel />
        <BuildingsTable onOpen={(b) => openBuilding(b, villageId)} />
        <BuildPlanPanel />
      </>
    );
  } else {
    body = <BuildingInfo building={building} />;
  }
  return (
    <div>
      <div className="flex items-center gap-3 mb-3">
        <button className="btn" onClick={onBack}>{t('buildingScreen.back')}</button>
        <h2 className="text-lg">{buildingName(building)} <span className="text-muted font-body text-[13px]">{t('buildingScreen.level', { n: level })}</span></h2>
      </div>
      {buildingDescription(building) && <p className="text-[12px] mb-3 max-w-[640px]">{buildingDescription(building)}</p>}
      {body}
    </div>
  );
}
