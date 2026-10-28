import { useCallback, useEffect, useMemo, useRef } from 'react';
import { LAYOUT, pixelUrl, type Layer, type VillagerSprite } from './layout';
import { facing, pathLength, pointAt, type Facing, type Pt } from './path';
import { gateOpen, type Route } from './routes';

const STEP_MS = 180;
/** Tool swing for the woodcutter and the builder: wind up, strike, hold, return. */
const SWING = [0, 1, 2, 2, 2, 1, 0, 0];

export interface Worker { key: string; sprite: 'chop' | 'hammer'; x: number; y: number }

type Bind = (key: string) => (el: SVGGElement | null) => void;

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
  const nodes = useRef(new Map<string, SVGGElement>());
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
    let open = false;
    let raf = 0;
    const tick = (now: number) => {
      const { routes: rs, workers: ws, lengths: ls, onGate: gate } = state.current;
      const t = (now - start) / 1000;
      const step = Math.floor(now / STEP_MS);
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
        view?.setAttribute('viewBox', frameView(s, step % s.frames));
        if (facingOf.get(`w${i}`) !== dir) {
          facingOf.set(`w${i}`, dir);
          view?.firstElementChild?.setAttribute('href', pixelUrl(s.file[dir]!));
        }
      });
      for (const w of ws) {
        const s = LAYOUT.villagers[w.sprite];
        nodes.current.get(w.key)?.firstElementChild?.setAttribute('viewBox', frameView(s, SWING[(step + (w.sprite === 'hammer' ? 3 : 0)) % SWING.length]!));
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
  return workers.map((w) => ({
    z: w.y,
    key: `worker-${w.key}`,
    node: <Sprite s={LAYOUT.villagers[w.sprite]} dir="east" at={[w.x, w.y]} bind={bind} id={w.key} walker={w.sprite} />,
  }));
}

/** Troops waiting at the rally point: standing spearmen in rows of four. */
export function troopLayers(count: number): Layer[] {
  const [rx, ry] = LAYOUT.rally;
  const s = LAYOUT.villagers.spear;
  return Array.from({ length: count }, (_, i) => {
    const at: Pt = [rx - 40 + (i % 4) * 14 - Math.floor(i / 4) * 6, ry + 34 + Math.floor(i / 4) * 9];
    return { z: at[1], key: `troop-${i}`, node: <Sprite s={s} dir="south" at={at} id={`t${i}`} /> };
  });
}
