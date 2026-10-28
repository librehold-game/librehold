/**
 * Army art written by `npm run assets:pixel-army`. Anything the manifest lacks falls back to the village spearman,
 * so the map works before the art exists. Sprites returned here carry resolved URLs in `file`.
 */
import raw from '../../assets/pixel/army/army.json';
import type { UnitId } from '../../engine/types';
import { LAYOUT, pixelUrl } from '../scene/pixel/layout';
import type { Ground } from './armies';

export type Dir = 'south' | 'east' | 'north' | 'west';
/** `scale` draws art made at another size as tall as the rest of the army. */
export interface Walk { file: Partial<Record<Dir, string>>; w: number; h: number; frames: number; anchor: [number, number]; scale?: number }
export interface ArmyManifest {
  units: Partial<Record<UnitId, Walk>>;
  torch?: Walk;
  shield?: Walk;
  boat?: Walk;
  ground: Partial<Record<Ground, string>>;
}

export const ARMY = raw as unknown as ArmyManifest;

const URLS = import.meta.glob<string>('../../assets/pixel/army/*.png', { eager: true, query: '?url', import: 'default' });

export function armyUrl(file: string): string {
  const url = URLS[`../../assets/pixel/army/${file}`];
  if (!url) throw new Error(`Missing army asset: ${file}`);
  return url;
}

function resolved(w: Walk, url: (f: string) => string): Walk {
  const file: Partial<Record<Dir, string>> = {};
  for (const [d, f] of Object.entries(w.file) as [Dir, string][]) file[d] = url(f);
  return { ...w, file, anchor: [w.anchor[0], w.anchor[1]] };
}

let spear: Walk | null = null;
export function standIn(): Walk {
  const s = LAYOUT.villagers.spear;
  return (spear ??= resolved({ file: s.file, w: s.w, h: s.h, frames: s.frames, anchor: s.anchor }, pixelUrl));
}

const cache = new Map<string, Walk>();
export function armySprite(unit: UnitId | 'torch' | 'shield'): Walk {
  let s = cache.get(unit);
  if (!s) {
    const own = unit === 'torch' || unit === 'shield' ? ARMY[unit] : ARMY.units[unit];
    s = own ? resolved(own, armyUrl) : standIn();
    cache.set(unit, s);
  }
  return s;
}

export function boatSprite(): Walk | null {
  return ARMY.boat ? resolved(ARMY.boat, armyUrl) : null;
}

export function groundImage(g: Ground): string | null {
  const f = ARMY.ground[g];
  return f ? armyUrl(f) : null;
}
