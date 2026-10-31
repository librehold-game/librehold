import { useEffect, useRef } from 'react';
import type { AnimationMode } from '../../state/uiStore';
import { pixelIcon } from '../pixelIcons';
import { useDocumentHidden } from '../useDocumentHidden';
import { facing } from '../scene/pixel/path';
import { CROWD, FOREIGN_KINDS, TORCH_KINDS, armyPosition, crowdOf, groundAt, leadUnit, type MapArmy } from './armies';
import { armySprite, boatSprite, type Walk } from './armySprites';
import { crowdLayout, type CrowdUnit } from './crowdLayout';
import type { ArmyClock } from './armyClock';
import type { ToPx } from './useArmyLoop';

const FRAME_MS = 180;
const PER_BOAT = 10;
/** Count labels beside the crowds nearest the middle only; more would cover the map. */
const LABELS = 4;

/** The figures an army marches with: exact for own armies, a fixed generic group for foreign ones. */
export function crowdFor(a: MapArmy, cap: number = CROWD.maxPerArmy): { unit: CrowdUnit; figures: number }[] {
  if (TORCH_KINDS.has(a.kind)) return [{ unit: 'torch', figures: CROWD.genericFigures }];
  if (FOREIGN_KINDS.has(a.kind) || !a.units) return [{ unit: 'shield', figures: CROWD.genericFigures }];
  return crowdOf(a.units, cap);
}

const images = new Map<string, HTMLImageElement>();
export function image(url: string): HTMLImageElement | null {
  if (typeof Image === 'undefined') return null;
  let img = images.get(url);
  if (!img) { img = new Image(); img.src = url; images.set(url, img); }
  return img.complete && img.naturalWidth > 0 ? img : null;
}

export function drawFigure(ctx: CanvasRenderingContext2D, s: Walk, dir: keyof Walk['file'], frame: number, x: number, y: number, scale: number): void {
  const url = s.file[dir] ?? s.file.south;
  const img = url ? image(url) : null;
  if (!img) return;
  const f = frame % s.frames;
  const k = scale * (s.scale ?? 1);
  ctx.drawImage(img, f * s.w, 0, s.w, s.h, Math.round(x - s.anchor[0] * k), Math.round(y - s.anchor[1] * k), Math.round(s.w * k), Math.round(s.h * k));
}

/** The return arrow over a foreign army heading home; nothing until the icon has loaded. */
function drawReturnBadge(ctx: CanvasRenderingContext2D, x: number, y: number, scale: number): void {
  const icon = image(pixelIcon('return'));
  if (!icon) return;
  const size = Math.round(16 * scale);
  ctx.drawImage(icon, Math.round(x - size / 2), Math.round(y - size + 4 * scale), size, size);
}

/** Stand-in boat: a brown hull with the army's icon riding it. */
export function drawBoat(ctx: CanvasRenderingContext2D, a: MapArmy, dir: keyof Walk['file'], frame: number, x: number, y: number, scale: number): void {
  const boat = boatSprite();
  if (boat) { drawFigure(ctx, boat, dir, frame, x, y, scale); return; }
  const w = 30 * scale, h = 9 * scale;
  ctx.fillStyle = '#7a4a22';
  ctx.fillRect(Math.round(x - w / 2), Math.round(y - h), Math.round(w), Math.round(h));
  const lead = a.units && !FOREIGN_KINDS.has(a.kind) ? leadUnit(a.units) : 'spear';
  const icon = image(pixelIcon(`unit-${lead}`));
  const size = Math.round(18 * scale);
  if (icon) ctx.drawImage(icon, Math.round(x - size / 2), Math.round(y - h - size + 2 * scale), size, size);
}

/** Map armies as marching crowds on one canvas, within the on-screen figure budget; the rest keep their icons. */
export function CrowdCanvas({ armies, toPx, th, width, height, seed, isVillage, mode, clock }: {
  armies: MapArmy[]; toPx: ToPx; tw: number; th: number; width: number; height: number; seed: number;
  isVillage: (x: number, y: number) => boolean; mode: AnimationMode; clock: ArmyClock;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  const state = useRef({ armies, toPx, th, width, height, seed, isVillage, mode });
  state.current = { armies, toPx, th, width, height, seed, isVillage, mode };
  const hidden = useDocumentHidden();
  const dpr = typeof window === 'undefined' ? 1 : window.devicePixelRatio || 1;

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const draw = () => {
      const { armies: as, toPx: px, th: h, width: w, height: ht, seed: sd, isVillage: iv, mode: m } = state.current;
      const ctx = canvas.getContext('2d');
      const now = clock.now();
      const frame = m === 'full' ? Math.floor((typeof performance === 'undefined' ? Date.now() : performance.now()) / FRAME_MS) : 0;
      const scale = h / 125;
      const spacing = 17 * scale;
      if (ctx) {
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, ht);
        ctx.imageSmoothingEnabled = false;
      }
      const placed = as.map((a) => {
        const pos = armyPosition(a, now);
        const [x, y] = px(pos.x, pos.y);
        return { a, pos, x, y, d: Math.hypot(x - w / 2, y - ht / 2) };
      }).filter((p) => p.x > -w / 2 && p.x < w * 1.5 && p.y > -ht / 2 && p.y < ht * 1.5).sort((p, q) => p.d - q.d);
      let figures = 0, boats = 0, budget: number = CROWD.maxOnScreen;
      const inCrowd = new Set<string>();
      const labelled = new Set<string>();
      for (const p of placed) {
        if (p.a.kind === 'trade') continue;
        const crowd = crowdFor(p.a);
        const n = crowd.reduce((s, c) => s + c.figures, 0);
        if (n === 0 || n > budget) continue;
        budget -= n;
        inCrowd.add(p.a.id);
        if (labelled.size < LABELS) labelled.add(p.a.id);
        const [fx, fy] = px(p.a.from.x, p.a.from.y), [tx, ty] = px(p.a.to.x, p.a.to.y);
        const dir = facing([fx, fy], [tx, ty]);
        if (groundAt(sd, p.pos.x, p.pos.y, iv) === 'water') {
          const count = Math.ceil(n / PER_BOAT);
          boats += count;
          const hulls = crowdLayout([{ unit: 'spear', figures: count }], { dx: tx - fx, dy: ty - fy }, 40 * scale);
          if (ctx) for (const s of hulls) drawBoat(ctx, p.a, dir, frame, p.x + s.dx, p.y + s.dy, scale);
          continue;
        }
        const spots = crowdLayout(crowd, { dx: tx - fx, dy: ty - fy }, spacing);
        figures += spots.length;
        if (!ctx) continue;
        spots.sort((s, t) => s.dy - t.dy);
        const homeward = p.a.kind === 'passingReturn';
        if (homeward) ctx.globalAlpha = 0.5;
        for (const s of spots) drawFigure(ctx, armySprite(s.unit), dir, frame + (s.dx > 0 ? 1 : 0), p.x + s.dx, p.y + s.dy, scale);
        if (homeward) {
          ctx.globalAlpha = 1;
          drawReturnBadge(ctx, p.x, p.y - 52 * scale, scale);
        }
      }
      canvas.setAttribute('data-figures', String(figures));
      canvas.setAttribute('data-boats', String(boats));
      const host = canvas.parentElement;
      for (const marker of host?.querySelectorAll<SVGGElement>('[data-army] [data-marker]') ?? []) {
        const id = marker.closest('[data-army]')!.getAttribute('data-army')!;
        const on = inCrowd.has(id);
        const counts = marker.querySelector('[data-counts]');
        const label = labelled.has(id) ? 'inline' : 'none';
        if (counts && counts.getAttribute('display') !== label) counts.setAttribute('display', label);
        if (marker.hasAttribute('data-in-crowd') === on) continue;
        if (on) marker.setAttribute('data-in-crowd', '');
        else marker.removeAttribute('data-in-crowd');
        marker.querySelector('[data-icon]')?.setAttribute('display', on ? 'none' : 'inline');
      }
    };
    draw();
    const unsubscribe = clock.subscribe(draw);
    let raf = 0;
    if (!hidden && typeof requestAnimationFrame === 'function') {
      const loop = () => { draw(); raf = requestAnimationFrame(loop); };
      raf = requestAnimationFrame(loop);
    }
    return () => {
      unsubscribe();
      if (raf) cancelAnimationFrame(raf);
      for (const marker of canvas.parentElement?.querySelectorAll<SVGGElement>('[data-in-crowd]') ?? []) {
        marker.removeAttribute('data-in-crowd');
        marker.querySelector('[data-icon]')?.setAttribute('display', 'inline');
        marker.querySelector('[data-counts]')?.setAttribute('display', 'none');
      }
    };
  }, [hidden, dpr]);

  return (
    <canvas
      ref={ref}
      data-crowd
      className="absolute inset-0 pointer-events-none"
      width={Math.round(width * dpr)}
      height={Math.round(height * dpr)}
      style={{ width, height }}
    />
  );
}
