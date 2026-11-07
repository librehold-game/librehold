import { MAP } from '../../data/world.config';

/**
 * Map window fitted to the available space. On desktop it comes out at the default 15 × 13 tiles,
 * on a phone it shrinks to the screen width — the tile count follows from the size, not the other way round.
 */
export interface MapViewport {
  tw: number;
  th: number;
  width: number;
  height: number;
  cols: number;
  rows: number;
}

/** Coordinate ruler on the left and below the map + 9-slice frame on both sides. */
export const RULER_W = 28;
export const RULER_H = 16;
export const FRAME_W = 48;
export const FRAME_H = 38;

/** The smallest window the corner drag lets the player make. */
export const MIN_W = 220;
export const MIN_H = 200;

export const MAX_W = MAP.viewCols * MAP.tileW;
export const MAX_H = MAP.viewRows * MAP.tileH;

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

export function mapViewport(availW: number, availH: number, zoomIdx: number): MapViewport {
  const width = clamp(availW - FRAME_W, MIN_W, MAX_W);
  const height = clamp(availH - FRAME_H, MIN_H, MAX_H);
  return mapViewportAt(width, height, zoomIdx);
}

/**
 * The window at its own size, whatever the player dragged out.
 */
export function mapViewportAt(width: number, height: number, zoomIdx: number): MapViewport {
  const zoom = MAP.zoomLevels[zoomIdx] ?? 1;
  const tw = Math.round(MAP.tileW * zoom);
  const th = Math.round(MAP.tileH * zoom);
  const w = Math.round(Math.max(MIN_W, width));
  const h = Math.round(Math.max(MIN_H, height));
  return { tw, th, width: w, height: h, cols: Math.ceil(w / tw) + 1, rows: Math.ceil(h / th) + 1 };
}

/** A window size kept inside the room the layout gives it: never under the minimum, never past what fits. */
export function clampWindow(size: { w: number; h: number }, room: { w: number; h: number }): { w: number; h: number } {
  return {
    w: Math.round(clamp(size.w, MIN_W, Math.max(MIN_W, room.w))),
    h: Math.round(clamp(size.h, MIN_H, Math.max(MIN_H, room.h))),
  };
}

/** The map window the player dragged out last time, remembered in the browser. */
const MAP_WINDOW_KEY = 'lh-map-window';

export function readMapWindow(): { w: number; h: number } | null {
  try {
    const raw = localStorage.getItem(MAP_WINDOW_KEY);
    if (!raw) return null;
    const v = JSON.parse(raw) as { w?: unknown; h?: unknown };
    const ok = (n: unknown): n is number => typeof n === 'number' && Number.isFinite(n) && n > 0;
    return ok(v.w) && ok(v.h) ? { w: v.w, h: v.h } : null;
  } catch {
    return null;
  }
}

export function saveMapWindow(size: { w: number; h: number }): void {
  try { localStorage.setItem(MAP_WINDOW_KEY, JSON.stringify(size)); } catch { /* no localStorage */ }
}

/** Spreading fingers zooms in (lower zoom index), pinching zooms out. The threshold is set so it doesn't jitter while held. */
export const PINCH_STEP = 1.3;

export function pinchZoomDelta(startDist: number, dist: number): number {
  if (startDist <= 0 || dist <= 0) return 0;
  const ratio = dist / startDist;
  if (ratio >= PINCH_STEP) return -1;
  if (ratio <= 1 / PINCH_STEP) return 1;
  return 0;
}

export function touchDistance(a: { x: number; y: number }, b: { x: number; y: number }): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

/** The field under the mouse and where its tile sits on the map surface. */
export interface MapHover { x: number; y: number; left: number; top: number; tw: number; th: number }
