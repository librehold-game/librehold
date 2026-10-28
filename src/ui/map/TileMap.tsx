import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { MAP } from '../../data/world.config';
import type { TileInfo } from '../../engine/world';
import { t as tr } from '../../i18n/i18n';
import type { AnimationMode } from '../../state/uiStore';
import { formatCoords } from '../format';
import type { MapArmy } from './armies';
import type { ArmyClock } from './armyClock';
import { ArmyLayerView } from './ArmyLayerView';
import { BIOME, tileBiome, type Biome } from './biomes';
import { MAP_GROUND, TileArt, villageSize, villageTint, type TileArtProps } from './MapArt';
import { MINIMAP_PALETTE, minimapImage } from './minimap';
import { TerrainLayer } from './TerrainLayer';
import { FRAME_H, FRAME_W, MAX_H, MAX_W, RULER_H, RULER_W, mapViewport, pinchZoomDelta, touchDistance, type MapHover } from './viewport';

export type Dot = NonNullable<TileArtProps['dot']>;

/** Where the map stands now: what the overlays need to place themselves. */
export interface TileMapView {
  x0: number; y0: number; offX: number; offY: number; tw: number; th: number; width: number; height: number;
  /** The map and its side column fit side by side; otherwise cards come up as sheets below. */
  wide: boolean;
}

/**
 * The tile map, whatever feeds it: the player's world or the host's god view. It draws the terrain, a tile per field,
 * the grid and rulers, the armies and a minimap, and lets the map be dragged and zoomed; what a click opens is the
 * caller's.
 */
export interface TileMapProps {
  mapSize: number;
  seed: number;
  /** Changes when villages change, so the terrain around them is drawn again. */
  revision: string;
  /** A hook: the tile at (x, y), stable between renders while it does not change. */
  useTile: (x: number, y: number) => TileInfo;
  isVillage: (x: number, y: number) => boolean;
  dots?: Record<string, Dot>;
  activeId?: string;
  minimapDots: { x: number; y: number; player: boolean }[];
  armies: MapArmy[];
  clock: ArmyClock;
  animations: AnimationMode;
  center: { x: number; y: number };
  onCenter: (c: { x: number; y: number }) => void;
  zoomIdx: number;
  onZoom: (delta: number) => void;
  colors: boolean;
  maxPoints: number;
  selected: { x: number; y: number } | null;
  onSelectTile: (x: number, y: number, village: boolean) => void;
  onSelectArmy: (id: string) => void;
  /** The fields in view and around them; the caller loads what it has not got, and may return a cancel for when the
   * view moves on. */
  onRegion?: (x0: number, y0: number, x1: number, y1: number) => (() => void) | void;
  onHover?: (h: MapHover | null) => void;
  /** Drawn over the map surface (popups, hover cards). */
  surface?: (view: TileMapView) => ReactNode;
  /** Drawn beside or below the map (sheets on a narrow screen). */
  outside?: (view: TileMapView) => ReactNode;
  /** Under the minimap. */
  side?: ReactNode;
}

const BIOME_KEY: Record<Biome, string> = { [BIOME.grass]: 'grass', [BIOME.water]: 'water', [BIOME.forest]: 'forest', [BIOME.mountain]: 'mountain' };

function Tile({ x, y, tw, th, left, top, selected, maxPoints, colors, seed, activeId, dot, useTile, onSelect }: {
  x: number; y: number; tw: number; th: number; left: number; top: number; selected: boolean; maxPoints: number; colors: boolean;
  seed: number; activeId?: string; dot?: Dot; useTile: TileMapProps['useTile'];
  onSelect: (x: number, y: number, village: boolean) => void;
}) {
  const t = useTile(x, y);
  const ownerColor = colors ? villageTint(t) : undefined;
  const hidden = t.kind === 'village' && t.ownerId === null && t.points > maxPoints;
  const village: TileArtProps['village'] = t.kind === 'village'
    ? { owner: t.ownerId === null ? 'barbarian' : 'player', size: villageSize(t.points), bonus: !!t.bonus, active: t.id === activeId, hidden, ownerColor }
    : undefined;
  const label = tr(`biome.${BIOME_KEY[tileBiome(seed, x, y)]}`);
  // A village is described by the hover card (`MapHoverCard`); an empty field gets a plain tooltip.
  const title = t.kind === 'empty' ? `${formatCoords(x, y)} — ${label}` : undefined;
  return (
    <div
      className={`absolute box-border select-none ${selected ? 'ring-2 ring-bad z-10' : ''}`}
      style={{ left, top, width: tw, height: th, cursor: 'pointer' }}
      title={title}
      data-tile={`${x}|${y}`}
      data-tile-village={t.kind === 'village' ? t.id : undefined}
      onClick={() => onSelect(x, y, t.kind === 'village')}
    >
      <TileArt village={village} dot={dot} tw={tw} th={th} />
    </div>
  );
}

function Minimap({ seed, mapSize, dots, center, cols, rows, onJump }: {
  seed: number; mapSize: number; dots: TileMapProps['minimapDots']; center: { x: number; y: number }; cols: number; rows: number;
  onJump: (x: number, y: number) => void;
}) {
  const [image, setImage] = useState<string | null>(null);
  useEffect(() => {
    const id = setTimeout(() => setImage(minimapImage(seed, mapSize)), 0);
    return () => clearTimeout(id);
  }, [seed, mapSize]);
  const ground: React.CSSProperties = {
    backgroundColor: MINIMAP_PALETTE.meadow, backgroundImage: image ? `url(${image})` : undefined, backgroundSize: '100% 100%', imageRendering: 'pixelated',
  };
  const k = 200 / mapSize;
  return (
    <svg
      width={200} height={200} className="frame-wood cursor-crosshair shrink-0" style={ground} data-minimap
      onClick={(e) => {
        const r = e.currentTarget.getBoundingClientRect();
        onJump(Math.floor((e.clientX - r.left) / k), Math.floor((e.clientY - r.top) / k));
      }}
    >
      {dots.filter((d) => !d.player).map((d) => <rect key={`b${d.x},${d.y}`} x={d.x * k - 1} y={d.y * k - 1} width={2} height={2} fill="#5c4528" />)}
      {dots.filter((d) => d.player).map((d) => <rect key={`p${d.x},${d.y}`} x={d.x * k - 2} y={d.y * k - 2} width={4} height={4} fill="#2f5fa8" stroke="#fff" strokeWidth={0.5} />)}
      <rect x={(center.x - cols / 2) * k} y={(center.y - rows / 2) * k} width={cols * k} height={rows * k} fill="none" stroke="#a33a2a" strokeWidth={1} />
    </svg>
  );
}

/** The minimap and the side panel fit beside the map only from this width on. */
const SIDE_COL = 212;
/** A finger trembles more than a mouse: below this a touch is still a click, not a drag. */
const TAP_SLOP = 8;

export function TileMap(p: TileMapProps) {
  const { mapSize, seed, center, onCenter, zoomIdx, onZoom, onRegion, onHover } = p;
  const [avail, setAvail] = useState({ w: MAX_W + FRAME_W, h: MAX_H + FRAME_H, wide: true });
  const wrapRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ sx: number; sy: number; cx: number; cy: number; moved: boolean } | null>(null);
  const pointers = useRef(new Map<number, { x: number; y: number }>());
  const pinchBase = useRef<number | null>(null);
  const surfaceRef = useRef<HTMLDivElement>(null);

  // touch-action alone does not stop iOS Safari from scrolling the page under a drag.
  useEffect(() => {
    const el = surfaceRef.current;
    if (!el) return;
    const block = (e: TouchEvent) => e.preventDefault();
    el.addEventListener('touchmove', block, { passive: false });
    return () => el.removeEventListener('touchmove', block);
  }, []);

  useLayoutEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const measure = () => {
      const room = el.clientWidth || MAX_W + FRAME_W;
      const wide = room >= MAX_W + FRAME_W + SIDE_COL;
      const vh = typeof window === 'undefined' ? MAX_H + FRAME_H : window.innerHeight;
      setAvail({
        w: wide ? MAX_W + FRAME_W : room,
        h: wide ? MAX_H + FRAME_H : Math.round(vh * 0.56),
        wide,
      });
    };
    measure();
    const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null;
    ro?.observe(el);
    window.addEventListener('resize', measure);
    return () => { ro?.disconnect(); window.removeEventListener('resize', measure); };
  }, []);

  const { tw, th, width, height, cols, rows } = mapViewport(avail.w, avail.h, zoomIdx);
  const x0 = Math.floor(center.x - width / tw / 2);
  const y0 = Math.floor(center.y - height / th / 2);
  const offX = (center.x - width / tw / 2 - x0) * tw;
  const offY = (center.y - height / th / 2 - y0) * th;
  const view: TileMapView = { x0, y0, offX, offY, tw, th, width, height, wide: avail.wide };

  // The fields in view and 10 more around them, so dragging never uncovers fields not loaded yet.
  useEffect(() => onRegion?.(x0 - 10, y0 - 10, x0 + cols + 10, y0 + rows + 10), [x0, y0, cols, rows, onRegion]);

  const clampCenter = (c: { x: number; y: number }) => ({
    x: Math.max(0.5, Math.min(mapSize - 0.5, c.x)),
    y: Math.max(0.5, Math.min(mapSize - 0.5, c.y)),
  });

  /** The field under the mouse; the hover card goes while dragging, on touch and over the selected village. */
  const trackHover = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!onHover) return;
    if (e.pointerType !== 'mouse' || drag.current?.moved) { onHover(null); return; }
    const rect = e.currentTarget.getBoundingClientRect();
    const px = e.clientX - rect.left - e.currentTarget.clientLeft, py = e.clientY - rect.top - e.currentTarget.clientTop;
    const x = x0 + Math.floor((px + offX) / tw), y = y0 + Math.floor((py + offY) / th);
    const left = Math.round((x - x0) * tw - offX), top = Math.round((y - y0) * th - offY);
    onHover(p.selected?.x === x && p.selected?.y === y ? null : { x, y, left, top, tw, th });
  };

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    onHover?.(null);
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    const pts = [...pointers.current.values()];
    if (pts.length >= 2) {
      pinchBase.current = touchDistance(pts[0]!, pts[1]!);
      drag.current = null;
      return;
    }
    drag.current = { sx: e.clientX, sy: e.clientY, cx: center.x, cy: center.y, moved: false };
    e.currentTarget.setPointerCapture?.(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    trackHover(e);
    if (pointers.current.has(e.pointerId)) pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    const pts = [...pointers.current.values()];
    if (pts.length >= 2 && pinchBase.current !== null) {
      const d = touchDistance(pts[0]!, pts[1]!);
      const step = pinchZoomDelta(pinchBase.current, d);
      if (step !== 0) { onZoom(step); pinchBase.current = d; }
      return;
    }
    const d = drag.current;
    if (!d) return;
    const dx = e.clientX - d.sx, dy = e.clientY - d.sy;
    if (Math.abs(dx) + Math.abs(dy) > TAP_SLOP) d.moved = true;
    if (d.moved) onCenter(clampCenter({ x: d.cx - dx / tw, y: d.cy - dy / th }));
  };

  const onPointerEnd = (e: React.PointerEvent<HTMLDivElement>) => {
    pointers.current.delete(e.pointerId);
    if (pointers.current.size < 2) pinchBase.current = null;
    window.setTimeout(() => { drag.current = null; }, 0);
  };

  const tiles: React.ReactNode[] = [];
  for (let i = 0; i < cols; i++) {
    for (let j = 0; j < rows; j++) {
      const x = x0 + i, y = y0 + j;
      if (x < 0 || y < 0 || x >= mapSize || y >= mapSize) continue;
      tiles.push(
        <Tile
          key={`${x},${y}`} x={x} y={y} tw={tw} th={th}
          left={Math.round(i * tw - offX)} top={Math.round(j * th - offY)}
          selected={p.selected?.x === x && p.selected?.y === y} maxPoints={p.maxPoints} colors={p.colors}
          seed={seed} activeId={p.activeId} dot={p.dots?.[`v:${x}:${y}`]} useTile={p.useTile}
          onSelect={(sx, sy, village) => { if (!drag.current?.moved) p.onSelectTile(sx, sy, village); }}
        />,
      );
    }
  }

  const rulerX: React.ReactNode[] = [];
  const grid: React.ReactNode[] = [];
  for (let i = 0; i < cols; i++) {
    const x = x0 + i;
    if (x < 0 || x >= mapSize) continue;
    const left = Math.round(i * tw - offX);
    rulerX.push(<span key={x} className="absolute text-[10px] font-semibold tabular-nums text-center" style={{ left, top: 1, width: tw }}>{x}</span>);
    if (x % 5 === 0) grid.push(<div key={`gx${x}`} className="absolute top-0 bottom-0 pointer-events-none" style={{ left, width: 1, background: 'rgba(20,40,10,0.45)' }} />);
  }
  const rulerY: React.ReactNode[] = [];
  for (let j = 0; j < rows; j++) {
    const y = y0 + j;
    if (y < 0 || y >= mapSize) continue;
    const top = Math.round(j * th - offY);
    rulerY.push(<span key={y} className="absolute text-[10px] font-semibold tabular-nums text-right" style={{ top: top + Math.round(th / 2) - 7, left: 0, width: 24 }}>{y}</span>);
    if (y % 5 === 0) grid.push(<div key={`gy${y}`} className="absolute left-0 right-0 pointer-events-none" style={{ top, height: 1, background: 'rgba(20,40,10,0.45)' }} />);
  }

  return (
    <div ref={wrapRef} className={`flex gap-3 items-start ${avail.wide ? 'flex-row' : 'flex-col'}`}>
      <div className="relative frame-wood shrink-0 self-center lg:self-start" style={{ width: width + FRAME_W, height: height + FRAME_H }}>
        <div className="absolute left-7 overflow-hidden wood-strip border-2 border-t-0 border-border-dark" style={{ width, height: RULER_H, top: height + 2 }}>{rulerX}</div>
        <div className="absolute left-0 top-0 overflow-hidden wood-strip map-ruler-y border-2 border-r-0 border-border-dark" style={{ width: RULER_W, height }}>{rulerY}</div>
        <div
          ref={surfaceRef}
          className="absolute left-7 top-0 overflow-hidden border-2 border-wood-border cursor-grab active:cursor-grabbing"
          style={{ width, height, touchAction: 'none', ...MAP_GROUND }}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerEnd}
          onPointerCancel={onPointerEnd}
          onPointerLeave={() => onHover?.(null)}
          data-map-surface
          onWheel={(e) => onZoom(e.deltaY > 0 ? 1 : -1)}
        >
          <TerrainLayer
            seed={seed} x0={x0} y0={y0} cols={cols} rows={rows} offX={offX} offY={offY} tw={tw} th={th} mapSize={mapSize}
            isVillage={p.isVillage} revision={p.revision}
          />
          {tiles}
          {grid}
          <ArmyLayerView
            armies={p.armies} seed={seed} isVillage={p.isVillage} mode={p.animations} clock={p.clock}
            x0={x0} y0={y0} offX={offX} offY={offY} tw={tw} th={th} width={width} height={height}
            onSelect={p.onSelectArmy} isDragging={() => !!drag.current?.moved}
          />
          {p.surface?.(view)}
        </div>
      </div>
      {p.outside?.(view)}
      <div className={`flex gap-3 ${avail.wide ? 'flex-col' : 'flex-row flex-wrap w-full justify-center'}`}>
        <Minimap seed={seed} mapSize={mapSize} dots={p.minimapDots} center={center} cols={width / tw} rows={height / th}
          onJump={(x, y) => onCenter(clampCenter({ x: x + 0.5, y: y + 0.5 }))} />
        {p.side}
      </div>
    </div>
  );
}

/** How far the zoom may go, for the callers' zoom buttons. */
export const ZOOM_MAX = MAP.zoomLevels.length - 1;
