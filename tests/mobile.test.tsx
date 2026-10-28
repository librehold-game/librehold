// @vitest-environment happy-dom
import { t } from '../src/i18n/i18n';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import { BUILDINGS } from '../src/data/buildings.data';
import { MAP } from '../src/data/world.config';
import { BUILDING_IDS, UNIT_IDS, type GameState } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { addOwnVillage, newDefaultGame, activeVillage } from './helpers';
import { FRAME_H, FRAME_W, MAX_H, MAX_W, mapViewport, pinchZoomDelta } from '../src/ui/map/viewport';
import { Nav, PRIMARY_TABS, SECONDARY_TABS, TABS } from '../src/ui/layout/Nav';
import { MapView } from '../src/ui/views/MapView';
import { VillageView } from '../src/ui/views/VillageView';
import { OverviewView } from '../src/ui/views/OverviewView';
import { ReportsView } from '../src/ui/views/ReportsView';
import { FarmAssistantView } from '../src/ui/views/FarmAssistantView';
import { RallyPointView } from '../src/ui/views/RallyPointView';
import { RecruitScreen } from '../src/ui/views/RecruitScreen';
import { ResearchScreen } from '../src/ui/views/ResearchScreen';

/**
 * Mobile layout: the map sizes its window from the available space, tables turn into cards,
 * and tabs move to a bottom bar. These tests guard contracts the types don't show.
 */

/** Portrait phone screen — the narrowest the game must support. */
const PHONE_W = 360;

function phoneGame(): GameState {
  const g = newDefaultGame();
  const v = activeVillage(g);
  for (const b of BUILDING_IDS) v.buildings[b] = Math.min(BUILDINGS[b].maxLevel, 10);
  v.buildings.academy = 1;
  for (const u of UNIT_IDS) { v.units[u] = 25; v.research[u] = 1; }
  v.resources = { wood: 15_000, clay: 15_000, iron: 15_000 };
  v.points = 1500;
  const second = addOwnVillage(g, v.x + 3, v.y + 2, 'Second village');
  for (const b of BUILDING_IDS) second.buildings[b] = 4;
  second.points = 400;
  return g;
}

const cached = (() => {
  let g: GameState | null = null;
  return () => (g ??= phoneGame());
})();

/** happy-dom does no layout, so the container width has to be stubbed — otherwise the map thinks it's on a desktop. */
function setClientWidth(px: number): () => void {
  const proto = globalThis.HTMLElement.prototype as unknown as Record<string, unknown>;
  const original = Object.getOwnPropertyDescriptor(proto, 'clientWidth');
  Object.defineProperty(proto, 'clientWidth', { configurable: true, get: () => px });
  return () => {
    if (original) Object.defineProperty(proto, 'clientWidth', original);
    else delete proto.clientWidth;
  };
}

beforeEach(() => {
  useGameStore.setState({ game: structuredClone(cached()), phase: 'ready', devMode: false, notice: null, lastError: null });
  useUiStore.setState({ view: 'village' });
});
afterEach(() => cleanup());

describe('the map window fits the screen', () => {
  it('keeps the default 15 × 13 tiles on desktop', () => {
    const vp = mapViewport(MAX_W + FRAME_W, MAX_H + FRAME_H, MAP.defaultZoomIdx);
    expect(vp.width).toBe(MAX_W);
    expect(vp.height).toBe(MAX_H);
    expect(vp.tw).toBe(53);
    expect(vp.th).toBe(38);
  });

  it('shrinks to the screen width on a phone', () => {
    const vp = mapViewport(PHONE_W, 430, 0);
    expect(vp.width).toBe(PHONE_W - FRAME_W);
    expect(vp.width + FRAME_W).toBeLessThanOrEqual(PHONE_W);
    expect(vp.cols).toBe(Math.ceil(vp.width / vp.tw) + 1);
    expect(vp.rows).toBe(Math.ceil(vp.height / vp.th) + 1);
  });

  it('never exceeds the default size and never vanishes', () => {
    const huge = mapViewport(4000, 3000, 0);
    expect(huge.width).toBe(MAX_W);
    expect(huge.height).toBe(MAX_H);
    const tiny = mapViewport(40, 40, 0);
    expect(tiny.width).toBeGreaterThan(200);
    expect(tiny.height).toBeGreaterThan(180);
  });

  it('zooming out fits more tiles in the same window', () => {
    const near = mapViewport(PHONE_W, 430, MAP.defaultZoomIdx);
    const far = mapViewport(PHONE_W, 430, MAP.defaultZoomIdx + 2);
    expect(far.tw).toBeLessThan(near.tw);
    expect(far.cols).toBeGreaterThan(near.cols);
    expect(far.width).toBe(near.width);
  });
});

describe('pinch changes the zoom', () => {
  it('spreading fingers zooms in, pinching zooms out', () => {
    expect(pinchZoomDelta(100, 140)).toBe(-1);
    expect(pinchZoomDelta(140, 100)).toBe(1);
  });

  it('a finger twitch does not change the zoom', () => {
    expect(pinchZoomDelta(100, 105)).toBe(0);
    expect(pinchZoomDelta(100, 95)).toBe(0);
  });

  it('a missing second finger does not break the calculation', () => {
    expect(pinchZoomDelta(0, 120)).toBe(0);
    expect(pinchZoomDelta(120, 0)).toBe(0);
  });

  it('the map responds to a two-finger pinch', () => {
    const restore = setClientWidth(PHONE_W);
    try {
      const { container } = render(<MapView />);
      const surface = container.querySelector<HTMLElement>('[style*="touch-action"]')!;
      const tiles = () => container.querySelectorAll('[title*="|"]').length;
      const before = tiles();
      fireEvent.pointerDown(surface, { pointerId: 1, clientX: 100, clientY: 200 });
      fireEvent.pointerDown(surface, { pointerId: 2, clientX: 200, clientY: 200 });
      fireEvent.pointerMove(surface, { pointerId: 2, clientX: 120, clientY: 200 });
      fireEvent.pointerUp(surface, { pointerId: 2, clientX: 120, clientY: 200 });
      fireEvent.pointerUp(surface, { pointerId: 1, clientX: 100, clientY: 200 });
      expect(tiles()).toBeGreaterThan(before);
    } finally {
      restore();
    }
  });

  it('the map starts at zoom 1 and zooms in two steps further', () => {
    const { container } = render(<MapView />);
    const tileW = () => container.querySelector<HTMLElement>('[data-tile]')!.style.width;
    expect(tileW()).toBe('53px');
    const plus = screen.getByRole('button', { name: '＋' });
    fireEvent.click(plus);
    fireEvent.click(plus);
    expect(tileW()).toBe('133px');
    fireEvent.click(plus);
    expect(tileW()).toBe('133px');
  });
});

describe('phone navigation', () => {
  it('the bottom bar shows the primary tabs and a More button', () => {
    render(<Nav view="village" onChange={() => {}} />);
    const bar = document.querySelector('.navbar-bottom')!;
    expect(bar).toBeTruthy();
    for (const tab of PRIMARY_TABS) expect(bar.textContent).toContain(t(`nav.${tab.id}.short`));
    expect(bar.textContent).toContain('Więcej');
    expect(PRIMARY_TABS.length + 1).toBeLessThanOrEqual(5);
  });

  it('More expands the remaining tabs and picking one closes them', () => {
    let picked = '';
    render(<Nav view="village" onChange={(v) => { picked = v; }} />);
    const sheet = () => document.querySelector('.navbar-bottom')!.parentElement!.querySelector('.panel');
    expect(sheet()).toBeNull();
    fireEvent.click(screen.getByText('Więcej'));
    expect(sheet()).toBeTruthy();
    fireEvent.click(sheet()!.querySelector('button')!);
    expect(picked).toBe(SECONDARY_TABS[0]!.id);
    expect(sheet()).toBeNull();
  });

  it('badges of hidden tabs add up on the More button', () => {
    render(<Nav view="village" onChange={() => {}} badges={{ quests: 2, news: 3, reports: 7 }} />);
    const more = document.querySelector('.navbar-bottom')!.lastElementChild!;
    expect(more.textContent).toContain('Więcej');
    expect(more.textContent).toContain('5');
  });

  it('every tab has a unique 0–9 shortcut — otherwise the keyboard loses a view', () => {
    const keys = TABS.map((t) => t.key);
    expect(new Set(keys).size).toBe(TABS.length);
    expect(keys.every((k) => /^[0-9]$/.test(k))).toBe(true);
    expect(PRIMARY_TABS.length + SECONDARY_TABS.length).toBe(TABS.length);
  });
});

const VIEWS: [string, ReactElement][] = [
  ['Village', <VillageView />],
  ['Map', <MapView />],
  ['Overview', <OverviewView />],
  ['Reports', <ReportsView />],
  ['Farm assistant', <FarmAssistantView />],
  ['Rally point', <RallyPointView />],
  ['Recruitment', <RecruitScreen building="barracks" />],
  ['Research', <ResearchScreen />],
];

/** Views where a table carries the main content — without card mode they can't be read on a phone. */
const MUST_STACK = ['Village', 'Overview', 'Recruitment', 'Research'];

describe('tables turn into cards', () => {
  for (const [name, el] of VIEWS) {
    it(`${name}: every card cell has a column label`, () => {
      const { container } = render(el);
      const tables = container.querySelectorAll('table.table-stack');
      if (MUST_STACK.includes(name)) expect(tables.length, `${name}: no table has card mode`).toBeGreaterThan(0);
      for (const table of tables) {
        for (const row of table.querySelectorAll('tbody tr')) {
          const cells = [...row.querySelectorAll('td')];
          cells.forEach((td, i) => {
            const ok = i === 0 || td.hasAttribute('data-label') || td.classList.contains('stack-inline');
            expect(ok, `${name}: cell ${i + 1} has no data-label — on a phone it will be a bare value with no caption`).toBe(true);
          });
        }
      }
    });
  }
});

describe('views fit on a 360 px screen', () => {
  for (const [name, el] of VIEWS) {
    it(`${name} sets no fixed width wider than the screen`, () => {
      const restore = setClientWidth(PHONE_W);
      try {
        const { container } = render(el);
        for (const node of container.querySelectorAll<HTMLElement>('[style]')) {
          // Terrain chunks are wider than a phone by design; the map surface clips them.
          if (node.closest('[data-terrain-layer]')) continue;
          const m = node.getAttribute('style')!.match(/(?:^|[^-])width:\s*([\d.]+)px/);
          if (!m) continue;
          expect(Number(m[1]), `${name}: an element ${m[1]} px wide won't fit on a ${PHONE_W} px screen`).toBeLessThanOrEqual(PHONE_W);
        }
      } finally {
        restore();
      }
    });
  }
});
