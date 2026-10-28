import './scene.css';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { PixelVillageScene } from './pixel/PixelVillageScene';

/** Village illustration — remounted on village change so no timers or frame loops leak across villages. */
export function VillageScene() {
  const villageId = useGameStore((s) => s.game!.activeVillageId);
  const mode = useUiStore((s) => s.prefs.animations);
  const show = useUiStore((s) => s.prefs.showScene);
  if (!show) return null;
  return (
    <div className="overflow-hidden bg-[#4f7a35] leading-none">
      <PixelVillageScene key={villageId} villageId={villageId} mode={mode} />
    </div>
  );
}
