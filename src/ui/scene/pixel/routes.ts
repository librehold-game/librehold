import type { BuildingId } from '../../../engine/types';
import type { SceneModel } from '../sceneModel';
import { LAYOUT } from './layout';
import { loop, type Pt } from './path';

export type WalkerState = 'walk' | 'carry_log' | 'carry_sack' | 'spear';

export interface Route {
  state: WalkerState;
  points: Pt[];
  /** Start distance along the loop in px, so walkers on one road do not overlap. */
  offset: number;
  /** px per second. */
  speed: number;
  /** Draw order against buildings and the wall halves. */
  z: number;
}

const ORDER: BuildingId[] = ['main', 'barracks', 'market', 'stable', 'academy', 'smithy', 'storage', 'workshop', 'hide'];
const SPEED = 17;
const GUARD_SPEED = 12;
/** Through the south gate: drawn over the front wall, under the gatehouse. */
const GATE_Z = LAYOUT.ring.cy + LAYOUT.ring.ry + 2;

function road(key: string): Pt[] | null {
  return LAYOUT.roads[key]?.points ?? null;
}

function insideZ(points: Pt[]): number {
  return Math.max(...points.map((p) => p[1])) + 12;
}

function arc(start: number, step: number, n: number): Pt[] {
  const { cx, cy, rx, ry } = LAYOUT.ring;
  return Array.from({ length: n }, (_, i) => [cx + (rx - 30) * Math.cos(start + i * step), cy + (ry - 24) * Math.sin(start + i * step)] as Pt);
}

/** Walker routes for the current village: building walkers first, then carriers and side-road walkers, then guards. */
export function buildRoutes(m: Pick<SceneModel, 'levels' | 'construction'>, budget: { citizens: number; guards: number }): Route[] {
  const routes: Route[] = [];
  const { levels } = m;
  ORDER.filter((id) => levels[id] > 0).forEach((id, i) => {
    const pts = road(id);
    if (pts && routes.length < budget.citizens) routes.push({ state: 'walk', points: loop(pts), offset: i * 41, speed: SPEED, z: insideZ(pts) });
  });

  if (budget.citizens >= 3) {
    const extras: Route[] = [];
    const gate = road('gate')!.slice(1);
    const [gx] = LAYOUT.gates.s;
    const outside: Pt = [gx, LAYOUT.ring.cy + LAYOUT.ring.ry + 60];
    extras.push({ state: 'walk', points: loop([...gate].reverse().concat([outside])), offset: 20, speed: SPEED, z: GATE_Z });
    const site = m.construction.find((c) => c.started)?.building;
    const siteRoad = site ? road(site) : null;
    const wood = road('wood');
    if (siteRoad && levels.wood > 0 && wood) {
      const fork: Pt = [gx + 6, LAYOUT.ring.cy + LAYOUT.ring.ry + 26];
      extras.push({ state: 'carry_log', points: loop([...wood].reverse().concat([fork], gate, siteRoad.slice(1))), offset: 0, speed: SPEED, z: GATE_Z });
    }
    const storage = road('storage');
    if (siteRoad && site !== 'storage' && levels.storage > 0 && storage) {
      const pts = [...storage].reverse().concat(siteRoad.slice(1));
      extras.push({ state: 'carry_sack', points: loop(pts), offset: 60, speed: SPEED, z: insideZ(pts) });
    }
    const sideW = road('side_w');
    if (levels.iron > 0 && storage && sideW) {
      const pts = storage.concat(sideW.slice(1));
      extras.push({ state: 'walk', points: loop(pts), offset: 10, speed: SPEED, z: insideZ(storage) });
    }
    const academy = road('academy');
    const sideE = road('side_e');
    if (levels.farm > 0 && academy && sideE) {
      const pts = academy.concat(sideE.slice(1));
      extras.push({ state: 'carry_sack', points: loop(pts), offset: 70, speed: SPEED, z: insideZ(academy) });
    }
    routes.push(...extras.slice(0, Math.max(0, budget.citizens - routes.length)));
  }

  const arcs = [arc(Math.PI + 0.3, 0.012, 130), arc(-0.2, -0.012, 110)];
  arcs.slice(0, Math.min(budget.guards, 2)).forEach((pts, i) => {
    routes.push({ state: 'spear', points: loop(pts), offset: i * 50, speed: GUARD_SPEED, z: Math.min(...pts.map((p) => p[1])) });
  });
  return routes;
}

export function gateOpen(positions: Pt[], gate: Pt): boolean {
  return positions.some(([x, y]) => Math.hypot(x - gate[0], y - gate[1]) < 44);
}
