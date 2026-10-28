import type { ReactNode } from 'react';
import type { BuildingId } from '../../../engine/types';
import raw from '../../../assets/pixel/village/layout.json';
import type { Pt } from './path';

export type Stage = 1 | 2 | 3;
export type WallKind = 'palisade' | 'stone';
type Dir = 'south' | 'east' | 'north' | 'west';

export interface StripInfo { file: string; w: number; h: number; frames: number }
export interface VillagerSprite { file: Partial<Record<Dir, string>>; w: number; h: number; frames: number; anchor: Pt }

/** Shape of `layout.json` written by `scripts/pixel/export.py`; every `file` is relative to `assets/pixel/village/`. */
export interface PixelLayout {
  w: number;
  h: number;
  files: Record<string, [number, number]>;
  ring: { cx: number; cy: number; rx: number; ry: number };
  slots: Record<Exclude<BuildingId, 'wall'>, Pt>;
  rally: Pt;
  plaza: Pt;
  gates: { s: Pt; w: Pt; e: Pt };
  roads: Record<string, { file: string; x: number; y: number; points: Pt[] }>;
  scenery: { file: string; x: number; y: number; z: number; flip?: true }[];
  lights: { x: number; y: number; r: number; at: Exclude<BuildingId, 'wall'> | 'rally' }[];
  buildings: Record<Exclude<BuildingId, 'wall'>, Partial<Record<Stage, string>>>;
  construction: string;
  place: string;
  /** Clickable shape of each building sprite, relative to its top-left corner. */
  hits: Record<string, string>;
  walls: Record<WallKind, {
    x: number; y: number; w: number; h: number; back: string; front: string;
    strips: { x0: number; zBack: number; zFront: number }[];
    /** `y` is where the closed gate stands; `foot` is that row inside a frame, above the open doors' swing. */
    gate: StripInfo & { x: number; y: number; foot: number };
  }>;
  anim: {
    iron: StripInfo & { stage: 2 };
    farm: StripInfo & { stage: 2 };
    wood: StripInfo & { stage: 2; x: number; y: number };
  };
  villagers: Record<'walk' | 'carry_log' | 'carry_sack' | 'spear' | 'chop' | 'hammer', VillagerSprite>;
}

/** One drawable of the scene; layers are drawn in ascending `z`. */
export interface Layer { z: number; key: string; node: ReactNode }

export const LAYOUT = raw as unknown as PixelLayout;

const URLS = import.meta.glob<string>('../../../assets/pixel/village/**/*.png', { eager: true, query: '?url', import: 'default' });
const PREFIX = '../../../assets/pixel/village/';

export function pixelUrl(file: string): string {
  const url = URLS[PREFIX + file];
  if (!url) throw new Error(`Missing pixel art asset: ${file}`);
  return url;
}

/** File of a building stage, or the nearest lower one, then the nearest higher one. */
export function stageFile(id: Exclude<BuildingId, 'wall'>, variant: Stage): string {
  const stages = LAYOUT.buildings[id];
  const order: Stage[] = variant === 1 ? [1, 2, 3] : variant === 2 ? [2, 1, 3] : [3, 2, 1];
  for (const s of order) {
    const file = stages[s];
    if (file) return file;
  }
  throw new Error(`Missing pixel art asset: ${id} stage ${variant}`);
}

const INSIDE: BuildingId[] = ['main', 'barracks', 'stable', 'workshop', 'academy', 'smithy', 'market', 'storage', 'hide'];

/**
 * Road overlays to draw: only roads that lead somewhere that exists or is being built. The side roads to the mine and
 * the farm start where the storage and academy roads end, so those come along to link them to the plaza.
 */
export function roadKeys(levels: Record<BuildingId, number>, construction: BuildingId[]): string[] {
  const keys = new Set(['gate', 'rally']);
  for (const id of INSIDE) if (levels[id] > 0 || construction.includes(id)) keys.add(id);
  if (levels.iron > 0) keys.add('side_w').add('storage');
  if (levels.farm > 0) keys.add('side_e').add('academy');
  if (levels.wood > 0) keys.add('wood');
  if (levels.clay > 0) keys.add('clay');
  return [...keys];
}

export function wallKind(variant: 0 | 1 | 2 | 3): WallKind | null {
  if (variant === 0) return null;
  return variant === 3 ? 'stone' : 'palisade';
}

/** Ground line of the wall ellipse in column `x`; `cy` outside the ellipse. */
export function ringY(x: number, side: 'north' | 'south'): number {
  const { cx, cy, rx, ry } = LAYOUT.ring;
  const k = 1 - ((x - cx) / rx) ** 2;
  if (k <= 0) return cy;
  return side === 'north' ? cy - ry * Math.sqrt(k) : cy + ry * Math.sqrt(k);
}
