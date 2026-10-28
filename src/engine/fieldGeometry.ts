/**
 * Port of server/internal/game/field_geometry.go: where armies are between villages and where an interceptor meets
 * one. Villages stand at their integer coordinates, as `distance` measures them.
 */
import { travelTimeMs } from './commands';
import { coordsOfVillageId, MS_PER_MINUTE, type Command, type FieldPoint } from './types';

export function legEnds(c: Pick<Command, 'type' | 'originId' | 'targetId' | 'point'>): { from: FieldPoint; to: FieldPoint } | null {
  const from = c.originId === '' && c.point ? c.point : coordsOfVillageId(c.originId);
  const to = (c.targetId === '' || c.type === 'intercept') && c.point ? c.point : coordsOfVillageId(c.targetId);
  return from && to ? { from: { x: from.x, y: from.y }, to: { x: to.x, y: to.y } } : null;
}

function progress(departedAt: number, arrivesAt: number, t: number): number {
  if (arrivesAt <= departedAt) return 1;
  return Math.max(0, Math.min(1, (t - departedAt) / (arrivesAt - departedAt)));
}

const lerp = (a: FieldPoint, b: FieldPoint, p: number): FieldPoint => ({ x: a.x + (b.x - a.x) * p, y: a.y + (b.y - a.y) * p });

export function positionAt(c: Pick<Command, 'type' | 'originId' | 'targetId' | 'point' | 'departedAt' | 'arrivesAt'>, t: number): FieldPoint | null {
  const ends = legEnds(c);
  return ends ? lerp(ends.from, ends.to, progress(c.departedAt, c.arrivesAt, t)) : null;
}

function smallestRoot(a: number, b: number, c: number): number | null {
  if (c === 0) return 0;
  if (Math.abs(a) < 1e-18) return b >= 0 ? null : -c / b;
  const disc = b * b - 4 * a * c;
  if (disc < 0) return null;
  const sq = Math.sqrt(disc);
  const [r1, r2] = [(-b - sq) / (2 * a), (-b + sq) / (2 * a)].sort((x, y) => x - y) as [number, number];
  if (r1 >= 0) return r1;
  return r2 >= 0 ? r2 : null;
}

/** The first moment from `now` an army leaving `home` at `minPerTile` can stand where the target is; null when not before it arrives. */
export function meetingTime(from: FieldPoint, to: FieldPoint, departedAt: number, arrivesAt: number, home: FieldPoint, minPerTile: number, unitSpeed: number, now: number): { t: number; at: FieldPoint } | null {
  if (now >= arrivesAt || minPerTile <= 0 || unitSpeed <= 0) return null;
  const v = unitSpeed / (minPerTile * MS_PER_MINUTE);
  const start = lerp(from, to, progress(departedAt, arrivesAt, now));
  let vx = 0, vy = 0;
  if (now >= departedAt && arrivesAt > departedAt) {
    vx = (to.x - from.x) / (arrivesAt - departedAt);
    vy = (to.y - from.y) / (arrivesAt - departedAt);
  }
  const dx = start.x - home.x, dy = start.y - home.y;
  const s = smallestRoot(vx * vx + vy * vy - v * v, 2 * (dx * vx + dy * vy), dx * dx + dy * dy);
  if (s === null) return null;
  const at = { x: start.x + vx * s, y: start.y + vy * s };
  const t = now + travelTimeMs(Math.hypot(home.x - at.x, home.y - at.y), minPerTile, unitSpeed);
  return t < arrivesAt ? { t, at } : null;
}
