// @vitest-environment happy-dom
import { describe, it, expect, afterEach, beforeAll, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { CHUNK, TerrainLayer, chunkPaint, chunksInView, drawChunk, type TerrainSheets } from '../src/ui/map/TerrainLayer';
import { MapView } from '../src/ui/views/MapView';
import { useGameStore } from '../src/state/gameStore';
import { BIOME, tileBiome } from '../src/ui/map/biomes';
import { formatCoords } from '../src/ui/format';
import { newDefaultGame } from './helpers';

type Call = [string, ...unknown[]];
const contexts = new WeakMap<HTMLCanvasElement, { log: Call[] }>();

/** A 2D context that records what is drawn; happy-dom has no canvas. */
function recordingContext(canvas: HTMLCanvasElement) {
  let ctx = contexts.get(canvas);
  if (!ctx) {
    const log: Call[] = [];
    const rec = (name: string) => (...args: unknown[]) => { log.push([name, ...args]); };
    ctx = Object.assign({ log }, Object.fromEntries(['beginPath', 'rect', 'clip', 'drawImage', 'setTransform', 'clearRect', 'save', 'restore', 'translate', 'scale'].map((n) => [n, rec(n)])));
    contexts.set(canvas, ctx);
  }
  return ctx;
}

/** Calls of the last draw on a canvas: everything after its last clearRect. */
function lastDraw(canvas: HTMLCanvasElement): Call[] {
  const log = recordingContext(canvas).log;
  const at = log.map((c) => c[0]).lastIndexOf('clearRect');
  return log.slice(at);
}

class FakeImage {
  width = 32;
  height = 32;
  onload: (() => void) | null = null;
  set src(_: string) { queueMicrotask(() => this.onload?.()); }
}

const img = (width: number, height: number) => ({ width, height }) as HTMLImageElement;
const SHEET: TerrainSheets = {
  wang: { water: img(128, 128), forest: img(128, 128), mountain: img(128, 128) },
  trees: { oak: img(28, 35), pine: img(16, 35) },
  peaks: Array.from({ length: 7 }, () => img(74, 74)),
  decals: { dry: [0, 1, 2].map(() => img(64, 64)), flower: [0, 1, 2, 3, 4].map(() => img(24, 22)), dirt: [0, 1].map(() => img(64, 64)) },
  decor: Array.from({ length: 12 }, () => img(24, 22)),
};

beforeAll(() => {
  vi.stubGlobal('Image', FakeImage);
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(function (this: HTMLCanvasElement) {
    return recordingContext(this) as unknown as CanvasRenderingContext2D;
  } as never);
});
afterEach(cleanup);

const VIEW = { x0: 0, y0: 0, cols: 16, rows: 14, offX: 0, offY: 0, tw: 53, th: 38, mapSize: 200 };

describe('terrain chunks in view', () => {
  it('covers the visible tiles and one ring of chunks around them, so panning finds them drawn', () => {
    const chunks = chunksInView(VIEW);
    expect(chunks.map((c) => `${c.cx}|${c.cy}`).sort()).toEqual(['0|0', '0|1', '0|2', '1|0', '1|1', '1|2', '2|0', '2|1', '2|2']);
    const c = chunks.find((k) => k.cx === 1 && k.cy === 0)!;
    expect(c).toEqual({ cx: 1, cy: 0, left: 424, top: 0, width: CHUNK * 53, height: CHUNK * 38, visible: true });
    expect(chunks.filter((k) => k.visible).map((k) => `${k.cx}|${k.cy}`).sort()).toEqual(['0|0', '0|1', '1|0', '1|1']);
  });

  it('never reaches before the world start', () => {
    const chunks = chunksInView({ ...VIEW, x0: -5, y0: -5 });
    expect(chunks.length).toBeGreaterThan(0);
    expect(chunks.every((c) => c.cx >= 0 && c.cy >= 0)).toBe(true);
  });

  it('never reaches past the world end', () => {
    const chunks = chunksInView({ ...VIEW, x0: 195 });
    expect(chunks.length).toBeGreaterThan(0);
    expect(Math.max(...chunks.map((c) => c.cx))).toBe(24);
  });

  it('adjacent chunks leave no gap at a fractional offset', () => {
    const chunks = chunksInView({ ...VIEW, x0: 3, offX: 12.7, tw: 37, cols: 30 });
    const row = chunks.filter((c) => c.cy === 0).sort((a, b) => a.cx - b.cx);
    expect(row.length).toBeGreaterThan(1);
    for (let i = 1; i < row.length; i++) expect(row[i]!.left - row[i - 1]!.left).toBe(CHUNK * 37);
  });
});

describe('terrain layer', () => {
  it('renders a canvas per chunk and survives a missing 2D context', () => {
    const { container } = render(<TerrainLayer seed={777} {...VIEW} isVillage={() => false} revision="0" />);
    expect(container.querySelector('[data-terrain-layer]')).not.toBeNull();
    expect(container.querySelectorAll('canvas[data-chunk]').length).toBe(chunksInView(VIEW).length);
  });
});

describe('drawing a chunk', () => {
  const atlasCells = (calls: Call[]) => calls.filter((c) => c[0] === 'drawImage' && c.length === 10);

  it('draws every Wang cell of a chunk inside the world, then blits it once', () => {
    const target = document.createElement('canvas');
    drawChunk(target, SHEET, 777, 0, 0, 200, () => false);
    const blit = lastDraw(target).filter((c) => c[0] === 'drawImage');
    expect(blit).toHaveLength(1);
    const art = lastDraw(blit[0]![1] as HTMLCanvasElement);
    expect(art.find((c) => c[0] === 'rect')).toEqual(['rect', 0, 0, 768, 512]);
    expect(atlasCells(art)).toHaveLength(24 * 16);
  });

  it('meadow decals lie under every tree, peak and decoration', () => {
    const target = document.createElement('canvas');
    drawChunk(target, SHEET, 777, 3, 3, 200, () => false);
    const art = lastDraw(lastDraw(target).find((c) => c[0] === 'drawImage')![1] as HTMLCanvasElement);
    const flat = new Set<unknown>([...Object.values(SHEET.decals).flat()]);
    const upright = new Set<unknown>([...Object.values(SHEET.trees), ...SHEET.peaks, ...SHEET.decor]);
    const order = art.filter((c) => c[0] === 'drawImage').map((c) => c[1]);
    const lastFlat = order.map((o) => flat.has(o)).lastIndexOf(true);
    const firstUpright = order.findIndex((o) => upright.has(o));
    expect(lastFlat).toBeGreaterThan(-1);
    expect(firstUpright).toBeGreaterThan(lastFlat);
  });

  it('clips the last chunk at the world edge', () => {
    const target = document.createElement('canvas');
    drawChunk(target, SHEET, 777, 24, 24, 196, () => false);
    const art = lastDraw(lastDraw(target).find((c) => c[0] === 'drawImage')![1] as HTMLCanvasElement);
    expect(art.find((c) => c[0] === 'rect')).toEqual(['rect', 0, 0, 4 * 96, 4 * 64]);
    expect(atlasCells(art)).toHaveLength(4 * 3 * 4 * 2);
  });

  it('upscaled chunks are drawn without smoothing, downscaled ones with it', () => {
    const near = document.createElement('canvas');
    near.width = CHUNK * 133; near.height = CHUNK * 95;
    drawChunk(near, SHEET, 777, 0, 0, 200, () => false);
    expect((near.getContext('2d') as unknown as { imageSmoothingEnabled: boolean }).imageSmoothingEnabled).toBe(false);
    const far = document.createElement('canvas');
    far.width = CHUNK * 53; far.height = CHUNK * 38;
    drawChunk(far, SHEET, 777, 0, 0, 200, () => false);
    expect((far.getContext('2d') as unknown as { imageSmoothingEnabled: boolean }).imageSmoothingEnabled).toBe(true);
  });

  it('a missing 2D context draws nothing and does not throw', () => {
    const target = { getContext: () => null } as unknown as HTMLCanvasElement;
    expect(() => drawChunk(target, SHEET, 777, 0, 0, 200, () => false)).not.toThrow();
  });

  it('the paint of a chunk is computed once per village set', () => {
    const a = chunkPaint(777, 3, 4, 200, () => false, '');
    expect(chunkPaint(777, 3, 4, 200, () => false, '')).toBe(a);
    expect(chunkPaint(777, 3, 4, 200, (x, y) => x === 27 && y === 35, '27|35')).not.toBe(a);
  });

  it('a chunk redraws when a village appears in its reach, not on every revision', async () => {
    const draws = (c: Element) => recordingContext(c as HTMLCanvasElement).log.filter((l) => l[0] === 'clearRect').length;
    const props = { seed: 777, ...VIEW, isVillage: () => false, revision: '0' };
    const { container, rerender } = render(<TerrainLayer {...props} />);
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
    const chunk = container.querySelector('canvas[data-chunk="0|0"]')!;
    const first = draws(chunk);
    expect(first).toBeGreaterThan(0);
    rerender(<TerrainLayer {...props} revision="1" />);
    expect(draws(chunk)).toBe(first);
    rerender(<TerrainLayer {...props} revision="2" isVillage={(x, y) => x === 3 && y === 3} />);
    expect(draws(chunk)).toBe(first + 1);
  });
});

describe('map view terrain', () => {
  const game = newDefaultGame();
  const showMap = () => {
    useGameStore.setState({ game: structuredClone(game), phase: 'ready', devMode: false, notice: null, lastError: null });
    return render(<MapView />).container;
  };

  it('the map draws the pixel terrain layer under the tiles', () => {
    const map = showMap();
    const layer = map.querySelector('[data-terrain-layer]');
    expect(layer).not.toBeNull();
    expect(layer!.querySelectorAll('canvas[data-chunk]').length).toBeGreaterThan(0);
  });

  it('an empty tile is labelled with its biome', () => {
    const map = showMap();
    const LABEL = { [BIOME.grass]: 'łąka', [BIOME.water]: 'jezioro', [BIOME.forest]: 'las', [BIOME.mountain]: 'góry' };
    const tiles = [...map.querySelectorAll('[data-tile][title]')];
    expect(tiles.length).toBeGreaterThan(0);
    for (const t of tiles) {
      const [x, y] = t.getAttribute('data-tile')!.split('|').map(Number);
      expect(t.getAttribute('title')).toBe(`${formatCoords(x!, y!)} — ${LABEL[tileBiome(game.worldSeed, x!, y!)]}`);
    }
  });

  it('the map has the pixel minimap', () => {
    expect(showMap().querySelector('svg[data-minimap]')).not.toBeNull();
  });
});
