// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { MapView } from '../src/ui/views/MapView';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { BIOME, tileBiome } from '../src/ui/map/biomes';
import { armiesOnMap, armyPosition, groundAt } from '../src/ui/map/armies';
import { tileAt } from '../src/ui/map/tiles';
import type { Command, GameState, Village } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';

const HOUR = 3_600_000;

function addCmd(g: GameState, c: Partial<Command> & Pick<Command, 'id' | 'type' | 'originId' | 'targetId'>): Command {
  const full: Command = { units: {}, departedAt: g.now - HOUR, arrivesAt: g.now + 3 * HOUR, ...c };
  g.commands.push(full);
  return full;
}

function setup(build: (g: GameState, home: Village) => void): { g: GameState; home: Village } {
  const g = newTestGame();
  const home = activeVillage(g);
  build(g, home);
  useGameStore.setState({ game: g, phase: 'ready', devMode: false, notice: null, lastError: null });
  return { g, home };
}

const translate = (el: Element) => {
  const m = /translate\((-?[\d.]+)[ ,]+(-?[\d.]+)\)/.exec(el.getAttribute('transform') ?? '');
  return m ? [Number(m[1]), Number(m[2])] : null;
};

/** Pixel centre of a tile, read from the rendered tile itself. */
function tileCentre(container: HTMLElement, x: number, y: number): [number, number] {
  const el = container.querySelector<HTMLElement>(`[data-tile="${x}|${y}"]`)!;
  return [parseFloat(el.style.left) + parseFloat(el.style.width) / 2, parseFloat(el.style.top) + parseFloat(el.style.height) / 2];
}

function advance(ms: number) {
  act(() => {
    const g = useGameStore.getState().game!;
    useGameStore.setState({ game: { ...g, now: g.now + ms } });
  });
}

beforeEach(() => useUiStore.setState({ view: 'map', prefs: { ...useUiStore.getState().prefs, animations: 'full' } }));
afterEach(() => cleanup());

describe('armies on the map', () => {
  it('draws own attacks, incoming attacks and incoming support with arrows', () => {
    setup((g, home) => {
      const bot = makeBotVillage(g);
      addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
      addCmd(g, { id: 'inc', type: 'attack', originId: bot.id, targetId: home.id });
      addCmd(g, { id: 'sup', type: 'support', originId: bot.id, targetId: home.id });
    });
    const { container } = render(<MapView />);
    const kinds = [...container.querySelectorAll('[data-army]')].map((el) => el.getAttribute('data-kind')).sort();
    expect(kinds).toEqual(['incomingAttack', 'incomingSupport', 'ownAttack']);
    for (const el of container.querySelectorAll('[data-army]')) {
      expect(el.querySelector('line')!.getAttribute('marker-end')).toMatch(/^url\(#/);
    }
  });

  it("marker sits at the army's position for the current time", () => {
    let bot!: Village;
    const { home } = setup((g, home) => {
      bot = makeBotVillage(g);
      addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    });
    const { container } = render(<MapView />);
    const [ax, ay] = tileCentre(container, home.x, home.y);
    const [bx, by] = tileCentre(container, bot.x, bot.y);
    const at = translate(container.querySelector('[data-army="own"] [data-marker]')!)!;
    expect(Math.abs(at[0]! - (ax + (bx - ax) * 0.25))).toBeLessThanOrEqual(1);
    expect(Math.abs(at[1]! - (ay + (by - ay) * 0.25))).toBeLessThanOrEqual(1);
  });

  it('incoming attack never shows units even if the state has them', () => {
    setup((g, home) => {
      const bot = makeBotVillage(g);
      addCmd(g, { id: 'inc', type: 'attack', originId: bot.id, targetId: home.id, units: { axe: 100 } });
    });
    const { container } = render(<MapView />);
    const army = container.querySelector('[data-army="inc"]')!;
    expect(army.textContent).not.toContain('100');
    expect(army.innerHTML).not.toContain('unit-axe');
    expect(army.innerHTML).not.toContain('100');
  });

  it('marker over water shows a boat and land shows the army icon', () => {
    const { g } = setup((g, home) => {
      // A tile whose road home crosses drawn water outside both village clearings.
      const crossesWater = (x: number, y: number) => {
        const ends = (vx: number, vy: number) => (vx === x && vy === y) || (vx === home.x && vy === home.y);
        for (let i = 1; i < 50; i++) {
          const f = i / 50;
          if (groundAt(g.worldSeed, x + 0.5 + (home.x - x) * f, y + 0.5 + (home.y - y) * f, ends) === 'water') return true;
        }
        return false;
      };
      let water: [number, number] | null = null;
      for (let r = 2; r < 8 && !water; r++) {
        for (let dx = -r; dx <= r && !water; dx++) for (let dy = -r; dy <= r && !water; dy++) {
          if (tileBiome(g.worldSeed, home.x + dx, home.y + dy) === BIOME.water && crossesWater(home.x + dx, home.y + dy)) water = [home.x + dx, home.y + dy];
        }
      }
      if (!water) throw new Error('no water near home');
      const lake = makeBotVillage(g, water[0], water[1]);
      addCmd(g, { id: 'sup', type: 'support', originId: lake.id, targetId: home.id, departedAt: g.now, arrivesAt: g.now + HOUR });
    });
    const isVillage = (x: number, y: number) => tileAt(useGameStore.getState().game!, x, y).kind === 'village';
    const army = armiesOnMap(g).find((a) => a.id === 'sup')!;
    const ground = (t: number) => { const p = armyPosition(army, t); return groundAt(g.worldSeed, p.x, p.y, isVillage); };
    const STEP = HOUR / 200;
    let wet = -1;
    for (let t = g.now; t <= g.now + HOUR && wet < 0; t += STEP) if (ground(t) === 'water') wet = t;
    expect(wet).toBeGreaterThan(g.now);
    expect(ground(g.now)).not.toBe('water');
    const { container } = render(<MapView />);
    const marker = () => container.querySelector('[data-army="sup"] [data-marker]')!;
    expect(marker().hasAttribute('data-boat')).toBe(false);
    advance(wet - g.now);
    expect(marker().hasAttribute('data-boat')).toBe(true);
    advance(g.now + HOUR - wet);
    expect(marker().hasAttribute('data-boat')).toBe(false);
  });

  it('off mode still moves markers on the store tick', () => {
    useUiStore.setState({ prefs: { ...useUiStore.getState().prefs, animations: 'off' } });
    setup((g, home) => {
      const bot = makeBotVillage(g);
      addCmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    });
    const { container } = render(<MapView />);
    const before = translate(container.querySelector('[data-army="own"] [data-marker]')!);
    advance(HOUR);
    const after = translate(container.querySelector('[data-army="own"] [data-marker]')!);
    expect(after).not.toEqual(before);
  });
});
