// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { BUILDING_IDS, type BuildingId, type GameState, type UnitId } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { PixelVillageScene } from '../src/ui/scene/pixel/PixelVillageScene';
import { WORK_HIT, SHOOT_RELEASE_FRAME, SHOOT_SEQ, useWalkerLoop, WALK_CYCLE_MS, WORK_STEP_MS } from '../src/ui/scene/pixel/Walkers';
import { wallLayers } from '../src/ui/scene/pixel/WallRing';
import { buildRoutes, gateOpen, MOW_SPEED, RIDE_SPEED } from '../src/ui/scene/pixel/routes';
import { LAYOUT } from '../src/ui/scene/pixel/layout';
import { villageStorageCapacity } from '../src/engine/resources';
import { variantFor } from '../src/ui/scene/sceneModel';
import { BUILDINGS } from '../src/data/buildings.data';
import { newDefaultGame, activeVillage } from './helpers';

const sceneVariant = (level: number) => variantFor(level, 15) as 1 | 2 | 3;
/** The lowest level at which a building is drawn with the art of `stage`. */
const levelFor = (b: BuildingId, stage: 1 | 2 | 3) => {
  let level = 1;
  while (variantFor(level, BUILDINGS[b].maxLevel) !== stage) level++;
  return level;
};
const drillAt = (unit: UnitId, stage: 1 | 2 | 3) => LAYOUT.drills[unit]!.find((d) => d.stage === stage)!;
const placed = (el: Element, sprite: keyof typeof LAYOUT.villagers, [x, y]: readonly number[]) => {
  const s = LAYOUT.villagers[sprite];
  expect(el.getAttribute('transform')).toBe(`translate(${Math.round(x! - s.anchor[0])}, ${Math.round(y! - s.anchor[1])})`);
};

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

function recruit(g: GameState, unit: UnitId) {
  return { id: `r-${unit}`, unit, total: 5, done: 0, perUnitMs: 60_000, startedAt: g.now - 1000, completesAt: g.now + 299_000 };
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
    expect(c.querySelector('[data-marks="barracks"] [data-icon="hourglass"]')).toBeTruthy();
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

  it('walkers through the gate draw over the whole gate and under its front, after every front wall strip', () => {
    const levels = { ...Object.fromEntries(BUILDING_IDS.map((id) => [id, 0])), main: 5, wood: 5 } as Record<BuildingId, number>;
    const routes = buildRoutes({ levels, construction: [{ building: 'main', ratio: 0.2, started: true }] }, { citizens: 10, guards: 0 });
    const through = routes.filter((r) => r.points.some((p) => gateOpen([p], LAYOUT.gates.s)));
    expect(through.map((r) => r.state).sort()).toEqual(['carry_log', 'walk']);
    for (const kind of ['palisade', 'stone'] as const) {
      const layers = wallLayers(kind, 6);
      const z = (key: string) => layers.find((l) => l.key === key)?.z ?? NaN;
      const strips = layers.filter((l) => l.key.startsWith('wall-f')).map((l) => l.z);
      expect(Math.max(...strips), kind).toBeLessThan(z('wall-gate'));
      for (const r of through) {
        expect(r.z, `${kind} ${r.state}`).toBeGreaterThan(z('wall-gate'));
        expect(r.z, `${kind} ${r.state}`).toBeLessThan(z('wall-gate-front'));
      }
    }
  });

  it('the gate front shows the same frame as the gate, clipped by a shape defined in the scene', () => {
    for (const [kind, wall] of [['palisade', 1], ['stone', 20]] as const) {
      setup({ main: 5, wall });
      const c = scene();
      const gate = c.querySelector(`[data-anim="${kind}-gate"]`)!;
      const front = c.querySelector(`[data-anim="${kind}-gate-front"]`);
      expect(front, kind).not.toBeNull();
      expect(front!.innerHTML).toBe(gate.innerHTML);
      expect(['x', 'y'].map((a) => front!.getAttribute(a))).toEqual(['x', 'y'].map((a) => gate.getAttribute(a)));
      const id = /^url\(#(.+)\)$/.exec(front!.parentElement!.getAttribute('clip-path') ?? '')?.[1];
      expect(c.querySelector(`clipPath[id="${id}"] path`), kind).not.toBeNull();
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

  it('a farmer mows the wheat field once the farm is built', () => {
    setup({ main: 1, farm: 1 });
    const mower = scene('full').querySelector('[data-walker="mow"]');
    expect(mower?.innerHTML).toContain('mow-east');
    cleanup();
    setup({ main: 1 });
    expect(scene('full').querySelector('[data-walker="mow"]')).toBeNull();
  });

  it('the farmer advances while he mows, then turns and walks back to the start of his pass', () => {
    setup({ main: 1, farm: 1 });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const mower = scene('full').querySelector('[data-walker="mow"]')!;
    const at = (seconds: number) => {
      ticks.at(-1)!(seconds * 1000);
      const [, x, y] = /translate\((-?[\d.]+), (-?[\d.]+)\)/.exec(mower.getAttribute('transform')!)!.map(Number);
      return { x: x!, y: y!, href: mower.querySelector('image')!.getAttribute('href')! };
    };
    const [[x0], [x1]] = LAYOUT.mow;
    const mowing = (x1 - x0) / MOW_SPEED;
    const start = at(0);
    const half = at(mowing / 2);
    expect(half.x - start.x).toBeCloseTo((x1 - x0) / 2, -0.5);
    expect(half.href).toContain('mow-east');
    const back = at(mowing + mowing / 8);
    expect(back.href).toContain('scythe_walk-west');
    expect(back.x).toBeLessThan(start.x + (x1 - x0));
    expect(back.x).toBeGreaterThan(start.x);
    expect(at(mowing * 1.25 + 0.5).href).toContain('mow-east');
    raf.mockRestore();
    now.mockRestore();
  });

  it('the wheat covers the mower around his feet and follows him along his pass', () => {
    setup({ main: 1, farm: 1 });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const c = scene('full');
    const mower = c.querySelector('[data-walker="mow"]')!;
    const cover = c.querySelector('[data-cover="mow"]')!;
    expect(cover.innerHTML).toContain('plot-wheat');
    expect(mower.compareDocumentPosition(cover) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const band = c.querySelector(`${cover.getAttribute('clip-path')!.slice(4, -1)} rect`)!;
    const left = () => Number(band.getAttribute('x'));
    const mowerLeft = () => Number(/translate\((-?[\d.]+),/.exec(mower.getAttribute('transform')!)![1]);
    const [, footY] = LAYOUT.mow[0];
    expect(Number(band.getAttribute('y'))).toBeLessThan(footY);
    expect(Number(band.getAttribute('y')) + Number(band.getAttribute('height'))).toBeGreaterThan(footY);
    const [[x0], [x1]] = LAYOUT.mow;
    ticks.at(-1)!(((x1 - x0) / 2 / MOW_SPEED) * 1000);
    expect(left()).toBe(mowerLeft());
    expect(left()).toBeGreaterThan(Math.round(x0 - LAYOUT.villagers.mow.anchor[0]));
    raf.mockRestore();
    now.mockRestore();
  });

  it('walkers take one walk cycle per WALK_CYCLE_MS whatever their frame count; workers play their whole loop', () => {
    setup({ main: 5, barracks: 5, storage: 5, wood: 5 });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const c = scene('full');
    const walk = LAYOUT.villagers.walk;
    const chop = LAYOUT.villagers.chop;
    ticks.at(-1)!(WALK_CYCLE_MS / 2);
    const walker = c.querySelector('[data-walker="walk"] svg')!;
    expect(walker.getAttribute('viewBox')).toBe(`${(walk.frames / 2) * walk.w} 0 ${walk.w} ${walk.h}`);
    const last = chop.frames - 1;
    ticks.at(-1)!(last * WORK_STEP_MS);
    expect(c.querySelector('[data-walker="chop"] svg')!.getAttribute('viewBox')).toBe(`${last * chop.w} 0 ${chop.w} ${chop.h}`);
    raf.mockRestore();
    now.mockRestore();
  });

  it('a guard, whose draw order changes along the patrol, has a copy per order, and only one of them shows at a time', () => {
    setup({ main: 5, wall: 20 });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const c = scene('full');
    const copies = () => [...c.querySelectorAll('[data-walker="guard"]')];
    expect(copies().length).toBeGreaterThan(2);
    for (const seconds of [0, 7, 19, 31]) {
      ticks.at(-1)!(seconds * 1000);
      expect(copies().filter((el) => el.getAttribute('visibility') !== 'hidden')).toHaveLength(2);
    }
    raf.mockRestore();
    now.mockRestore();
  });

  it('an archer shoots at two targets in the yard while a stage-3 barracks trains archers', () => {
    setup({ main: 1, barracks: 17 }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, 'archer')]; });
    const c = scene('full');
    const archer = c.querySelector('[data-walker="shoot"]')!;
    expect(archer.innerHTML).toContain('shoot-east');
    const targets = c.querySelectorAll('[data-prop="target"]');
    expect(targets.length).toBe(2);
    const barracks = titled(c, 'Koszary')!.parentElement!;
    for (const el of [archer, ...targets]) expect(barracks.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it.each([['spear', 'thrust'], ['sword', 'slash'], ['axe', 'axe_chop']] as const)('%s training: one soldier strikes a dummy in the yard with %s', (unit, sprite) => {
    setup({ main: 1, barracks: 17 }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, unit)]; });
    const c = scene('full');
    expect(c.querySelectorAll('[data-walker]:is([data-walker="shoot"], [data-walker="thrust"], [data-walker="slash"], [data-walker="axe_chop"])').length).toBe(1);
    expect(c.querySelector(`[data-walker="${sprite}"]`)?.innerHTML).toContain(`${sprite}-east`);
    expect(c.querySelectorAll('[data-prop="dummy"]').length).toBe(1);
    expect(c.querySelector('[data-prop="target"]')).toBeNull();
  });

  it('the dummy jolts when the blow lands, then settles', () => {
    setup({ main: 1, barracks: 17 }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, 'axe')]; });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const dummy = scene('full').querySelector('[data-prop="dummy"]')!;
    const dx = (step: number) => {
      ticks.at(-1)!(step * WORK_STEP_MS + 1);
      return Number(/translate\((-?\d+)/.exec(dummy.getAttribute('transform') ?? 'translate(0')![1]);
    };
    const hit = WORK_HIT.axe_chop;
    expect(dx(hit - 1)).toBe(0);
    expect(dx(hit)).not.toBe(0);
    expect(dx(hit + 4)).toBe(0);
    raf.mockRestore();
    now.mockRestore();
  });

  it('a digger works the clay pit and sits down to rest while the warehouse is full of clay', () => {
    setup({ main: 1, clay: 1, storage: 10 });
    expect(scene('full').querySelector('[data-walker="dig"]')?.innerHTML).toContain('dig-east');
    cleanup();
    setup({ main: 1, clay: 1, storage: 10 }, (g) => { activeVillage(g).resources.clay = villageStorageCapacity(activeVillage(g)); });
    const c = scene('full');
    expect(c.querySelector('[data-walker="dig_rest"]')?.innerHTML).toContain('dig_rest-east');
    expect(c.querySelector('[data-walker="dig"]')).toBeNull();
    cleanup();
    setup({ main: 1 });
    expect(scene('full').querySelector('[data-walker="dig"], [data-walker="dig_rest"]')).toBeNull();
  });

  it('a mine and a sawmill stand still and the woodcutter sits down while the warehouse is full of what they make', () => {
    const moving = (c: HTMLElement, name: string) => c.querySelector(`[data-anim="${name}"] image`)!.getAttribute('style')!.includes('animation');
    setup({ main: 1, wood: 12, iron: 12, storage: 10 });
    let c = scene('full');
    expect(moving(c, 'iron')).toBe(true);
    expect(moving(c, 'wood')).toBe(true);
    cleanup();
    setup({ main: 1, wood: 12, iron: 12, storage: 10 }, (g) => {
      const v = activeVillage(g);
      v.resources.iron = villageStorageCapacity(v);
      v.resources.wood = villageStorageCapacity(v);
    });
    c = scene('full');
    expect(moving(c, 'iron')).toBe(false);
    expect(moving(c, 'wood')).toBe(false);
    expect(c.querySelector('[data-walker="chop_rest"]')).toBeTruthy();
    expect(c.querySelector('[data-walker="chop"]')).toBeNull();
  });

  it('the blacksmith hammers at the anvil while research runs, and sparks fly on the blow', () => {
    const research = (g: GameState) => {
      activeVillage(g).researchQueue = [{ id: 'rs', unit: 'sword', targetLevel: 1, startedAt: g.now - 1000, completesAt: g.now + 60_000 }];
    };
    setup({ main: 1, smithy: 14 });
    expect(scene('full').querySelector('[data-walker="forge"]')).toBeNull();
    cleanup();
    setup({ main: 1, smithy: 14 }, research);
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const c = scene('full');
    expect(c.querySelector('[data-walker="forge"]')?.innerHTML).toContain('forge-east');
    const sparks = c.querySelector('[data-sparks]')!;
    const at = (step: number) => { ticks.at(-1)!(step * WORK_STEP_MS + 1); return sparks.getAttribute('visibility'); };
    expect(at(WORK_HIT.forge - 1)).toBe('hidden');
    expect(at(WORK_HIT.forge)).toBe('visible');
    expect(at(WORK_HIT.forge + 2)).toBe('hidden');
    raf.mockRestore();
    now.mockRestore();
  });

  it.each([1, 2, 3] as const)('the blacksmith of a stage-%i smithy stands at its anvil; only stage 2 brings its own', (stage) => {
    setup({ main: 1, smithy: levelFor('smithy', stage) }, (g) => {
      activeVillage(g).researchQueue = [{ id: 'rs', unit: 'sword', targetLevel: 1, startedAt: g.now - 1000, completesAt: g.now + 60_000 }];
    });
    const c = scene('full');
    placed(c.querySelector('[data-walker="forge"]')!, 'forge', LAYOUT.workers.smith[stage].at);
    expect(c.querySelector('[data-prop="anvil"]') !== null).toBe(stage === 2);
  });

  it.each([1, 2, 3])('a hetman in training commands on the square of a level-%i academy', (level) => {
    setup({ main: 1, academy: level });
    expect(scene('full').querySelector('[data-walker="command"]')).toBeNull();
    cleanup();
    setup({ main: 1, academy: level }, (g) => { activeVillage(g).recruitQueues.academy = [recruit(g, 'noble')]; });
    const hetman = scene('full').querySelector('[data-walker="command"]')!;
    expect(hetman.innerHTML).toContain('command-south');
    const [x, y] = LAYOUT.workers.hetman[level as 1 | 2 | 3];
    const s = LAYOUT.villagers.command;
    expect(hetman.getAttribute('transform')).toBe(`translate(${Math.round(x - s.anchor[0])}, ${Math.round(y - s.anchor[1])})`);
  });

  it.each([1, 8, 15])('a carpenter hammers in a level-%i workshop while it builds siege engines', (level) => {
    setup({ main: 1, workshop: level });
    expect(scene('full').querySelector('[data-walker="hammer"]')).toBeNull();
    cleanup();
    setup({ main: 1, workshop: level }, (g) => { activeVillage(g).recruitQueues.workshop = [recruit(g, 'ram')]; });
    const c = scene('full');
    const carpenter = c.querySelector('[data-walker="hammer"]')!;
    placed(carpenter, 'hammer', LAYOUT.workers.carpenter[sceneVariant(level)].at);
    expect(c.querySelector('[data-prop="engine"]')?.innerHTML).toContain('ram-build');
  });

  it('the half-built engine in the workshop yard is the one being built', () => {
    setup({ main: 1, workshop: 15 }, (g) => { activeVillage(g).recruitQueues.workshop = [recruit(g, 'catapult')]; });
    expect(scene('full').querySelector('[data-prop="engine"]')?.innerHTML).toContain('catapult-build');
    cleanup();
    setup({ main: 1, workshop: 15 });
    expect(scene('full').querySelector('[data-prop="engine"]')).toBeNull();
  });

  it.each([1, 2, 3] as const)('a stage-%i barracks drills its spearmen on its own ground', (stage) => {
    setup({ main: 1, barracks: levelFor('barracks', stage) }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, 'spear')]; });
    const c = scene('full');
    const drill = drillAt('spear', stage);
    placed(c.querySelector('[data-walker="thrust"]')!, 'thrust', drill.at);
    expect(c.querySelectorAll('[data-prop="dummy"]').length).toBe(1);
  });

  it('no drill with limited animations', () => {
    setup({ main: 1, barracks: 17 }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, 'archer')]; });
    expect(scene('limited').querySelector('[data-walker="shoot"], [data-prop="target"]')).toBeNull();
  });

  it.each(['scout', 'lightCav', 'mountedArcher', 'heavyCav'] as const)('a %s in training walks his horse by the stable, from stage 2 on', (unit) => {
    const train = (g: GameState) => { activeVillage(g).recruitQueues.stable = [recruit(g, unit)]; };
    setup({ main: 1, stable: levelFor('stable', 1) }, train);
    expect(scene('full').querySelector(`[data-walker="ride_${unit}"]`)).toBeNull();
    for (const stage of [2, 3] as const) {
      cleanup();
      setup({ main: 1, stable: levelFor('stable', stage) }, train);
      const rider = scene('full').querySelector(`[data-walker="ride_${unit}"]`)!;
      expect(rider.innerHTML).toContain(`ride_${unit}-east`);
      placed(rider, `ride_${unit}`, drillAt(unit, stage).at);
    }
  });

  it('the rider walks to the end of the paddock, turns and walks back', () => {
    setup({ main: 1, stable: levelFor('stable', 3) }, (g) => { activeVillage(g).recruitQueues.stable = [recruit(g, 'lightCav')]; });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const rider = scene('full').querySelector('[data-walker="ride_lightCav"]')!;
    const at = (seconds: number) => {
      ticks.at(-1)!(seconds * 1000);
      const [, x] = /translate\((-?[\d.]+),/.exec(rider.getAttribute('transform')!)!.map(Number);
      return { x: x!, href: rider.querySelector('image')!.getAttribute('href')! };
    };
    const { at: from, to } = drillAt('lightCav', 3);
    const leg = Math.hypot(to![0] - from[0], to![1] - from[1]) / RIDE_SPEED;
    const start = at(0);
    const out = at(leg / 2);
    expect(out.x).toBeGreaterThan(start.x);
    expect(out.href).toContain('ride_lightCav-east');
    const back = at(leg * 1.5);
    expect(back.href).toContain('ride_lightCav-west');
    expect(at(leg * 2 + 0.01).href).toContain('ride_lightCav-east');
    raf.mockRestore();
    now.mockRestore();
  });

  it('the arrow waits while the archer draws, flies to a target after the release and stays there until the next draw', () => {
    setup({ main: 1, barracks: 17 }, (g) => { activeVillage(g).recruitQueues.barracks = [recruit(g, 'archer')]; });
    const ticks: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => ticks.push(cb));
    const now = vi.spyOn(performance, 'now').mockReturnValue(0);
    const c = scene('full');
    const arrow = c.querySelector('[data-arrow]')!;
    const at = (step: number) => {
      ticks.at(-1)!(step * WORK_STEP_MS + 1);
      const [, x, y] = /translate\((-?[\d.]+), (-?[\d.]+)\)/.exec(arrow.getAttribute('transform') ?? 'translate(0, 0)')!.map(Number);
      return { shown: arrow.getAttribute('visibility') !== 'hidden', x: x!, y: y! };
    };
    const release = SHOOT_SEQ.indexOf(SHOOT_RELEASE_FRAME);
    const { from, to } = drillAt('archer', 3).arrow!;
    expect(at(0).shown).toBe(false);
    expect(at(release - 1).shown).toBe(false);
    const flying = at(release + 1);
    expect(flying.shown).toBe(true);
    expect(flying.x).toBeGreaterThan(from[0]);
    expect(flying.x).toBeLessThan(to[0]);
    const stuck = at(SHOOT_SEQ.length - 1);
    expect(stuck).toEqual({ shown: true, x: to[0], y: to[1] });
    expect(at(SHOOT_SEQ.length).shown).toBe(false);
    raf.mockRestore();
    now.mockRestore();
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
