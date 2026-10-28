// @vitest-environment happy-dom
import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MapView } from '../src/ui/views/MapView';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { BIOME, tileBiome } from '../src/ui/map/biomes';
import { crowdLayout } from '../src/ui/map/crowdLayout';
import { groundAt } from '../src/ui/map/armies';
import type { Command, GameState, Village } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';

const HOUR = 3_600_000;

/** A 2D context that accepts every call; happy-dom has no canvas. */
function fakeContext(): CanvasRenderingContext2D {
  const props: Record<string | symbol, unknown> = {};
  return new Proxy(props, {
    get: (t, k) => (k in t ? t[k] : () => undefined),
    set: (t, k, v) => { t[k] = v; return true; },
  }) as unknown as CanvasRenderingContext2D;
}

beforeAll(() => {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation((() => fakeContext()) as never);
});

function addCmd(g: GameState, c: Partial<Command> & Pick<Command, 'id' | 'type' | 'originId' | 'targetId'>): Command {
  const full: Command = { units: {}, departedAt: g.now - HOUR, arrivesAt: g.now + 3 * HOUR, ...c };
  g.commands.push(full);
  return full;
}

function setup(build: (g: GameState, home: Village) => void): void {
  const g = newTestGame();
  build(g, activeVillage(g));
  useGameStore.setState({ game: g, phase: 'ready', devMode: false, notice: null, lastError: null });
}

function renderZoomed(steps = 2) {
  const r = render(<MapView />);
  const plus = screen.getByRole('button', { name: '＋' });
  for (let i = 0; i < steps; i++) fireEvent.click(plus);
  return r;
}

const crowd = (c: HTMLElement) => c.querySelector<HTMLCanvasElement>('canvas[data-crowd]');

beforeEach(() => useUiStore.setState({ view: 'map', mapCenter: null, prefs: { ...useUiStore.getState().prefs, animations: 'full' } }));
afterEach(() => cleanup());

describe('marching crowds', () => {
  it('own army of 50 and 5000 axemen draw clearly different crowds', () => {
    for (const [axe, figures] of [[50, 41], [5000, 147]] as const) {
      setup((g, home) => addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: makeBotVillage(g).id, units: { axe } }));
      const { container } = renderZoomed();
      expect(crowd(container)!.getAttribute('data-figures')).toBe(String(figures));
      cleanup();
    }
  });

  it('incoming attack draws the generic torch group of fixed size', () => {
    setup((g, home) => addCmd(g, { id: 'inc', type: 'attack', originId: makeBotVillage(g).id, targetId: home.id, units: { axe: 5000 } }));
    const { container } = renderZoomed();
    expect(crowd(container)!.getAttribute('data-figures')).toBe('8');
  });

  it('army over water draws boats', () => {
    setup((g, home) => {
      // A lake tile and how far along the road from home the army is over drawn water, outside both clearings.
      const wetAt = (x: number, y: number) => {
        const ends = (vx: number, vy: number) => (vx === x && vy === y) || (vx === home.x && vy === home.y);
        for (let i = 1; i < 50; i++) {
          const f = i / 50;
          if (groundAt(g.worldSeed, home.x + 0.5 + (x - home.x) * f, home.y + 0.5 + (y - home.y) * f, ends) === 'water') return f;
        }
        return 0;
      };
      let water: [number, number, number] | null = null;
      for (let r = 2; r < 9 && !water; r++) {
        for (let dx = -r; dx <= r && !water; dx++) for (let dy = -r; dy <= r && !water; dy++) {
          const x = home.x + dx, y = home.y + dy;
          const f = tileBiome(g.worldSeed, x, y) === BIOME.water ? wetAt(x, y) : 0;
          if (f > 0) water = [x, y, f];
        }
      }
      if (!water) throw new Error('no water near home');
      const [x, y, f] = water;
      const lake = makeBotVillage(g, x, y);
      addCmd(g, { id: 'own', type: 'support', originId: home.id, targetId: lake.id, departedAt: g.now - f * HOUR, arrivesAt: g.now + (1 - f) * HOUR, units: { spear: 40 } });
      useUiStore.setState({ mapCenter: { x: Math.round(home.x + (x - home.x) * f), y: Math.round(home.y + (y - home.y) * f) } });
    });
    const { container } = renderZoomed();
    expect(Number(crowd(container)!.getAttribute('data-boats'))).toBeGreaterThan(0);
    expect(crowd(container)!.getAttribute('data-figures')).toBe('0');
  });

  it('crowd budget leaves extra armies as icons', () => {
    setup((g, home) => {
      const bot = makeBotVillage(g);
      for (let i = 0; i < 5; i++) addCmd(g, { id: `own${i}`, type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 5000 } });
    });
    const { container } = renderZoomed();
    expect(Number(crowd(container)!.getAttribute('data-figures'))).toBeLessThanOrEqual(600);
    const markers = [...container.querySelectorAll('[data-army] [data-marker]')];
    expect(markers).toHaveLength(5);
    expect(markers.some((m) => !m.hasAttribute('data-in-crowd'))).toBe(true);
    expect(markers.some((m) => m.hasAttribute('data-in-crowd'))).toBe(true);
  });

  it('an own crowd shows the exact count of each unit type beside it, a foreign one shows none', () => {
    setup((g, home) => {
      const bot = makeBotVillage(g);
      addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 777, noble: 1 } });
      addCmd(g, { id: 'inc', type: 'attack', originId: bot.id, targetId: home.id, units: { axe: 4321 } });
    });
    const { container } = renderZoomed();
    const counts = container.querySelector('[data-army="own"] [data-counts]')!;
    expect(counts.getAttribute('display')).toBe('inline');
    expect(counts.textContent).toContain('777');
    expect(counts.innerHTML).toContain('unit-noble');
    expect(container.querySelector('[data-army="inc"] [data-counts]')).toBeNull();
  });

  it('only the crowds nearest the middle carry count labels', () => {
    setup((g, home) => {
      const bot = makeBotVillage(g);
      for (let i = 0; i < 6; i++) addCmd(g, { id: `own${i}`, type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 30 } });
    });
    const { container } = renderZoomed();
    const shown = [...container.querySelectorAll('[data-counts]')].filter((el) => el.getAttribute('display') === 'inline');
    expect(container.querySelectorAll('[data-in-crowd]')).toHaveLength(6);
    expect(shown).toHaveLength(4);
  });

  it('no crowd at zoom 1 or with animations off', () => {
    setup((g, home) => addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: makeBotVillage(g).id, units: { axe: 50 } }));
    const { container } = renderZoomed(0);
    expect(crowd(container)).toBeNull();
    cleanup();
    useUiStore.setState({ prefs: { ...useUiStore.getState().prefs, animations: 'off' } });
    const again = renderZoomed();
    expect(crowd(again.container)).toBeNull();
  });

  it('crowdLayout puts every figure behind the head in rows of up to six, hetmans in the middle', () => {
    const spots = crowdLayout([{ unit: 'axe', figures: 10 }, { unit: 'noble', figures: 1 }, { unit: 'lightCav', figures: 10 }], { dx: 1, dy: 0 }, 10);
    expect(spots).toHaveLength(21);
    expect(spots.every((s) => s.dx <= 0)).toBe(true);
    const rows = new Map<number, number>();
    for (const s of spots) rows.set(s.dx, (rows.get(s.dx) ?? 0) + 1);
    expect(Math.max(...rows.values())).toBeLessThanOrEqual(6);
    const order = spots.map((s) => s.unit);
    const noble = order.indexOf('noble');
    expect(noble).toBeGreaterThan(0);
    expect(noble).toBeLessThan(order.length - 1);
    expect(order.slice(0, noble).every((u) => u === order[0])).toBe(true);
  });
});
