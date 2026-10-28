import type { PlayerId, PlayerStatus, VillageBonus } from './types';

export function distance(ax: number, ay: number, bx: number, by: number): number {
  return Math.sqrt((ax - bx) ** 2 + (ay - by) ** 2);
}

/** A map field as the client shows it (see ui/map/tiles.ts). */
export type TileInfo =
  | { kind: 'empty'; x: number; y: number; terrain: number }
  | {
      kind: 'village'; x: number; y: number; id: string; ownerId: PlayerId | null; ownerName?: string;
      ownerStatus: PlayerStatus | null; ownerColor?: string; ownerIsBot?: boolean;
      name: string; points: number; bonus?: VillageBonus; materialized: boolean;
    };
