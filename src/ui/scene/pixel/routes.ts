import type { BuildingId } from '../../../engine/types';
import type { SceneModel } from '../sceneModel';
import { GATE_Z, LAYOUT } from './layout';
import { loop, pathLength, type Pt } from './path';

export type WalkerState = 'walk' | 'carry_log' | 'carry_sack' | 'guard';

export interface Route {
  state: WalkerState;
  points: Pt[];
  /** Start distance along the loop in px, so walkers on one road do not overlap. */
  offset: number;
  /** px per second. */
  speed: number;
  /** Draw order against buildings and the wall halves. */
  z: number;
  /**
   * Draw order that changes along the loop: each zone holds from the previous one's `until` (a distance along the loop)
   * to its own. A route without zones keeps `z` all the way round.
   */
  zones?: Zone[];
}

export interface Zone { until: number; z: number }

const ORDER: BuildingId[] = ['main', 'barracks', 'market', 'stable', 'academy', 'smithy', 'storage', 'workshop', 'hide'];
const SPEED = 17;
const GUARD_SPEED = 12;

function road(key: string): Pt[] | null {
  return LAYOUT.roads[key]?.points ?? null;
}

function insideZ(points: Pt[]): number {
  return Math.max(...points.map((p) => p[1])) + 12;
}

/** Guards keep this far inside the wall line, so they walk on the grass rather than against the wall's face. */
const GUARD_INSET = { x: 46, y: 37 };

function arc(start: number, step: number, n: number): Pt[] {
  const { cx, cy, rx, ry } = LAYOUT.ring;
  return Array.from({ length: n }, (_, i) => [cx + (rx - GUARD_INSET.x) * Math.cos(start + i * step), cy + (ry - GUARD_INSET.y) * Math.sin(start + i * step)] as Pt);
}

/**
 * Zones of a there-and-back route over `points`, from each point's draw order: a stretch takes the lower order of its
 * two ends, so it changes as soon as the walker heads into a stretch drawn further back.
 */
export function zonesOf(points: Pt[], zs: number[]): Zone[] {
  const pts = loop(points);
  const z = [...zs, ...zs.slice(1, -1).reverse()];
  const zones: Zone[] = [];
  let dist = 0;
  for (let i = 1; i < pts.length; i++) {
    dist += pathLength([pts[i - 1]!, pts[i]!]);
    const zi = Math.min(z[i - 1]!, z[i]!);
    const last = zones.at(-1);
    if (last && last.z === zi) last.until = dist;
    else zones.push({ until: dist, z: zi });
  }
  return zones;
}

/** The zone a walker `dist` along its loop is in. */
export function zoneAt(zones: Zone[], dist: number): number {
  const i = zones.findIndex((zone) => dist <= zone.until);
  return i < 0 ? zones.length - 1 : i;
}

/** Just behind the back wall where it passes over `x`: a walker out through a side gate goes round behind it. */
function behindWall(x: number): number {
  const strips = LAYOUT.walls.stone.strips;
  const strip = [...strips].reverse().find((s) => s.x0 <= x) ?? strips[0]!;
  return strip.zBack - 0.5;
}

/**
 * A walk from a building's door out through a side gate: in front of the building on its own road, behind it on the
 * side road that passes it, and behind the back wall from the gate on.
 */
function sideRoute(state: WalkerState, building: BuildingId, own: Pt[], side: Pt[], gate: Pt, offset: number): Route {
  const front = insideZ(own);
  const behind = LAYOUT.slots[building as Exclude<BuildingId, 'wall'>][1] + 12 - 0.5;
  const out = side.findIndex(([x, y]) => x === gate[0] && y === gate[1]);
  const zs = [...own.map(() => front), ...side.slice(1).map((_, i) => (out >= 0 && i + 1 >= out ? behindWall(gate[0]) : behind))];
  const points = own.concat(side.slice(1));
  return { state, points: loop(points), offset, speed: SPEED, z: front, zones: zonesOf(points, zs) };
}

/** Guards sort by where their feet are, in steps, so they pass in front of a building or behind it. */
const GUARD_Z_STEP = 4;

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
    extras.push({ state: 'walk', points: loop([...gate].reverse().concat([outside])), offset: 20, speed: SPEED, z: GATE_Z.walker });
    const site = m.construction.find((c) => c.started)?.building;
    const siteRoad = site ? road(site) : null;
    const wood = road('wood');
    if (siteRoad && levels.wood > 0 && wood) {
      const fork: Pt = [gx + 6, LAYOUT.ring.cy + LAYOUT.ring.ry + 26];
      extras.push({ state: 'carry_log', points: loop([...wood].reverse().concat([fork], gate, siteRoad.slice(1))), offset: 0, speed: SPEED, z: GATE_Z.walker });
    }
    const storage = road('storage');
    if (siteRoad && site !== 'storage' && levels.storage > 0 && storage) {
      const pts = [...storage].reverse().concat(siteRoad.slice(1));
      extras.push({ state: 'carry_sack', points: loop(pts), offset: 60, speed: SPEED, z: insideZ(pts) });
    }
    const sideW = road('side_w');
    if (levels.iron > 0 && storage && sideW) extras.push(sideRoute('walk', 'storage', storage, sideW, LAYOUT.gates.w, 10));
    const academy = road('academy');
    const sideE = road('side_e');
    if (levels.farm > 0 && academy && sideE) extras.push(sideRoute('carry_sack', 'academy', academy, sideE, LAYOUT.gates.e, 70));
    routes.push(...extras.slice(0, Math.max(0, budget.citizens - routes.length)));
  }

  const arcs = [arc(Math.PI + 0.3, 0.012, 130), arc(-0.2, -0.012, 110)];
  arcs.slice(0, Math.min(budget.guards, 2)).forEach((pts, i) => {
    const zs = pts.map(([, y]) => Math.ceil(y / GUARD_Z_STEP) * GUARD_Z_STEP);
    routes.push({ state: 'guard', points: loop(pts), offset: i * 50, speed: GUARD_SPEED, z: Math.min(...zs), zones: zonesOf(pts, zs) });
  });
  return routes;
}

export function gateOpen(positions: Pt[], gate: Pt): boolean {
  return positions.some(([x, y]) => Math.hypot(x - gate[0], y - gate[1]) < 44);
}

/** px per second the mower advances while mowing; he walks back four times as fast. */
export const MOW_SPEED = 3;

/** Where the mower is `t` seconds in: mowing along `pass`, then walking back to its start, over and over. */
export function mowerAt([from, to]: [Pt, Pt], t: number): { at: Pt; mowing: boolean } {
  const len = Math.hypot(to[0] - from[0], to[1] - from[1]);
  const mowing = len / MOW_SPEED;
  const u = t % (mowing + len / (4 * MOW_SPEED));
  const lerp = (a: Pt, b: Pt, k: number): Pt => [a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k];
  if (u < mowing) return { at: lerp(from, to, u / mowing), mowing: true };
  return { at: lerp(to, from, (u - mowing) / (len / (4 * MOW_SPEED))), mowing: false };
}

/** px per second a stable rider's horse walks. */
export const RIDE_SPEED = 10;

/** Where a rider is `t` seconds in: walking the horse from `from` to `to`, then back, facing the way it goes. */
export function riderAt([from, to]: [Pt, Pt], t: number): { at: Pt; dir: 'east' | 'west' } {
  const leg = Math.hypot(to[0] - from[0], to[1] - from[1]) / RIDE_SPEED;
  const u = t % (2 * leg);
  const [a, b] = u < leg ? [from, to] : [to, from];
  const k = (u < leg ? u : u - leg) / leg;
  return { at: [a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k], dir: b[0] >= a[0] ? 'east' : 'west' };
}
