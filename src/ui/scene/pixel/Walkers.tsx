import { useCallback, useEffect, useMemo, useRef } from 'react';
import { LAYOUT, pixelUrl, type Layer, type VillagerSprite } from './layout';
import { facing, pathLength, pointAt, type Facing, type Pt } from './path';
import { gateOpen, mowerAt, type Route } from './routes';

/** One walk cycle, two steps, lasts this long whatever the frame count, so the feet keep pace with the ground. */
export const WALK_CYCLE_MS = 1080;
/** Frame time of the working loops: the woodcutter, the builder and the mower. */
export const WORK_STEP_MS = 130;

/** A villager working on the spot; the mower also moves, along his pass to `to` and back (see `mowerAt`). */
export interface Worker { key: string; sprite: 'chop' | 'hammer' | 'mow'; x: number; y: number; to?: Pt }

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

function Sprite({ s, dir, at, bind, id, walker }: { s: VillagerSprite; dir: Facing; at: Pt; bind?: Bind; id: string; walker?: string }) {
  return (
    <g ref={bind?.(id)} data-walker={walker} transform={place(s, at)}>
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
        const el = nodes.current.get(`w${i}`);
        const len = ls[i] ?? 0;
        if (!el || len === 0) return;
        const s = LAYOUT.villagers[r.state];
        const dist = (t * r.speed + r.offset) % len;
        const p = pointAt(r.points, dist);
        const dir = facing(p, pointAt(r.points, (dist + 1) % len));
        positions.push(p);
        el.setAttribute('transform', place(s, p));
        const view = el.firstElementChild as SVGSVGElement | null;
        view?.setAttribute('viewBox', frameView(s, Math.floor(now / (WALK_CYCLE_MS / s.frames)) % s.frames));
        if (facingOf.get(`w${i}`) !== dir) {
          facingOf.set(`w${i}`, dir);
          view?.firstElementChild?.setAttribute('href', pixelUrl(s.file[dir]!));
        }
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
        el?.firstElementChild?.setAttribute('viewBox', frameView(s, Math.floor(now / WORK_STEP_MS) % s.frames));
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

/** Each walker is its own layer so it sorts against buildings and wall halves by its route's depth. */
export function walkerLayers(routes: Route[], bind: Bind): Layer[] {
  return routes.map((r, i) => {
    const s = LAYOUT.villagers[r.state];
    const start = pointAt(r.points, r.offset);
    return { z: r.z, key: `walker-${i}`, node: <Sprite s={s} dir="south" at={start} bind={bind} id={`w${i}`} walker={r.state} /> };
  });
}

export function workerLayers(workers: Worker[], bind: Bind): Layer[] {
  return workers.flatMap((w) => {
    const s = LAYOUT.villagers[w.sprite];
    const layers: Layer[] = [{ z: w.y, key: `worker-${w.key}`, node: <Sprite s={s} dir="east" at={[w.x, w.y]} bind={bind} id={w.key} walker={w.sprite} /> }];
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

/** Troops waiting at the rally point: standing guards in rows of four. */
export function troopLayers(count: number): Layer[] {
  const [rx, ry] = LAYOUT.rally;
  const s = LAYOUT.villagers.guard;
  return Array.from({ length: count }, (_, i) => {
    const at: Pt = [rx - 40 + (i % 4) * 14 - Math.floor(i / 4) * 6, ry + 34 + Math.floor(i / 4) * 9];
    return { z: at[1], key: `troop-${i}`, node: <Sprite s={s} dir="south" at={at} id={`t${i}`} /> };
  });
}
