import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { UNIT_IDS } from '../src/engine/types';
import { TABS } from '../src/ui/layout/Nav';

const ROOT = join(__dirname, '..');
const UI = join(ROOT, 'src', 'assets', 'pixel', 'ui');

const CHROME = ['panel', 'header', 'plank', 'btn', 'btn-danger', 'tab', 'tab-active', 'input', 'frame', 'iconbtn-active', 'thumb', 'leather'];

function dirSize(dir: string): number {
  return readdirSync(dir, { withFileTypes: true }).reduce((t, e) => t + (e.isDirectory() ? dirSize(join(dir, e.name)) : statSync(join(dir, e.name)).size), 0);
}

describe('pixel UI assets', () => {
  it('every chrome piece is exported', () => {
    for (const name of CHROME) expect(existsSync(join(UI, 'chrome', `${name}.png`)), name).toBe(true);
  });

  it('there is an icon for every resource, the population, every unit and every nav tab', () => {
    const names = ['wood', 'clay', 'iron', 'pop', 'nav-menu', ...UNIT_IDS.map((u) => `unit-${u}`), ...TABS.map((t) => `nav-${t.id}`)];
    const missing = names.filter((n) => !existsSync(join(UI, 'icons', `${n}.png`)));
    expect(missing).toEqual([]);
  });

  it('src/assets/pixel/ui stays within 400 KB', () => {
    expect(dirSize(UI)).toBeLessThanOrEqual(400 * 1024);
  });
});

describe('pixel skin CSS', () => {
  const css = readFileSync(join(ROOT, 'src', 'ui', 'pixelSkin.css'), 'utf8');

  it('uses every chrome piece', () => {
    const unused = CHROME.filter((name) => !css.includes(`chrome/${name}.png`));
    expect(unused).toEqual([]);
  });

  it('every rule is scoped to html.pixel, which game.html sets', () => {
    expect(readFileSync(join(ROOT, 'game.html'), 'utf8')).toMatch(/<html[^>]*class="pixel"/);
    const selectors = [...css.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{/g)]
      .map((m) => m[1]!.trim())
      .filter((s) => s && !s.startsWith('@'));
    expect(selectors.length).toBeGreaterThan(5);
    for (const list of selectors) for (const sel of list.split(',')) expect(sel.trim(), sel).toMatch(/^html\.pixel/);
  });

  const block = (selector: string) => {
    const at = css.indexOf(`${selector} {`);
    expect(at, selector).toBeGreaterThanOrEqual(0);
    return css.slice(at, css.indexOf('}', at));
  };

  it('panels carry no filter, which would trap position:fixed popups inside them', () => {
    expect(block('html.pixel .panel')).not.toMatch(/filter/);
  });

  it('the wooden frame keeps the 10px border the map layout constants assume, the art is drawn outward', () => {
    const frame = block('html.pixel .frame-wood');
    expect(frame).toMatch(/border-width: 10px/);
    expect(frame).toMatch(/border-image-outset: 10px/);
  });

  it('only the page background is pixelated: descendants are reset so photo textures stay smooth', () => {
    expect(block('html.pixel body *')).toMatch(/image-rendering: auto/);
  });

  it('inputs and touch buttons are styled outside the layers so utility borders and paddings do not undo them', () => {
    const unlayered = css.slice(css.indexOf('\n}\n'));
    expect(unlayered).toMatch(/html\.pixel input\[type="text"\]/);
    expect(unlayered).toMatch(/border-width: 6px/);
    expect(unlayered).toMatch(/@media \(pointer: coarse\)/);
  });

  it('the active bottom-nav button does not change its box', () => {
    const active = block('html.pixel .navbtn-active');
    expect(active).toMatch(/border-width: 0/);
    expect(active).not.toMatch(/margin/);
  });

  it('the map hover card has an opaque parchment body, so it reads over the pixel terrain', () => {
    expect(readFileSync(join(ROOT, 'src', 'ui', 'map', 'MapHoverCard.tsx'), 'utf8')).toMatch(/className="[^"]*\bmap-card\b/);
    expect(block('html.pixel .map-card')).toMatch(/background-color: #[0-9a-f]{6}/);
  });

  it('the vertical map ruler stacks planks at their own height instead of stretching one', () => {
    expect(readFileSync(join(ROOT, 'src', 'ui', 'map', 'TileMap.tsx'), 'utf8')).toMatch(/wood-strip map-ruler-y/);
    const ruler = block('html.pixel .map-ruler-y');
    expect(ruler).toMatch(/background-size: auto 16px/);
    expect(ruler).toMatch(/background-repeat: repeat/);
  });

  it('is imported by the app stylesheet', () => {
    expect(readFileSync(join(ROOT, 'src', 'index.css'), 'utf8')).toContain('./ui/pixelSkin.css');
  });
});
