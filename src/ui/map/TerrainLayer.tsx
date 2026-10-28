import { useEffect, useMemo, useRef, useState } from 'react';
import { biomeAt, biomeGrid, SUB_X, SUB_Y } from './biomes';
import { CELL, DECALS, DECOR, groundPlacements, spritePlacements, TREES, wangCell, type Decal, type SpritePlacement, type WangSet } from './terrainPaint';

/**
 * Pixel terrain of the classic skin's map, drawn under the village tiles. The map is cut into CHUNK × CHUNK-tile
 * chunks, each its own canvas, so panning only draws the chunks that scroll into view. A chunk is composed at art
 * resolution (pixel-exact Wang cells and sprites), then scaled down once onto its canvas, like the approved preview.
 */
export const CHUNK = 8;

/** `visible` is false for the ring around the view, which is drawn when the browser is idle. */
export interface ChunkPos { cx: number; cy: number; left: number; top: number; width: number; height: number; visible: boolean }

/** Chunks over the visible tiles plus one ring around them, so a pan reveals chunks that are already drawn. */
export function chunksInView(v: { x0: number; y0: number; cols: number; rows: number; offX: number; offY: number; tw: number; th: number; mapSize: number }): ChunkPos[] {
  const lo = (start: number) => Math.max(0, start);
  const hi = (start: number, n: number) => Math.min(v.mapSize, start + n);
  if (lo(v.x0) >= hi(v.x0, v.cols) || lo(v.y0) >= hi(v.y0, v.rows)) return [];
  const last = Math.ceil(v.mapSize / CHUNK) - 1;
  const range = (start: number, n: number) => [Math.floor(lo(start) / CHUNK), Math.floor((hi(start, n) - 1) / CHUNK)] as const;
  const [vx0, vx1] = range(v.x0, v.cols), [vy0, vy1] = range(v.y0, v.rows);
  const cx0 = Math.max(0, vx0 - 1), cx1 = Math.min(last, vx1 + 1), cy0 = Math.max(0, vy0 - 1), cy1 = Math.min(last, vy1 + 1);
  const out: ChunkPos[] = [];
  for (let cy = cy0; cy <= cy1; cy++) {
    for (let cx = cx0; cx <= cx1; cx++) {
      out.push({
        cx, cy,
        left: Math.round((cx * CHUNK - v.x0) * v.tw - v.offX),
        top: Math.round((cy * CHUNK - v.y0) * v.th - v.offY),
        width: CHUNK * v.tw,
        height: CHUNK * v.th,
        visible: cx >= vx0 && cx <= vx1 && cy >= vy0 && cy <= vy1,
      });
    }
  }
  return out;
}

const URLS = import.meta.glob<string>('../../assets/pixel/map/*.png', { eager: true, query: '?url', import: 'default' });

export interface TerrainSheets {
  wang: Record<WangSet, HTMLImageElement>;
  trees: Record<(typeof TREES)[number], HTMLImageElement>;
  peaks: HTMLImageElement[];
  decals: Record<Decal['kind'], HTMLImageElement[]>;
  decor: HTMLImageElement[];
}

let sheets: Promise<TerrainSheets | null> | null = null;

function loadImage(name: string): Promise<HTMLImageElement> {
  const url = URLS[`../../assets/pixel/map/${name}.png`];
  if (!url) return Promise.reject(new Error(`Missing pixel map sprite: ${name}`));
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = url;
  });
}

function loadSheets(): Promise<TerrainSheets | null> {
  sheets ??= Promise.all([
    Promise.all((['water', 'forest', 'mountain'] as const).map((s) => loadImage(`wang-${s}`))),
    Promise.all((['oak', 'pine'] as const).map((t) => loadImage(`tree-${t}`))),
    Promise.all(Array.from({ length: 7 }, (_, i) => loadImage(`peak-${i}`))),
    Promise.all(Array.from({ length: DECALS.dry }, (_, i) => loadImage(`decal-dry-${i}`))),
    Promise.all(Array.from({ length: DECALS.flower }, (_, i) => loadImage(`flower-${i}`))),
    Promise.all(Array.from({ length: DECALS.dirt }, (_, i) => loadImage(`decal-dirt-${i}`))),
    Promise.all(Array.from({ length: DECOR }, (_, i) => loadImage(`decor-${i}`))),
  ]).then(([[water, forest, mountain], [oak, pine], peaks, dry, flower, dirt, decor]) => ({
    wang: { water: water!, forest: forest!, mountain: mountain! }, trees: { oak: oak!, pine: pine! }, peaks, decals: { dry, flower, dirt }, decor,
  }))
    .catch(() => null);
  return sheets;
}

/** Sprites and decals reach this many vertices beyond the vertex they stand on (a peak is 74 art px, a decal 64). */
const MARGIN = 2;
/** Villages this many tiles outside a chunk still clear vertices its biome grid reads (MARGIN + 1 for the meadow
 * neighbourhood, plus the grid's padding). */
const VILLAGE_REACH = 5;

export interface ChunkPaint { cols: number; rows: number; cells: { set: WangSet; mask: number }[]; decals: Decal[]; sprites: SpritePlacement[] }

const PAINT_CACHE_SIZE = 96;
const paintCache = new Map<string, ChunkPaint>();

/**
 * What a chunk draws, in art px from its top-left corner. Cached per village set (`villageKey`), so zooming and
 * panning back to a chunk skip the noise.
 */
export function chunkPaint(seed: number, cx: number, cy: number, mapSize: number, isVillage: (x: number, y: number) => boolean, villageKey: string): ChunkPaint {
  const key = `${seed}|${cx}|${cy}|${mapSize}|${villageKey}`;
  const hit = paintCache.get(key);
  if (hit) {
    paintCache.delete(key);
    paintCache.set(key, hit);
    return hit;
  }
  const tx0 = cx * CHUNK, ty0 = cy * CHUNK;
  const cols = Math.min(CHUNK, mapSize - tx0), rows = Math.min(CHUNK, mapSize - ty0);
  const vx0 = tx0 * SUB_X, vy0 = ty0 * SUB_Y;
  const g = biomeGrid(seed, vx0 - MARGIN - 1, vy0 - MARGIN - 1, CHUNK * SUB_X + 3 + 2 * MARGIN, CHUNK * SUB_Y + 3 + 2 * MARGIN, isVillage);
  const cells: ChunkPaint['cells'] = [];
  for (let vy = vy0; vy < vy0 + rows * SUB_Y; vy++) {
    for (let vx = vx0; vx < vx0 + cols * SUB_X; vx++) {
      const { set, mask } = wangCell(biomeAt(g, vx, vy), biomeAt(g, vx + 1, vy), biomeAt(g, vx, vy + 1), biomeAt(g, vx + 1, vy + 1));
      cells.push({ set: set ?? 'water', mask });
    }
  }
  const local = <T extends { x: number; y: number }>(p: T): T => ({ ...p, x: p.x - vx0 * CELL, y: p.y - vy0 * CELL });
  const inner = [g.vx0 + 1, g.vy0 + 1, g.w - 2, g.h - 2] as const;
  const ground = groundPlacements(seed, g, ...inner);
  const sprites = [...spritePlacements(seed, g, ...inner), ...ground.decor].sort((p, q) => p.y - q.y || p.x - q.x).map(local);
  const paint = { cols, rows, cells, decals: ground.decals.map(local), sprites };
  paintCache.set(key, paint);
  if (paintCache.size > PAINT_CACHE_SIZE) paintCache.delete(paintCache.keys().next().value!);
  return paint;
}

let artCanvas: HTMLCanvasElement | null = null;

/** Ring chunks wait this long, so a zoom or a pan first paints what is on screen. */
const IDLE_DRAW_MS = 60;

export function drawChunk(canvas: HTMLCanvasElement, s: TerrainSheets, seed: number, cx: number, cy: number, mapSize: number, isVillage: (x: number, y: number) => boolean, villageKey = '') {
  const ctx = canvas.getContext('2d');
  if (!ctx) return;
  artCanvas ??= document.createElement('canvas');
  const art = artCanvas;
  const actx = art.getContext('2d');
  if (!actx) return;
  const { cols, rows, cells, decals, sprites } = chunkPaint(seed, cx, cy, mapSize, isVillage, villageKey);
  const w = CHUNK * SUB_X * CELL, h = CHUNK * SUB_Y * CELL;
  if (art.width !== w) art.width = w;
  if (art.height !== h) art.height = h;
  actx.save();
  actx.clearRect(0, 0, w, h);
  actx.imageSmoothingEnabled = false;
  actx.beginPath();
  actx.rect(0, 0, cols * SUB_X * CELL, rows * SUB_Y * CELL);
  actx.clip();
  const perRow = cols * SUB_X;
  cells.forEach(({ set, mask }, i) => {
    actx.drawImage(s.wang[set], (mask % 4) * CELL, Math.floor(mask / 4) * CELL, CELL, CELL, (i % perRow) * CELL, Math.floor(i / perRow) * CELL, CELL, CELL);
  });
  for (const d of decals) {
    const img = s.decals[d.kind][d.index]!;
    actx.save();
    actx.translate(Math.round(d.x), Math.round(d.y));
    if (d.flip) actx.scale(-1, 1);
    actx.drawImage(img, -Math.round(img.width / 2), -Math.round(img.height / 2));
    actx.restore();
  }
  for (const p of sprites) {
    const img = p.kind === 'peak' ? s.peaks[p.index]! : p.kind === 'decor' ? s.decor[p.index]! : s.trees[TREES[p.index]!];
    actx.drawImage(img, Math.round(p.x - img.width / 2), Math.round(p.y - img.height + img.height / 5));
  }
  actx.restore();

  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  // Enlarged pixel art stays crisp; shrunk art is smoothed so it does not shimmer.
  ctx.imageSmoothingEnabled = canvas.width < w;
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(art, 0, 0, canvas.width, canvas.height);
}

function Chunk({ pos, seed, mapSize, isVillage, revision, sheet }: {
  pos: ChunkPos; seed: number; mapSize: number; isVillage: (x: number, y: number) => boolean; revision: string; sheet: TerrainSheets | null;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  const { cx, cy, width, height } = pos;
  const villageKey = useMemo(() => {
    const found: string[] = [];
    for (let y = cy * CHUNK - VILLAGE_REACH; y < (cy + 1) * CHUNK + VILLAGE_REACH; y++) {
      for (let x = cx * CHUNK - VILLAGE_REACH; x < (cx + 1) * CHUNK + VILLAGE_REACH; x++) if (isVillage(x, y)) found.push(`${x}|${y}`);
    }
    return found.join(';');
  }, [revision, cx, cy]);
  const dpr = typeof window === 'undefined' ? 1 : window.devicePixelRatio || 1;
  const visible = pos.visible;
  const drawn = useRef('');
  useEffect(() => {
    const canvas = ref.current;
    const state = `${seed}|${mapSize}|${width}|${height}|${dpr}|${villageKey}`;
    if (!sheet || !canvas || drawn.current === state) return;
    const draw = () => {
      drawChunk(canvas, sheet, seed, cx, cy, mapSize, isVillage, villageKey);
      drawn.current = state;
    };
    if (visible) { draw(); return; }
    const id = setTimeout(draw, IDLE_DRAW_MS);
    return () => clearTimeout(id);
  }, [sheet, seed, cx, cy, mapSize, width, height, dpr, villageKey, visible]);
  return (
    <canvas
      ref={ref}
      data-chunk={`${cx}|${cy}`}
      width={Math.round(width * dpr)}
      height={Math.round(height * dpr)}
      className="absolute"
      style={{ left: pos.left, top: pos.top, width, height }}
    />
  );
}

export function TerrainLayer({ seed, isVillage, revision, ...view }: {
  seed: number; x0: number; y0: number; cols: number; rows: number; offX: number; offY: number; tw: number; th: number;
  mapSize: number; isVillage: (x: number, y: number) => boolean; revision: string;
}) {
  const [sheet, setSheet] = useState<TerrainSheets | null>(null);
  useEffect(() => {
    let live = true;
    void loadSheets().then((s) => { if (live) setSheet(s); });
    return () => { live = false; };
  }, []);
  return (
    <div data-terrain-layer className="absolute inset-0 pointer-events-none" aria-hidden>
      {chunksInView(view).map((pos) => (
        <Chunk key={`${pos.cx}|${pos.cy}|${view.tw}|${view.th}`} pos={pos} seed={seed} mapSize={view.mapSize} isVillage={isVillage} revision={revision} sheet={sheet} />
      ))}
    </div>
  );
}
