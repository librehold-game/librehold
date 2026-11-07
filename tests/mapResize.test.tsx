// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render } from '@testing-library/react';
import { MapView } from '../src/ui/views/MapView';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { FRAME_H, FRAME_W, MAX_H, MAX_W, MIN_H, MIN_W, readMapWindow, mapViewportAt } from '../src/ui/map/viewport';
import { newDefaultGame } from './helpers';

/** The room a desktop browser gives the map: wide enough for the default window with the minimap beside it. */
const DESKTOP_W = 1100;
const PHONE_W = 360;

function restoreWidth(): () => void {
  const proto = globalThis.HTMLElement.prototype as unknown as Record<string, unknown>;
  const original = Object.getOwnPropertyDescriptor(proto, 'clientWidth');
  return () => {
    if (original) Object.defineProperty(proto, 'clientWidth', original);
    else delete proto.clientWidth;
  };
}

function setWidth(px: number): () => void {
  const restore = restoreWidth();
  const proto = globalThis.HTMLElement.prototype as unknown as Record<string, unknown>;
  Object.defineProperty(proto, 'clientWidth', { configurable: true, get: () => px });
  return restore;
}

/** happy-dom has no canvas; the terrain layer only needs a context that accepts every call. */
function fakeContext(): CanvasRenderingContext2D {
  const props: Record<string | symbol, unknown> = {};
  return new Proxy(props, {
    get: (t, k) => (k in t ? t[k] : () => undefined),
    set: (t, k, v) => { t[k] = v; return true; },
  }) as unknown as CanvasRenderingContext2D;
}

const surface = (c: HTMLElement) => c.querySelector<HTMLElement>('[data-map-surface]')!;
const grip = (c: HTMLElement) => c.querySelector<HTMLElement>('[data-map-resize]');
const minimap = (c: HTMLElement) => c.querySelector<SVGElement>('svg[data-minimap]')!;
/** The wooden frame box around the map surface. */
const frame = (c: HTMLElement) => surface(c).closest<HTMLElement>('.frame-wood')!;
/** The row/column box the map and the minimap sit in. */
const layout = (c: HTMLElement) => frame(c).parentElement!;
/** The box that holds the minimap and the details panel, under or beside the map. */
const under = (c: HTMLElement) => minimap(c).parentElement as HTMLElement;

function showMap(): HTMLElement {
  return render(<MapView />).container;
}

beforeEach(() => {
  vi.stubGlobal('innerHeight', 900);
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation((() => fakeContext()) as never);
  useGameStore.setState({ game: newDefaultGame(), phase: 'ready', devMode: false, notice: null, lastError: null });
  useUiStore.setState({ view: 'map' });
  localStorage.removeItem('lh-map-window');
});
afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('resizing the map from its bottom-right corner', () => {
  it('gives the desktop map a grabbable corner and keeps the minimap beside it', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      expect(grip(c)).not.toBeNull();
      expect(layout(c).className).toContain('flex-row');
      // The default window is the 15 × 13 fields, and its minimap stands to the right of it.
      expect(surface(c).style.width).toBe(`${MAX_W}px`);
      expect(minimap(c).getBoundingClientRect().left).toBeGreaterThanOrEqual(surface(c).getBoundingClientRect().right);
    } finally {
      restore();
    }
  });

  it('a drag on the corner shrinks the window and remembers the size', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      const before = surface(c).style.width;
      fireEvent.pointerDown(grip(c)!, { pointerId: 1, pointerType: 'mouse', clientX: 700, clientY: 500 });
      fireEvent.pointerMove(window, { pointerId: 1, clientX: 600, clientY: 440 });
      fireEvent.pointerUp(window, { pointerId: 1, clientX: 600, clientY: 440 });
      expect(surface(c).style.width).not.toBe(before);
      expect(surface(c).style.width).toBe(`${MAX_W - 100}px`);
      expect(surface(c).style.height).toBe(`${MAX_H - 60}px`);
      expect(readMapWindow()).toEqual({ w: MAX_W - 100, h: MAX_H - 60 });
    } finally {
      restore();
    }
  });

  it('dragging past the width the minimap needs moves the minimap under the map', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      expect(layout(c).className).toContain('flex-row');
      // The map may not grow past the room it is given — `maxW` leaves room for the frame's bevel.
      fireEvent.pointerDown(grip(c)!, { pointerId: 1, pointerType: 'mouse', clientX: 700, clientY: 500 });
      fireEvent.pointerMove(window, { pointerId: 1, clientX: 4000, clientY: 500 });
      fireEvent.pointerUp(window, { pointerId: 1, clientX: 4000, clientY: 500 });
      const map = surface(c).getBoundingClientRect();
      expect(map.width).toBeLessThanOrEqual(DESKTOP_W);
      expect(layout(c).className).toContain('flex-col');
      const mm = minimap(c).getBoundingClientRect();
      expect(mm.top).toBeGreaterThanOrEqual(map.bottom);
      // Centred under the map: the row under it is exactly as wide as the frame, so `justify-center` lands on the map
      // however wide the map is — while the map itself stays where it was, at the left of the row.
      expect(layout(c).className).toContain('items-start');
      expect(under(c).style.width).toBe(`${parseFloat(surface(c).style.width) + FRAME_W}px`);
      expect(Math.abs(mm.left + mm.width / 2 - (map.left + map.width / 2))).toBeLessThanOrEqual(1);
    } finally {
      restore();
    }
  });

  it('the size is kept between visits', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      fireEvent.pointerDown(grip(c)!, { pointerId: 1, pointerType: 'mouse', clientX: 700, clientY: 500 });
      fireEvent.pointerMove(window, { pointerId: 1, clientX: 650, clientY: 480 });
      fireEvent.pointerUp(window, { pointerId: 1, clientX: 650, clientY: 480 });
      const kept = surface(c).style.width;
      cleanup();
      const again = showMap();
      expect(surface(again).style.width).toBe(kept);
    } finally {
      restore();
    }
  });

  it('a phone has no corner to grab — the map still follows the screen', () => {
    const restore = setWidth(PHONE_W);
    try {
      const c = showMap();
      expect(grip(c)).toBeNull();
      expect(surface(c).getBoundingClientRect().width).toBeLessThanOrEqual(PHONE_W);
    } finally {
      restore();
    }
  });
});

describe('the wheel zooms the map and not the page', () => {
  /** `wheel` on the surface: what the browser would do with it (`defaultPrevented`) and the tile size after. */
  function wheelOn(c: HTMLElement, at: Element | null = null): { prevented: boolean; tileW: string } {
    const target = at ?? surface(c);
    const ev = new WheelEvent('wheel', { deltaY: 200, bubbles: true, cancelable: true });
    act(() => { target.dispatchEvent(ev); });
    return { prevented: ev.defaultPrevented, tileW: c.querySelector<HTMLElement>('[data-tile]')!.style.width };
  }

  it('a wheel over the map zooms it and cancels the scroll, so the page stays put', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      const before = c.querySelector<HTMLElement>('[data-tile]')!.style.width;
      const { prevented, tileW } = wheelOn(c);
      expect(prevented).toBe(true);
      expect(tileW).not.toBe(before);
    } finally {
      restore();
    }
  });

  it('a wheel over a card on the map scrolls the card instead of zooming the map', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      const before = c.querySelector<HTMLElement>('[data-tile]')!.style.width;
      const card = document.createElement('div');
      card.className = 'panel';
      card.style.overflowY = 'auto';
      surface(c).appendChild(card);
      const { prevented, tileW } = wheelOn(c, card);
      expect(tileW).toBe(before);
      // The wheel bubbling on to the page is fine: the card is what it scrolls.
      expect(prevented).toBe(false);
    } finally {
      restore();
    }
  });
});

describe('the village menu', () => {
  it('is anchored to the tile and keeps its own width, not the screen width', () => {
    const restore = setWidth(DESKTOP_W);
    try {
      const c = showMap();
      const village = c.querySelector<HTMLElement>('[data-tile-village]')!;
      fireEvent.click(village);
      const card = c.querySelector<HTMLElement>('[data-map-surface] .panel')!;
      expect(card).not.toBeNull();
      expect(card.className).toContain('absolute');
      expect(card.className).not.toContain('fixed');
      // Anchored to the tile: its own fixed width and a place on the surface, not the screen span a sheet takes.
      expect(card.style.width).toBe('256px');
      expect(card.style.left).not.toBe('');
      expect(card.style.top).not.toBe('');
    } finally {
      restore();
    }
  });

  it('still comes up as a sheet when the map itself is too narrow for an anchored card', () => {
    const restore = setWidth(PHONE_W);
    try {
      const c = showMap();
      fireEvent.click(c.querySelector<HTMLElement>('[data-tile-village]')!);
      const card = c.querySelector<HTMLElement>('.panel.fixed')!;
      expect(card).not.toBeNull();
      expect(card.closest('[data-map-surface]')).toBeNull();
    } finally {
      restore();
    }
  });
});

describe('the window a drag can make', () => {  it('never goes under the minimum and never past the room', () => {
    const tiny = mapViewportAt(MIN_W - 100, MIN_H - 100, 0);
    expect(tiny.width).toBe(MIN_W);
    expect(tiny.height).toBe(MIN_H);
    const big = mapViewportAt(4000, 3000, 0);
    expect(big.width).toBe(4000);
    expect(big.height).toBe(3000);
  });

  it('the field count follows the size it was given', () => {
    const vp = mapViewportAt(MAX_W + 200, MAX_H, 2);
    expect(vp.tw).toBe(53);
    expect(vp.th).toBe(38);
    expect(vp.cols).toBe(Math.ceil(vp.width / vp.tw) + 1);
    expect(vp.rows).toBe(Math.ceil(vp.height / vp.th) + 1);
    expect(vp.width + FRAME_W).toBeGreaterThan(MAX_W + FRAME_W);
    expect(vp.height + FRAME_H).toBe(MAX_H + FRAME_H);
  });
});
