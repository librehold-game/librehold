import { useCallback, useEffect, useMemo, useRef } from 'react';
import { LAYOUT, pixelUrl, type Layer, type VillagerSprite } from './layout';
import { facing, pathLength, pointAt, type Facing, type Pt } from './path';
import { gateOpen, mowerAt, riderAt, zoneAt, type Route } from './routes';

/** One walk cycle, two steps, lasts this long whatever the frame count, so the feet keep pace with the ground. */
export const WALK_CYCLE_MS = 1080;
/** Frame time of the working loops: the woodcutter, the builder and the mower. */
export const WORK_STEP_MS = 130;

export type WorkSprite = 'shoot' | 'thrust' | 'slash' | 'axe_chop' | 'forge';
export type RideSprite = 'ride_scout' | 'ride_lightCav' | 'ride_mountedArcher' | 'ride_heavyCav';

/**
 * Units in training play their frames in this order, one per WORK_STEP_MS. The archer raises and draws the bow, holds the
 * aim, releases, lowers it and stands a moment; the spearman stands ready, jabs and pulls back (the frames before 3 hold
 * the spear upright, so they are left out); the swordsman and the axeman wind up, strike the dummy and hold a beat; the
 * blacksmith holds the hammer up a beat before the blow.
 */
export const WORK_SEQ: Record<WorkSprite, readonly number[]> = {
  shoot: [0, 1, 2, 3, 3, 3, 4, 5, 6, 6, 7, 7, 7, 7],
  thrust: [7, 7, 7, 6, 3, 4, 4, 5, 7, 7],
  slash: [0, 0, 1, 2, 3, 4, 5, 6, 7, 7],
  axe_chop: [0, 0, 1, 2, 2, 3, 4, 5, 6, 7],
  forge: [0, 1, 2, 2, 3, 4, 5, 6, 7],
};
export const SHOOT_SEQ = WORK_SEQ.shoot;
/** The frame in which the string is let go; the arrow leaves the bow here. */
export const SHOOT_RELEASE_FRAME = 6;
/** The arrow takes this many steps to reach the target, then stays in it until the archer draws again. */
const ARROW_STEPS = 2;
const RELEASE = SHOOT_SEQ.indexOf(SHOOT_RELEASE_FRAME);

/** The step of each drill's sequence in which the blow lands on the dummy, or the arrow reaches the target. */
export const WORK_HIT: Record<WorkSprite, number> = { shoot: RELEASE + ARROW_STEPS, thrust: 4, slash: 5, axe_chop: 6, forge: 5 };
/** The struck prop's sideways jolt in whole pixels, one per step from the hit; afterwards it stands still. */
const JOLT = [2, -1, 1];
/** Sparks off the anvil show for this many steps from the blow. */
const SPARK_STEPS = 2;

/**
 * A villager working on the spot; the mower also moves, along his pass to `to` and back (see `mowerAt`), and a stable
 * rider walks his horse to `ride` and back (see `riderAt`). `z` sorts a worker drawn inside a building's yard over that
 * building; `arrow` is the flight of the archer's shots.
 */
export interface Worker {
  key: string;
  sprite: 'chop' | 'hammer' | 'mow' | 'dig' | 'dig_rest' | 'chop_rest' | 'command' | WorkSprite | RideSprite;
  dir?: Facing; x: number; y: number; to?: Pt; ride?: Pt; z?: number; arrow?: { from: Pt; to: Pt }; sparks?: Pt;
}

function isSequenced(sprite: Worker['sprite']): sprite is WorkSprite {
  return sprite in WORK_SEQ;
}

type Bind = (key: string) => (el: SVGElement | null) => void;

/** The wheat stands in front of the mower's legs: the field is drawn again over him, cut to a band around his feet. */
const WHEAT = LAYOUT.scenery.find((s) => s.file === 'scenery/plot-wheat.png')!;
const WHEAT_COVER = { above: 8, below: 10 };

function frameView(s: VillagerSprite, frame: number): string {
  return `${frame * s.w} 0 ${s.w} ${s.h}`;
}

function place(s: VillagerSprite, [x, y]: Pt): string {
  return `translate(${Math.round(x - s.anchor[0])}, ${Math.round(y - s.anchor[1])})`;
}

function Sprite({ s, dir, at, bind, id, walker, hidden }: { s: VillagerSprite; dir: Facing; at: Pt; bind?: Bind; id: string; walker?: string; hidden?: boolean }) {
  return (
    <g ref={bind?.(id)} data-walker={walker} transform={place(s, at)} visibility={hidden ? 'hidden' : undefined}>
      <svg width={s.w} height={s.h} viewBox={frameView(s, 0)} overflow="hidden">
        <image href={pixelUrl(s.file[dir] ?? s.file.east!)} width={s.w * s.frames} height={s.h} />
      </svg>
    </g>
  );
}

/**
 * One requestAnimationFrame loop moves every walker and swings every worker by writing attributes straight to
 * their elements; React renders them once. `onGate` fires only when someone enters or leaves the gate radius.
 */
export function useWalkerLoop(routes: Route[], workers: Worker[], paused: boolean, onGate: (open: boolean) => void): Bind {
  const nodes = useRef(new Map<string, SVGElement>());
  const lengths = useMemo(() => routes.map((r) => pathLength(r.points)), [routes]);
  const state = useRef({ routes, workers, lengths, onGate });
  state.current = { routes, workers, lengths, onGate };

  const bind = useCallback<Bind>((key) => (el) => {
    if (el) nodes.current.set(key, el);
    else nodes.current.delete(key);
  }, []);

  useEffect(() => {
    if (paused) return;
    const start = performance.now();
    const facingOf = new Map<string, Facing>();
    const mowingOf = new Map<string, boolean>();
    let open = false;
    let raf = 0;
    const tick = (now: number) => {
      const { routes: rs, workers: ws, lengths: ls, onGate: gate } = state.current;
      const t = (now - start) / 1000;
      const positions: Pt[] = [];
      rs.forEach((r, i) => {
        const len = ls[i] ?? 0;
        if (len === 0) return;
        const s = LAYOUT.villagers[r.state];
        const dist = (t * r.speed + r.offset) % len;
        const p = pointAt(r.points, dist);
        const dir = facing(p, pointAt(r.points, (dist + 1) % len));
        positions.push(p);
        const { depths, ofZone } = walkerDepths(r);
        const shown = r.zones ? ofZone[zoneAt(r.zones, dist)]! : 0;
        const turned = facingOf.get(`w${i}`) !== dir;
        facingOf.set(`w${i}`, dir);
        depths.forEach((_, k) => {
          const el = nodes.current.get(`w${i}-${k}`);
          if (!el) return;
          el.setAttribute('transform', place(s, p));
          el.setAttribute('visibility', k === shown ? 'visible' : 'hidden');
          const view = el.firstElementChild as SVGSVGElement | null;
          view?.setAttribute('viewBox', frameView(s, Math.floor(now / (WALK_CYCLE_MS / s.frames)) % s.frames));
          if (turned) view?.firstElementChild?.setAttribute('href', pixelUrl(s.file[dir]!));
        });
      });
      for (const w of ws) {
        const el = nodes.current.get(w.key);
        if (w.to) {
          const { at, mowing } = mowerAt([[w.x, w.y], w.to], t);
          const s = LAYOUT.villagers[mowing ? 'mow' : 'scythe_walk'];
          el?.setAttribute('transform', place(s, at));
          el?.firstElementChild?.setAttribute('viewBox', frameView(s, Math.floor(now / WORK_STEP_MS) % s.frames));
          nodes.current.get(`${w.key}-wheat`)?.setAttribute('x', String(Math.round(at[0] - s.anchor[0])));
          if (mowingOf.get(w.key) !== mowing) {
            mowingOf.set(w.key, mowing);
            el?.firstElementChild?.firstElementChild?.setAttribute('href', pixelUrl((mowing ? s.file.east : s.file.west)!));
          }
          continue;
        }
        const s = LAYOUT.villagers[w.sprite];
        const step = Math.floor(now / WORK_STEP_MS);
        if (w.ride) {
          const { at, dir } = riderAt([[w.x, w.y], w.ride], t);
          el?.setAttribute('transform', place(s, at));
          el?.firstElementChild?.setAttribute('viewBox', frameView(s, step % s.frames));
          if (facingOf.get(w.key) !== dir) {
            facingOf.set(w.key, dir);
            el?.firstElementChild?.firstElementChild?.setAttribute('href', pixelUrl(s.file[dir]!));
          }
          continue;
        }
        if (!isSequenced(w.sprite)) {
          el?.firstElementChild?.setAttribute('viewBox', frameView(s, step % s.frames));
          continue;
        }
        const seq = WORK_SEQ[w.sprite];
        const i = step % seq.length;
        el?.firstElementChild?.setAttribute('viewBox', frameView(s, seq[i] ?? 0));
        nodes.current.get(`${w.key}-struck`)?.setAttribute('transform', `translate(${JOLT[i - WORK_HIT[w.sprite]] ?? 0}, 0)`);
        const sparkStep = i - WORK_HIT[w.sprite];
        nodes.current.get(`${w.key}-sparks`)?.setAttribute('visibility', sparkStep >= 0 && sparkStep < SPARK_STEPS ? 'visible' : 'hidden');
        const arrow = nodes.current.get(`${w.key}-arrow`);
        if (!arrow || !w.arrow) continue;
        if (i < RELEASE) {
          arrow.setAttribute('visibility', 'hidden');
          continue;
        }
        const p = Math.min(1, (now / WORK_STEP_MS - (step - i) - RELEASE) / ARROW_STEPS);
        const { from, to } = w.arrow;
        arrow.setAttribute('transform', `translate(${Math.round(from[0] + (to[0] - from[0]) * p)}, ${Math.round(from[1] + (to[1] - from[1]) * p)})`);
        arrow.setAttribute('visibility', 'visible');
      }
      const near = gateOpen(positions, LAYOUT.gates.s);
      if (near !== open) {
        open = near;
        gate(near);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(raf);
      state.current.onGate(false);
    };
  }, [paused]);

  return bind;
}

const depthsOf = new WeakMap<Route, { depths: number[]; ofZone: number[] }>();

/** The distinct draw orders a walker takes along its route, and which of them each zone uses. */
function walkerDepths(r: Route): { depths: number[]; ofZone: number[] } {
  let d = depthsOf.get(r);
  if (!d) {
    const depths = r.zones ? [...new Set(r.zones.map((zone) => zone.z))] : [r.z];
    d = { depths, ofZone: r.zones ? r.zones.map((zone) => depths.indexOf(zone.z)) : [0] };
    depthsOf.set(r, d);
  }
  return d;
}

/**
 * Each walker is its own layer so it sorts against buildings and wall halves by its route's depth. A walker whose
 * depth changes along the route has one copy per depth; the loop shows the one for where it is.
 */
export function walkerLayers(routes: Route[], bind: Bind): Layer[] {
  return routes.flatMap((r, i) => {
    const s = LAYOUT.villagers[r.state];
    const start = pointAt(r.points, r.offset);
    const { depths, ofZone } = walkerDepths(r);
    const shown = r.zones ? ofZone[zoneAt(r.zones, r.offset)] : 0;
    return depths.map((z, k) => ({
      z, key: `walker-${i}-${k}`,
      node: <Sprite s={s} dir="south" at={start} bind={bind} id={`w${i}-${k}`} walker={r.state} hidden={k !== shown} />,
    }));
  });
}

export function workerLayers(workers: Worker[], bind: Bind): Layer[] {
  return workers.flatMap((w) => {
    const s = LAYOUT.villagers[w.sprite];
    const z = w.z ?? w.y;
    const layers: Layer[] = [{ z, key: `worker-${w.key}`, node: <Sprite s={s} dir={w.dir ?? 'east'} at={[w.x, w.y]} bind={bind} id={w.key} walker={w.sprite} /> }];
    if (w.sparks) {
      const [x, y] = w.sparks;
      layers.push({
        z: z + 0.002,
        key: `worker-${w.key}-sparks`,
        node: (
          <g ref={bind(`${w.key}-sparks`)} data-sparks={w.key} visibility="hidden" transform={`translate(${x}, ${y})`}>
            <rect x={-3} y={-2} width={1} height={1} fill="#ffd34a" />
            <rect x={2} y={-3} width={1} height={1} fill="#ffef9a" />
            <rect x={3} y={0} width={1} height={1} fill="#ff9a2a" />
            <rect x={-1} y={-5} width={1} height={1} fill="#ffd34a" />
            <rect x={0} y={-1} width={1} height={1} fill="#fff6c8" />
          </g>
        ),
      });
    }
    if (w.arrow) {
      const [x, y] = w.arrow.from;
      layers.push({
        z: z + 0.001,
        key: `worker-${w.key}-arrow`,
        node: (
          <g ref={bind(`${w.key}-arrow`)} data-arrow={w.key} visibility="hidden" transform={`translate(${x}, ${y})`}>
            <rect x={-8} y={-1} width={2} height={1} fill="#e8e0d0" />
            <rect x={-7} y={0} width={6} height={1} fill="#5a3a1c" />
            <rect x={-1} y={0} width={2} height={1} fill="#d8d8d8" />
          </g>
        ),
      });
    }
    if (w.to) {
      const [ww, wh] = LAYOUT.files[WHEAT.file]!;
      const clip = `px-${w.key}-wheat`;
      layers.push({
        z: w.y + 0.5,
        key: `worker-${w.key}-wheat`,
        node: (
          <>
            <clipPath id={clip}>
              <rect ref={bind(`${w.key}-wheat`)} x={Math.round(w.x - s.anchor[0])} y={w.y - WHEAT_COVER.above} width={s.w} height={WHEAT_COVER.above + WHEAT_COVER.below} />
            </clipPath>
            <g clipPath={`url(#${clip})`} data-cover={w.key}>
              <image href={pixelUrl(WHEAT.file)} x={Math.floor(WHEAT.x - ww / 2)} y={Math.floor(WHEAT.y - wh)} width={ww} height={wh} />
            </g>
          </>
        ),
      });
    }
    return layers;
  });
}

/** Troops waiting at the rally point: standing guards in rows of four, right of its campfire. */
export function troopLayers(count: number): Layer[] {
  const [rx, ry] = LAYOUT.rally;
  const s = LAYOUT.villagers.guard;
  return Array.from({ length: count }, (_, i) => {
    const at: Pt = [rx + 31 + (i % 4) * 12 - Math.floor(i / 4) * 6, ry + 28 + Math.floor(i / 4) * 9];
    return { z: at[1], key: `troop-${i}`, node: <Sprite s={s} dir="south" at={at} id={`t${i}`} /> };
  });
}
