import army from '../../assets/pixel/army/army.json';
import wallUrl from '../../assets/pixel/village/walls/stone-gate.png';
import type { BuildingId, UnitId } from '../../engine/types';

const unitFiles = import.meta.glob<string>(
  '../../assets/pixel/army/{spear,sword,axe,archer,scout,lightCav,mountedArcher,heavyCav,ram,catapult,paladin,noble}-south.png',
  { eager: true, import: 'default' },
);
const buildingFiles = import.meta.glob<string>('../../assets/pixel/village/buildings/*-3.png', { eager: true, import: 'default' });

const fileOf = (files: Record<string, string>, name: string) => {
  const hit = Object.entries(files).find(([path]) => path.endsWith(`/${name}`));
  if (!hit) throw new Error(`no sprite ${name}`);
  return hit[1];
};

type ArmyEntry = { frames: number };
const sheets = (army as unknown as { units: Record<string, ArmyEntry> }).units;

/** Each unit's south-facing walk sheet; the pages show its first frame. */
export const UNIT_SPRITES = Object.fromEntries(
  Object.keys(sheets).filter((id) => Object.keys(unitFiles).some((p) => p.endsWith(`/${id}-south.png`)))
    .map((id) => [id, { src: fileOf(unitFiles, `${id}-south.png`), frames: sheets[id]!.frames }]),
) as Record<UnitId, { src: string; frames: number }>;

/** Each building at its third stage; the wall is its stone gate. */
export const BUILDING_SPRITES = Object.fromEntries(
  Object.keys(buildingFiles).map((p) => [/\/(\w+)-3\.png$/.exec(p)![1], buildingFiles[p]]).concat([['wall', wallUrl]]),
) as Record<BuildingId, string>;
