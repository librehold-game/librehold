import { useCallback, useEffect, useRef } from 'react';
import type { AnimationMode } from '../../state/uiStore';
import { formatDuration } from '../format';
import { useDocumentHidden } from '../useDocumentHidden';
import { armyPosition, groundAt, type MapArmy } from './armies';
import type { ArmyClock } from './armyClock';

export type ToPx = (x: number, y: number) => [number, number];

/** What a marker shows at a moment; the layer renders it once and the loop keeps it current. */
export function markerState(a: MapArmy, now: number, toPx: ToPx, seed: number, isVillage: (x: number, y: number) => boolean): { px: number; py: number; boat: boolean; eta: string } {
  const pos = armyPosition(a, now);
  const [px, py] = toPx(pos.x, pos.y);
  return { px: Math.round(px), py: Math.round(py), boat: groundAt(seed, pos.x, pos.y, isVillage) === 'water', eta: formatDuration(Math.max(0, Math.ceil((a.arrivesAt - now) / 1000)) * 1000) };
}

export function applyMarker(el: SVGGElement, s: ReturnType<typeof markerState>): void {
  el.setAttribute('transform', `translate(${s.px}, ${s.py})`);
  if (el.hasAttribute('data-boat') !== s.boat) {
    if (s.boat) el.setAttribute('data-boat', '');
    else el.removeAttribute('data-boat');
    el.querySelector('[data-land]')?.setAttribute('display', s.boat ? 'none' : 'inline');
    el.querySelector('[data-water]')?.setAttribute('display', s.boat ? 'inline' : 'none');
  }
  const eta = el.querySelector('[data-eta]');
  if (eta && eta.textContent !== s.eta) eta.textContent = s.eta;
}

/**
 * Moves the bound markers every animation frame (or on every clock tick with animations off) without re-rendering,
 * like the village walkers. Returns a ref binder per army id.
 */
export function useArmyLoop(armies: MapArmy[], toPx: ToPx, mode: AnimationMode, seed: number, clock: ArmyClock, isVillage: (x: number, y: number) => boolean): (id: string) => (el: SVGGElement | null) => void {
  const nodes = useRef(new Map<string, SVGGElement>());
  const state = useRef({ armies, toPx, seed, clock, isVillage });
  state.current = { armies, toPx, seed, clock, isVillage };
  const hidden = useDocumentHidden();

  const bind = useCallback((id: string) => (el: SVGGElement | null) => {
    if (el) nodes.current.set(id, el);
    else nodes.current.delete(id);
  }, []);

  useEffect(() => {
    const update = () => {
      const { armies: as, toPx: px, seed: sd, clock: ck, isVillage: iv } = state.current;
      const now = ck.now();
      for (const a of as) {
        const el = nodes.current.get(a.id);
        if (el) applyMarker(el, markerState(a, now, px, sd, iv));
      }
    };
    const unsubscribe = clock.subscribe(update);
    let raf = 0;
    if (mode !== 'off' && !hidden && typeof requestAnimationFrame === 'function') {
      const frame = () => { update(); raf = requestAnimationFrame(frame); };
      raf = requestAnimationFrame(frame);
    }
    return () => { unsubscribe(); if (raf) cancelAnimationFrame(raf); };
  }, [mode, hidden, clock]);

  return bind;
}
