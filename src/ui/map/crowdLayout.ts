import type { UnitId } from '../../engine/types';

export type CrowdUnit = UnitId | 'torch' | 'shield';

const ROW = 6;
/** Rows sit closer than files: the map is seen from above at an angle. */
const ROW_DEPTH = 0.7;

/**
 * A marching column behind its head at (0, 0): rows of up to six across the heading, unit types kept together, and
 * the hetmans walking in the middle of the column where they are best protected.
 */
export function crowdLayout(crowd: { unit: CrowdUnit; figures: number }[], heading: { dx: number; dy: number }, spacing: number): { unit: CrowdUnit; dx: number; dy: number }[] {
  const groups = crowd.filter((c) => c.figures > 0 && c.unit !== 'noble');
  const nobles = crowd.filter((c) => c.figures > 0 && c.unit === 'noble');
  groups.splice(Math.ceil(groups.length / 2), 0, ...nobles);
  const units = groups.flatMap((c) => Array.from({ length: c.figures }, () => c.unit));
  const len = Math.hypot(heading.dx, heading.dy) || 1;
  const hx = heading.dx / len, hy = heading.dy / len;
  const px = -hy, py = hx;
  return units.map((unit, i) => {
    const row = Math.floor(i / ROW);
    const inRow = Math.min(ROW, units.length - row * ROW);
    const side = ((i % ROW) - (inRow - 1) / 2) * spacing;
    const back = row * spacing * ROW_DEPTH;
    return { unit, dx: -hx * back + px * side, dy: -hy * back + py * side };
  });
}
