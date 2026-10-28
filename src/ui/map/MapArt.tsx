import { t } from '../../i18n/i18n';
/* Village sprites and markers of the map tiles; the terrain under them is the pixel layer (`TerrainLayer`). */
import type { CSSProperties } from 'react';
import { HUMAN_ID } from '../../engine/types';
import type { TileInfo } from '../../engine/world';
import { MAP } from '../../data/world.config';
import { PixelVillage, type PixelVillageProps } from './PixelVillage';

export const GRASS = '#79a93f';

export const MAP_GROUND: CSSProperties = { backgroundColor: GRASS };

/** 0 = hut, 1 = fenced settlement, 2 = palisade, 3 = stone wall, 4 = stronghold, 5 = castle. Thresholds in `MAP.villageSizePoints`. */
export type VillageSize = 0 | 1 | 2 | 3 | 4 | 5;

export function villageSize(points: number): VillageSize {
  let s = 0;
  for (const p of MAP.villageSizePoints) if (points >= p) s++;
  return Math.min(5, s) as VillageSize;
}

/** The name of a village tier (0 = hut … 5 = castle). */
export const villageSizeLabel = (size: number): string => t(`villageSize.${size}`);

/** Colour of inactive players' villages (abandoned accounts, eliminated players). */
export const INACTIVE_COLOR = '#9a968c';

/** Village tint on the map: none for barbarians and for you, grey for inactive players, the player's colour for everyone else. */
export function villageTint(t: TileInfo): string | undefined {
  if (t.kind !== 'village' || t.ownerId === null || t.ownerId === HUMAN_ID) return undefined;
  return t.ownerStatus === 'active' ? t.ownerColor : INACTIVE_COLOR;
}
const INK = '#2e1a05';

export interface TileArtProps {
  /** `owner` picks the sprite (player or barbarian); `ownerColor` colours other players' roofs. */
  village?: PixelVillageProps['village'];
  dot?: 'i' | 'a' | 's' | 'r' | 't';
  tw: number;
  th: number;
}

const DOT_COLOR = { i: '#ff2d2d', a: '#a33a2a', s: '#2f5fa8', r: '#4c8f2f', t: '#d4af37' } as const;

export function TileArt({ village, dot, tw, th }: TileArtProps) {
  return (
    <div style={{ position: 'relative', width: tw, height: th }} aria-hidden>
      {village && <PixelVillage village={village} tw={tw} th={th} />}
      {dot && (
        <svg width={tw} height={th} viewBox="0 0 53 38" style={{ position: 'absolute', inset: 0 }}>
          <circle cx="6" cy="6" r="3.2" fill={DOT_COLOR[dot]} stroke={INK} strokeWidth="0.8" />
        </svg>
      )}
    </div>
  );
}
