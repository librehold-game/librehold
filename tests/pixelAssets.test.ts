import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

/** `npm run assets:pixel` output: the village layout and every file it references. */

const ROOT = join(__dirname, '..');
const PIXEL = join(ROOT, 'src', 'assets', 'pixel');
const DIR = join(PIXEL, 'village');
const LAYOUT_PATH = join(DIR, 'layout.json');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const layout: any = existsSync(LAYOUT_PATH) ? JSON.parse(readFileSync(LAYOUT_PATH, 'utf8')) : null;

const BUILDINGS = ['main', 'barracks', 'stable', 'workshop', 'academy', 'smithy', 'market', 'storage', 'hide',
  'statue', 'wood', 'clay', 'iron', 'farm'];

function referencedFiles(): string[] {
  const out: string[] = [layout.construction, layout.place];
  for (const r of Object.values(layout.roads) as { file: string }[]) out.push(r.file);
  for (const s of layout.scenery as { file: string }[]) out.push(s.file);
  for (const stages of Object.values(layout.buildings) as Record<string, string>[]) out.push(...Object.values(stages));
  for (const w of Object.values(layout.walls) as { back: string; front: string; gate: { file: string } }[]) out.push(w.back, w.front, w.gate.file);
  for (const a of Object.values(layout.anim) as { file: string }[]) out.push(a.file);
  for (const v of Object.values(layout.villagers) as { file: Record<string, string> }[]) out.push(...Object.values(v.file));
  return out;
}

function dirSize(dir: string, skip: string[] = []): number {
  let total = 0;
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (skip.includes(p)) continue;
    total += e.isDirectory() ? dirSize(p, skip) : statSync(p).size;
  }
  return total;
}

describe('pixel art village export', () => {
  it('layout.json exists', () => expect(layout).not.toBeNull());

  it('scene is 880×600 and every slot lies inside it', () => {
    expect([layout.w, layout.h]).toEqual([880, 600]);
    expect(Object.keys(layout.slots).sort()).toEqual([...BUILDINGS].sort());
    for (const [x, y] of Object.values(layout.slots) as [number, number][]) {
      expect(x).toBeGreaterThanOrEqual(0);
      expect(x).toBeLessThanOrEqual(880);
      expect(y).toBeGreaterThanOrEqual(0);
      expect(y).toBeLessThanOrEqual(600);
    }
  });

  it('every building has stages 1–3, the statue only 3', () => {
    for (const id of BUILDINGS) {
      const stages = Object.keys(layout.buildings[id]).sort();
      expect(stages, id).toEqual(id === 'statue' ? ['3'] : ['1', '2', '3']);
    }
  });

  it('every file named in layout.json exists and has its size recorded', () => {
    for (const f of referencedFiles()) {
      expect(existsSync(join(DIR, f)), f).toBe(true);
      expect(layout.files[f], f).toHaveLength(2);
    }
    expect(existsSync(join(DIR, 'ground.png'))).toBe(true);
  });

  it('wall strips cover the full ring width', () => {
    const { cx, rx } = layout.ring;
    for (const kind of ['palisade', 'stone']) {
      const xs = (layout.walls[kind].strips as { x0: number }[]).map((s) => s.x0);
      expect(Math.min(...xs)).toBeLessThanOrEqual(cx - rx + 8);
      expect(Math.max(...xs) + 16).toBeGreaterThanOrEqual(cx + rx);
      xs.slice(1).forEach((x, i) => expect(x - xs[i]!).toBe(8));
    }
  });

  it('every clickable sprite has a hit shape tighter than its rectangle', () => {
    const files = [layout.construction, layout.place, ...Object.values(layout.buildings).flatMap((s) => Object.values(s as Record<string, string>))];
    for (const f of new Set(files)) {
      const d = layout.hits?.[f] as string | undefined;
      expect(d, f).toMatch(/^M/);
      const [w, h] = layout.files[f] as [number, number];
      const area = [...d!.matchAll(/h(\d+)v(\d+)/g)].reduce((a, m) => a + Number(m[1]) * Number(m[2]), 0);
      expect(area, f).toBeLessThan(w * h * 0.9);
      expect(area, f).toBeGreaterThan(0);
    }
  });

  it('scenery footprints keep clear of the wall', () => {
    const { cx, cy, rx, ry } = layout.ring;
    const d = (x: number, y: number) => Math.hypot((x - cx) / rx, (y - cy) / ry);
    for (const s of layout.scenery as { file: string; x: number; y: number }[]) {
      const [w] = layout.files[s.file] as [number, number];
      const foot = [-0.5, -0.25, 0, 0.25, 0.5].map((t) => d(s.x + t * w, s.y));
      const label = `${s.file} at ${s.x},${s.y}`;
      if (d(s.x, s.y) > 1) expect(Math.min(...foot), label).toBeGreaterThanOrEqual(1.06);
      else expect(Math.max(...foot), label).toBeLessThanOrEqual(0.92);
    }
  });

  it('roads have at least 2 points and the gate road starts below the frame', () => {
    for (const r of Object.values(layout.roads) as { points: number[][] }[]) expect(r.points.length).toBeGreaterThanOrEqual(2);
    expect(layout.roads.gate.points[0][1]).toBeGreaterThan(600);
  });

  // The account site's clip and screenshots are not game art; tests/siteMedia.test.ts caps each of them.
  it('the game art in src/assets/pixel stays within 3 MB', () => {
    expect(dirSize(PIXEL, [join(PIXEL, 'account')])).toBeLessThanOrEqual(3 * 1024 * 1024);
  });
});
