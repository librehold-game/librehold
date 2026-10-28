// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { BUILDING_IDS, type BuildingId, type GameState } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { PixelVillageScene } from '../src/ui/scene/pixel/PixelVillageScene';
import { useWalkerLoop } from '../src/ui/scene/pixel/Walkers';
import { LAYOUT } from '../src/ui/scene/pixel/layout';
import { newDefaultGame, activeVillage } from './helpers';

function setup(levels: Partial<Record<BuildingId, number>>, edit?: (g: GameState) => void): GameState {
  const g = newDefaultGame();
  const v = activeVillage(g);
  for (const b of BUILDING_IDS) v.buildings[b] = 0;
  Object.assign(v.buildings, levels);
  v.buildQueue = [];
  edit?.(g);
  useGameStore.setState({ game: g, phase: 'ready', devMode: false });
  return g;
}

function job(g: GameState, building: BuildingId, startOffset: number) {
  return { id: `j-${building}`, building, targetLevel: 1, kind: 'build' as const, startedAt: g.now + startOffset, completesAt: g.now + startOffset + 60_000 };
}

function scene(mode: 'full' | 'limited' | 'off' = 'off') {
  const g = useGameStore.getState().game!;
  return render(<PixelVillageScene villageId={g.activeVillageId} mode={mode} />).container;
}

function titled(container: HTMLElement, prefix: string): Element | undefined {
  return [...container.querySelectorAll('svg title')].find((t) => t.textContent?.startsWith(prefix));
}

beforeEach(() => useUiStore.setState({ view: 'village' }));
afterEach(() => cleanup());

describe('pixel village scene', () => {
  it('draws the ground, roads of existing buildings only, and buildings in z order', () => {
    setup({ main: 10, barracks: 3, storage: 5 });
    const html = scene().innerHTML;
    expect(html).toContain('ground.png');
    expect(html).toContain('roads/gate.png');
    expect(html).toContain('roads/barracks.png');
    expect(html).not.toContain('roads/academy.png');
    expect(html.indexOf('main-')).toBeLessThan(html.indexOf('barracks-'));
  });

  it('a building has a hotspot titled with its level and the image of its stage', () => {
    setup({ main: 1, barracks: 3 });
    const c = scene();
    const title = titled(c, 'Koszary');
    expect(title?.textContent).toBe('Koszary poz. 3');
    const hotspot = title!.parentElement!;
    expect(hotspot.getAttribute('class')).toContain('hotspot');
    expect(hotspot.innerHTML).toContain('barracks-1');
  });

  it('level 0 with a queued build shows the construction site and "w kolejce"', () => {
    setup({ main: 5 }, (g) => { activeVillage(g).buildQueue = [job(g, 'main', -1000), job(g, 'academy', 60_000)]; });
    const c = scene();
    const title = titled(c, 'Akademia');
    expect(title?.textContent).toMatch(/w kolejce/);
    expect(title!.parentElement!.innerHTML).toContain('construction.png');
  });

  it('an upgrade in progress shows a progress bar and a queued one an hourglass, also in limited mode', () => {
    setup({ main: 5, barracks: 3 }, (g) => {
      activeVillage(g).buildQueue = [{ ...job(g, 'main', -30_000) }, job(g, 'barracks', 60_000)];
    });
    const c = scene('limited');
    const bar = c.querySelector('[data-marks="main"] [data-progress]');
    expect(bar?.getAttribute('data-progress')).toBe('0.5');
    expect(c.querySelector('[data-marks="barracks"]')!.textContent).toContain('⏳');
  });

  it('level badges draw above the wall towers and still open their building', () => {
    setup({ main: 5, academy: 1, wall: 1 });
    const c = scene();
    const html = c.innerHTML;
    expect(html.indexOf('data-marks="academy"')).toBeGreaterThan(html.lastIndexOf('palisade-front'));
    fireEvent.click(c.querySelector('[data-marks="academy"] .badge')!);
    expect(useUiStore.getState().buildingScreen?.building).toBe('academy');
  });

  it('building hotspots are hit through the sprite shape, not the image rectangle', () => {
    setup({ main: 5, barracks: 20, workshop: 15 });
    const c = scene();
    for (const name of ['Koszary', 'Warsztat']) {
      const hit = titled(c, name)!.parentElement!.querySelector('path.hit');
      expect(hit?.getAttribute('d'), name).toMatch(/^M/);
    }
  });

  it('level 0 without a build draws nothing but the hotspot', () => {
    setup({ main: 5 });
    const c = scene();
    const hotspot = titled(c, 'Akademia')!.parentElement!;
    expect(hotspot.querySelector('image')).toBeNull();
    expect(c.innerHTML).not.toContain('academy-');
  });

  it('the rally point is titled "Plac zborny" and opens the rally view', () => {
    setup({ main: 5 });
    const c = scene();
    const title = titled(c, 'Plac zborny');
    fireEvent.click(title!.parentElement!);
    expect(useUiStore.getState().view).toBe('rally');
  });

  it('wall level 1 draws the palisade, level 20 the stone ring, level 0 nothing — the wall hotspot is always there', () => {
    setup({ main: 5, wall: 1 });
    let c = scene();
    expect(c.innerHTML).toContain('walls/palisade-back.png');
    expect(c.innerHTML).not.toContain('walls/stone-back.png');
    expect(titled(c, 'Mur obronny')?.textContent).toBe('Mur obronny poz. 1');
    cleanup();
    setup({ main: 5, wall: 20 });
    c = scene();
    expect(c.innerHTML).toContain('walls/stone-back.png');
    expect(c.innerHTML).toContain('walls/stone-gate.png');
    cleanup();
    setup({ main: 5, iron: 3 });
    c = scene();
    expect(c.innerHTML).not.toContain('walls/');
    expect(c.innerHTML).toContain('roads/side_w.png');
    const wall = titled(c, 'Mur obronny');
    expect(wall?.textContent).toBe('Mur obronny (niewybudowany)');
    fireEvent.click(wall!.parentElement!);
    expect(useUiStore.getState().buildingScreen?.building).toBe('wall');
  });

  it('the closed gate stands on the wall base, not above it', () => {
    // Bottom opaque row of the closed gate frame, measured in the exported strips; open doors swing lower.
    const closedFoot = { palisade: 73, stone: 107 };
    const wallBase = LAYOUT.ring.cy + LAYOUT.ring.ry;
    for (const [kind, wall] of [['palisade', 1], ['stone', 20]] as const) {
      setup({ main: 5, wall });
      const gate = scene().querySelector(`[data-anim="${kind}-gate"]`)!;
      const foot = Number(gate.getAttribute('y')) + closedFoot[kind];
      expect(Math.abs(foot - wallBase), `${kind} gate foot ${foot}, wall base ${wallBase}`).toBeLessThanOrEqual(4);
      cleanup();
    }
  });

  it('back wall strips draw before the town hall, front strips after the workshop', () => {
    setup({ main: 20, workshop: 5, wall: 20 });
    const html = scene().innerHTML;
    expect(html.indexOf('stone-back')).toBeLessThan(html.indexOf('buildings/main-'));
    expect(html.indexOf('buildings/main-')).toBeLessThan(html.indexOf('buildings/workshop-'));
    expect(html.indexOf('buildings/workshop-')).toBeLessThan(html.lastIndexOf('stone-front'));
  });

  it('night adds the dark overlay and lights, day does not', () => {
    const at = (hour: number) => (g: GameState) => { g.clock.originLocalMs = hour * 3_600_000 - (g.now % 86_400_000); };
    setup({ main: 5, iron: 5, smithy: 5 }, at(1));
    let html = scene().innerHTML;
    expect(html).toContain('data-night="night"');
    expect(html).toContain('url(#px-glow)');
    cleanup();
    setup({ main: 5, iron: 5, smithy: 5 }, at(12));
    html = scene().innerHTML;
    expect(html).not.toContain('data-night');
    expect(html).not.toContain('url(#px-glow)');
  });

  it('night lights only shine at buildings that exist', () => {
    const night = (g: GameState) => { g.clock.originLocalMs = 3_600_000 - (g.now % 86_400_000); };
    setup({ main: 5, iron: 5, smithy: 5 }, night);
    const all = scene().querySelectorAll('circle[fill="url(#px-glow)"]').length;
    cleanup();
    setup({ main: 5 }, night);
    const few = scene().querySelectorAll('circle[fill="url(#px-glow)"]').length;
    expect(few).toBeLessThan(all);
    expect(few).toBe(1);
  });

  it('full mode renders walkers, limited mode does not', () => {
    setup({ main: 10, barracks: 5, storage: 5, smithy: 5, market: 5, farm: 10 });
    expect(scene('full').querySelectorAll('[data-walker]').length).toBeGreaterThan(0);
    cleanup();
    expect(scene('limited').querySelectorAll('[data-walker]').length).toBe(0);
  });

  it('unmounting the scene cancels the frame loop', () => {
    setup({ main: 10, barracks: 5, storage: 5 });
    const cancel = vi.spyOn(window, 'cancelAnimationFrame');
    const g = useGameStore.getState().game!;
    const { unmount } = render(<PixelVillageScene villageId={g.activeVillageId} mode="full" />);
    unmount();
    expect(cancel).toHaveBeenCalled();
    cancel.mockRestore();
  });

  it('stopping the frame loop reports the gate clear, so it closes instead of staying open', () => {
    const onGate = vi.fn();
    function Harness() {
      useWalkerLoop([], [], false, onGate);
      return null;
    }
    const { unmount } = render(<Harness />);
    unmount();
    expect(onGate).toHaveBeenCalledWith(false);
  });

  it('off mode renders no animations', () => {
    setup({ main: 20, wood: 12, iron: 12, farm: 12, wall: 20 });
    expect(scene('off').innerHTML).not.toContain('animation:');
  });
});
