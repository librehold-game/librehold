import { minimapPixels } from '../../shared/terrain/minimap';
import { ONLINE_TERRAIN_SEED } from '../../shared/terrain/seed';
import { formatDuration } from '../../i18n/format';
import type { Frame, Timelapse } from './model';

export interface MapLayout { x: number; y: number; size: number }
/** What the video's title band says: the game's name and "Won by …", worded. */
export interface VideoInfo { title: string; winner: string }

export const VIDEO_SIZE = 1080;
const BAND = 120;
const VIDEO_MAP: MapLayout = { x: BAND / 2, y: BAND, size: VIDEO_SIZE - BAND };
const INK = '#fbe7b5';
const PANEL = '#2a1a0b';
const FONT = "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif";

const BARBARIAN = '#8c8c8c';
const FLASH = '#ffe9a8';
const BATTLE = '#e8402a';
/** The battle size class (1..5) is scaled to the line width against a 540 px map. */
const LINE_SCALE = 540;

const terrains = new Map<number, HTMLCanvasElement | null>();

/** The online terrain at one pixel per field (at most 200 a side), cached per map size; null without a 2D context. */
export function terrainCanvas(mapSize: number): HTMLCanvasElement | null {
  if (terrains.has(mapSize)) return terrains.get(mapSize)!;
  const { size, data } = minimapPixels(ONLINE_TERRAIN_SEED, mapSize);
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  const out = ctx && typeof ImageData !== 'undefined' ? canvas : null;
  if (out) ctx!.putImageData(new ImageData(data, size, size), 0, 0);
  terrains.set(mapSize, out);
  return out;
}

/** The terrain and one square per village, its owner's colour or grey for barbarians. */
export function drawMap(
  ctx: CanvasRenderingContext2D, mapSize: number, terrain: CanvasImageSource | null, layout: MapLayout,
  villages: Frame['villages'], colorOf: (owner: number) => string,
): void {
  const cell = layout.size / mapSize;
  const dot = Math.max(1, cell);
  ctx.imageSmoothingEnabled = false;
  ctx.clearRect(layout.x, layout.y, layout.size, layout.size);
  if (terrain) ctx.drawImage(terrain, layout.x, layout.y, layout.size, layout.size);
  for (const v of villages) {
    ctx.fillStyle = v.owner === 0 ? BARBARIAN : colorOf(v.owner);
    ctx.fillRect(layout.x + v.x * cell, layout.y + v.y * cell, dot, dot);
  }
}

/** One moment of the replay: the map, a ring widening over each fresh conquest, a fading line per battle. */
export function drawFrame(ctx: CanvasRenderingContext2D, tl: Timelapse, frame: Frame, layout: MapLayout, terrain: CanvasImageSource | null): void {
  const cell = layout.size / tl.mapSize;
  const centre = (n: number, origin: number) => origin + (n + 0.5) * cell;
  drawMap(ctx, tl.mapSize, terrain, layout, frame.villages, (owner) => tl.owners[owner - 1]?.color ?? BARBARIAN);
  ctx.strokeStyle = FLASH;
  for (const f of frame.flashes) {
    ctx.globalAlpha = 1 - f.age;
    ctx.lineWidth = Math.max(1, cell / 2);
    ctx.beginPath();
    ctx.arc(centre(f.x, layout.x), centre(f.y, layout.y), Math.max(2, cell) * (1 + 3 * f.age), 0, 2 * Math.PI);
    ctx.stroke();
  }
  ctx.strokeStyle = BATTLE;
  for (const l of frame.lines) {
    ctx.globalAlpha = 1 - l.age;
    ctx.lineWidth = (l.size * layout.size) / LINE_SCALE;
    ctx.beginPath();
    ctx.moveTo(centre(l.fx, layout.x), centre(l.fy, layout.y));
    ctx.lineTo(centre(l.x, layout.x), centre(l.y, layout.y));
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}

/** A square video frame: the title band with the winner, the clock top right, the map below and the legend over its corner. */
export function drawVideoFrame(
  ctx: CanvasRenderingContext2D, tl: Timelapse, frame: Frame, info: VideoInfo, legendNames: (owner: number) => string,
): void {
  ctx.fillStyle = PANEL;
  ctx.fillRect(0, 0, VIDEO_SIZE, VIDEO_SIZE);
  drawFrame(ctx, tl, frame, VIDEO_MAP, terrainCanvas(tl.mapSize));
  ctx.fillStyle = INK;
  ctx.textBaseline = 'middle';
  ctx.textAlign = 'left';
  ctx.font = `48px ${FONT}`;
  ctx.fillText(info.title, 40, 44);
  ctx.font = `30px ${FONT}`;
  ctx.fillText(info.winner, 40, 92);
  ctx.textAlign = 'right';
  ctx.font = `40px ${FONT}`;
  ctx.fillText(formatDuration(frame.t), VIDEO_SIZE - 40, 44);
  const row = 34;
  const left = VIDEO_MAP.x + 16;
  const top = VIDEO_SIZE - 16 - row * frame.legend.length - 16;
  if (frame.legend.length === 0) return;
  ctx.globalAlpha = 0.8;
  ctx.fillStyle = PANEL;
  ctx.fillRect(left, top, 380, row * frame.legend.length + 16);
  ctx.globalAlpha = 1;
  ctx.textAlign = 'left';
  ctx.font = `26px ${FONT}`;
  frame.legend.forEach(({ owner, villages }, i) => {
    const y = top + 8 + row * i + row / 2;
    ctx.fillStyle = tl.owners[owner - 1]?.color ?? BARBARIAN;
    ctx.fillRect(left + 12, y - 9, 18, 18);
    ctx.fillStyle = INK;
    ctx.fillText(`${legendNames(owner)} · ${villages}`, left + 42, y);
  });
}
