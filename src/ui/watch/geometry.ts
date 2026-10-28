import { legEnds } from '../../engine/fieldGeometry';
import type { Command } from '../../engine/types';
import type { MapLayout } from '../../account/timelapse/render';
import type { SpectatorState, WatchVillage } from '../../net/types';

/** How close, in pixels, a click must come to an army marker to pick it. */
const ARMY_HIT_PX = 6;

/** The world's time now: the state's, moved on by the wall time since it came, frozen while the world is paused. */
export function worldNowOf(s: Pick<SpectatorState, 'now' | 'paused'>, receivedAt: number, wallNow: number): number {
  return s.paused ? s.now : s.now + Math.max(0, wallNow - receivedAt);
}

export type ArmyRoute = Pick<Command, 'type' | 'originId' | 'targetId' | 'point' | 'departedAt' | 'arrivesAt'>;

/**
 * Where an army is at now, in map fields: on the straight line from its origin to its target, at the share of the
 * march done. A village end is the middle of its field, a field point stays as it is; null when an end is unknown.
 */
export function armyAt(c: ArmyRoute, now: number): { x: number; y: number } | null {
  const ends = legEnds(c);
  if (!ends) return null;
  const fromPoint = c.originId === '' && !!c.point;
  const toPoint = (c.targetId === '' || c.type === 'intercept') && !!c.point;
  const from = fromPoint ? ends.from : { x: ends.from.x + 0.5, y: ends.from.y + 0.5 };
  const to = toPoint ? ends.to : { x: ends.to.x + 0.5, y: ends.to.y + 0.5 };
  const share = c.arrivesAt <= c.departedAt ? 1 : Math.max(0, Math.min(1, (now - c.departedAt) / (c.arrivesAt - c.departedAt)));
  return { x: from.x + (to.x - from.x) * share, y: from.y + (to.y - from.y) * share };
}

export interface MapPick { kind: 'village' | 'army'; id: string }

/** What a click at (px, py) on the whole map picks: an army marker within reach first, then the village on that field. */
export function hitTest(
  px: number, py: number, layout: MapLayout, mapSize: number, villages: WatchVillage[], armies: { id: string; x: number; y: number }[],
): MapPick | null {
  const cell = layout.size / mapSize;
  let best: { id: string; d: number } | null = null;
  for (const a of armies) {
    const d = Math.hypot(layout.x + a.x * cell - px, layout.y + a.y * cell - py);
    if (d <= ARMY_HIT_PX && (!best || d < best.d)) best = { id: a.id, d };
  }
  if (best) return { kind: 'army', id: best.id };
  const fx = Math.floor((px - layout.x) / cell), fy = Math.floor((py - layout.y) / cell);
  const v = villages.find((w) => w.x === fx && w.y === fy);
  return v ? { kind: 'village', id: v.id } : null;
}
