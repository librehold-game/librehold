import { describe, it, expect } from 'vitest';
import { BUILDING_IDS, type BuildingId } from '../src/engine/types';
import { LAYOUT } from '../src/ui/scene/pixel/layout';
import { buildRoutes, gateOpen } from '../src/ui/scene/pixel/routes';

const zero = Object.fromEntries(BUILDING_IDS.map((id) => [id, 0])) as Record<BuildingId, number>;
const nine = { ...zero, main: 5, barracks: 5, market: 5, stable: 5, academy: 1, smithy: 5, storage: 5, workshop: 5, hide: 5 };

describe('walker routes', () => {
  it('no citizens and no buildings give no routes', () => {
    expect(buildRoutes({ levels: zero, construction: [] }, { citizens: 0, guards: 0 })).toEqual([]);
  });

  it('5 citizens and 9 buildings give 5 walking routes, each starting at the plaza', () => {
    const routes = buildRoutes({ levels: nine, construction: [] }, { citizens: 5, guards: 0 });
    expect(routes).toHaveLength(5);
    for (const r of routes) {
      expect(r.state).toBe('walk');
      expect(Math.abs(r.points[0]![0] - LAYOUT.plaza[0])).toBeLessThanOrEqual(1);
      expect(Math.abs(r.points[0]![1] - LAYOUT.plaza[1])).toBeLessThanOrEqual(1);
    }
  });

  it('the citizen budget caps every route, extra walkers only fill what buildings leave', () => {
    const levels = { ...zero, main: 5, wood: 5, storage: 5, iron: 5 };
    expect(buildRoutes({ levels, construction: [] }, { citizens: 3, guards: 0 })).toHaveLength(3);
    expect(buildRoutes({ levels, construction: [] }, { citizens: 18, guards: 0 }).length).toBeLessThanOrEqual(18);
  });

  it('a log carrier appears only with a sawmill and a started construction', () => {
    const levels = { ...zero, main: 5, wood: 5 };
    const started = [{ building: 'main' as BuildingId, ratio: 0.2, started: true }];
    const queued = [{ building: 'main' as BuildingId, ratio: 0, started: false }];
    const has = (l: typeof levels, c: typeof started) => buildRoutes({ levels: l, construction: c }, { citizens: 10, guards: 0 }).some((r) => r.state === 'carry_log');
    expect(has(levels, started)).toBe(true);
    expect(has(levels, queued)).toBe(false);
    expect(has({ ...levels, wood: 0 }, started)).toBe(false);
  });

  it('routes only use roads that exist for the current levels', () => {
    const levels = { ...zero, main: 5 };
    const routes = buildRoutes({ levels, construction: [{ building: 'iron', ratio: 0.5, started: true }] }, { citizens: 18, guards: 2 });
    expect(routes.every((r) => r.points.every(([x, y]) => Number.isFinite(x) && Number.isFinite(y)))).toBe(true);
    expect(routes.some((r) => r.state === 'carry_log' || r.state === 'carry_sack')).toBe(false);
  });

  it('side-road walkers go to the mine and the farm without needing the storage or the academy', () => {
    const routes = buildRoutes({ levels: { ...zero, main: 5, iron: 3, farm: 3 }, construction: [] }, { citizens: 18, guards: 0 });
    const ends = routes.map((r) => r.points[Math.floor(r.points.length / 2)]!);
    const near = (p: number[], q: number[]) => Math.hypot(p[0]! - q[0]!, p[1]! - q[1]!) < 60;
    expect(ends.some((p) => near(p, LAYOUT.slots.iron))).toBe(true);
    expect(ends.some((p) => near(p, LAYOUT.slots.farm))).toBe(true);
  });

  it('guards walk inside the ring', () => {
    const routes = buildRoutes({ levels: { ...zero, wall: 10 }, construction: [] }, { citizens: 0, guards: 3 });
    const guards = routes.filter((r) => r.state === 'spear');
    expect(guards).toHaveLength(2);
    const { cx, cy, rx, ry } = LAYOUT.ring;
    for (const g of guards) for (const [x, y] of g.points) expect(((x - cx) / rx) ** 2 + ((y - cy) / ry) ** 2).toBeLessThan(1);
  });

  it('gateOpen: 40 px from the gate opens it, 50 px does not', () => {
    expect(gateOpen([[400, 0], [440, 474]], [440, 514])).toBe(true);
    expect(gateOpen([[440, 464]], [440, 514])).toBe(false);
  });
});
