export type Pt = [number, number];
export type Facing = 'south' | 'east' | 'north' | 'west';

export function pathLength(points: Pt[]): number {
  let total = 0;
  for (let i = 1; i < points.length; i++) total += Math.hypot(points[i]![0] - points[i - 1]![0], points[i]![1] - points[i - 1]![1]);
  return total;
}

/** Point at `dist` along the polyline; the distance is clamped to the path. */
export function pointAt(points: Pt[], dist: number): Pt {
  let left = Math.max(0, dist);
  for (let i = 1; i < points.length; i++) {
    const [ax, ay] = points[i - 1]!;
    const [bx, by] = points[i]!;
    const len = Math.hypot(bx - ax, by - ay);
    if (left <= len) {
      const t = len === 0 ? 0 : left / len;
      return [ax + (bx - ax) * t, ay + (by - ay) * t];
    }
    left -= len;
  }
  return points[points.length - 1]!;
}

/** Sprite direction for a move; mostly-horizontal moves face east/west, the rest south/north. */
export function facing(from: Pt, to: Pt): Facing {
  const dx = to[0] - from[0];
  const dy = to[1] - from[1];
  if (Math.abs(dx) >= Math.abs(dy) * 1.2) return dx > 0 ? 'east' : 'west';
  return dy > 0 ? 'south' : 'north';
}

/** There and back again, without repeating either end. */
export function loop(points: Pt[]): Pt[] {
  return [...points, ...points.slice(1, -1).reverse()];
}
