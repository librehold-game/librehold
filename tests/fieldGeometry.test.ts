import { describe, it, expect } from 'vitest';
import { travelTimeMs } from '../src/engine/commands';
import { legEnds, meetingTime, positionAt } from '../src/engine/fieldGeometry';
import { MS_PER_MINUTE, type Command } from '../src/engine/types';

const SPEAR = 18, LIGHT_CAV = 10;

describe('meeting point (same vectors as field_geometry_test.go)', () => {
  it('head-on armies meet where both have walked their share', () => {
    const arrives = 10 * SPEAR * MS_PER_MINUTE;
    const m = meetingTime({ x: 10, y: 0 }, { x: 0, y: 0 }, 0, arrives, { x: 0, y: 0 }, LIGHT_CAV, 1, 0)!;
    const want = 10 / (1 + LIGHT_CAV / SPEAR);
    expect(Math.abs(m.at.x - want)).toBeLessThan(0.01);
    expect(m.at.y).toBe(0);
    expect(Math.abs(m.t - travelTimeMs(m.at.x, LIGHT_CAV, 1))).toBeLessThanOrEqual(1);
  });

  it('a slower chaser never catches up', () => {
    expect(meetingTime({ x: 2, y: 0 }, { x: 40, y: 0 }, 0, 38 * SPEAR * MS_PER_MINUTE, { x: 0, y: 0 }, 30, 1, 0)).toBeNull();
  });

  it('a target a minute from home is too late to reach from far away', () => {
    const arrives = 10 * SPEAR * MS_PER_MINUTE;
    expect(meetingTime({ x: 0, y: 0 }, { x: 10, y: 0 }, 0, arrives, { x: 9, y: 20 }, LIGHT_CAV, 1, arrives - MS_PER_MINUTE)).toBeNull();
  });

  it('a late target can still be met close to its goal', () => {
    const arrives = 10 * SPEAR * MS_PER_MINUTE;
    const m = meetingTime({ x: 0, y: 0 }, { x: 10, y: 0 }, 0, arrives, { x: 10, y: 1 }, LIGHT_CAV, 1, arrives / 2)!;
    expect(m.t).toBeLessThan(arrives);
    expect(m.t).toBeGreaterThanOrEqual(arrives / 2);
  });

  it('a return from a field point moves linearly from the point', () => {
    const c = { type: 'return', originId: '', targetId: 'v:10:0', point: { x: 4, y: 0 }, departedAt: 1000, arrivesAt: 7000 } as Command;
    expect(positionAt(c, 4000)).toEqual({ x: 7, y: 0 });
    expect(positionAt(c, 0)).toEqual({ x: 4, y: 0 });
    expect(positionAt(c, 9000)).toEqual({ x: 10, y: 0 });
  });

  it('an intercept goes from its village to its point', () => {
    const c = { type: 'intercept', originId: 'v:3:4', targetId: '', point: { x: 5.5, y: 6 } } as Command;
    expect(legEnds(c)).toEqual({ from: { x: 3, y: 4 }, to: { x: 5.5, y: 6 } });
    expect(legEnds({ originId: 'nowhere', targetId: 'v:1:1' } as Command)).toBeNull();
  });
});
