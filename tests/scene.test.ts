import { describe, it, expect } from 'vitest';
import { MS_PER_HOUR, MS_PER_MINUTE } from '../src/engine/types';
import { animationBudget, citizenCount, cycleMs, dayPhase, mergeBeat, sceneModel, trainingFigures, troopFigures, variantFor, wallGuards, woodSwingMs } from '../src/ui/scene/sceneModel';
import { activeVillage, newTestGame } from './helpers';

describe('village illustration model (16.6)', () => {
  it('formulas: cycle 1200–6000 ms, villagers 2–18, drill ground log2, troops logarithmic, guards ceil(wall/5)', () => {
    expect(cycleMs(100)).toBe(1200);
    expect(cycleMs(3000)).toBe(3000);
    expect(cycleMs(99_999)).toBe(6000);
    expect(citizenCount(240)).toBe(4);
    expect(citizenCount(24_000)).toBe(18);
    expect(citizenCount(0)).toBe(2);
    expect(trainingFigures(1)).toBe(2);
    expect(trainingFigures(1000)).toBe(8);
    expect(troopFigures(0)).toBe(0);
    expect(troopFigures(10)).toBeLessThan(troopFigures(10_000));
    expect(troopFigures(10_000)).toBeLessThanOrEqual(8);
    expect(wallGuards(0)).toBe(0);
    expect(wallGuards(5)).toBe(1);
    expect(wallGuards(6)).toBe(2);
    expect(variantFor(0)).toBe(0); expect(variantFor(10, 30)).toBe(1); expect(variantFor(11, 30)).toBe(2); expect(variantFor(21, 30)).toBe(3);
    expect(variantFor(6, 20)).toBe(1); expect(variantFor(7, 20)).toBe(2); expect(variantFor(14, 20)).toBe(3); expect(variantFor(1, 1)).toBe(3);
    expect(woodSwingMs(30)).toBeGreaterThan(woodSwingMs(1000));
  });

  it('day cycle from world time', () => {
    expect(dayPhase(6 * MS_PER_HOUR)).toBe('dawn');
    expect(dayPhase(12 * MS_PER_HOUR)).toBe('day');
    expect(dayPhase(20 * MS_PER_HOUR)).toBe('dusk');
    expect(dayPhase(23 * MS_PER_HOUR)).toBe('night');
  });

  it('beats: 800 ms throttling merges into a group of 3–5', () => {
    const b1 = mergeBeat(undefined, 'barracks', 1000, 1);
    expect(b1.count).toBe(1);
    const b2 = mergeBeat(b1, 'barracks', 1500, 1);
    expect(b2.at).toBe(1000);
    expect(b2.count).toBe(3);
    const b3 = mergeBeat(b2, 'barracks', 2500, 1);
    expect(b3.at).toBe(2500);
    expect(b3.count).toBe(1);
  });

  it('village time of day follows the local clock, not the time since the world was created', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    g.clock.originLocalMs = Date.UTC(2026, 9, 19, 12, 0, 0) - g.now;
    expect(sceneModel(g, v.id)!.dayPhase).toBe('day');
    g.clock.originLocalMs = Date.UTC(2026, 9, 19, 23, 0, 0) - g.now;
    expect(sceneModel(g, v.id)!.dayPhase).toBe('night');
  });

  it('model reads game state: scaffolding, drill ground, alarm, 30-element budget', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    v.buildings.main = 10; v.buildings.barracks = 5; v.buildings.wall = 12; v.buildings.farm = 10;
    v.resources = { wood: 50_000, clay: 50_000, iron: 50_000 };
    v.buildQueue.push({ id: 'b1', building: 'wood', targetLevel: 1, kind: 'build', startedAt: g.now, completesAt: g.now + 10 * MS_PER_MINUTE });
    v.recruitQueues.barracks.push({ id: 'r1', unit: 'spear', total: 40, done: 0, perUnitMs: 2 * MS_PER_MINUTE, startedAt: g.now, completesAt: g.now + 80 * MS_PER_MINUTE });
    g.now += MS_PER_MINUTE;
    const m = sceneModel(g, v.id)!;
    expect(m.construction).toHaveLength(1);
    expect(m.construction[0]!.building).toBe('wood');
    expect(m.construction[0]!.ratio).toBeGreaterThan(0);
    expect(m.barracks.active).toBe(true);
    expect(m.barracks.unit).toBe('spear');
    expect(m.barracks.figures).toBeGreaterThanOrEqual(2);
    expect(m.guards).toBe(3);
    expect(m.alarm).toBe(false);
    const b = animationBudget(m, 'full');
    const total = b.citizens + b.troops + b.guards;
    expect(total).toBeLessThanOrEqual(30);
    expect(animationBudget(m, 'limited').citizens).toBe(0);
    expect(animationBudget(m, 'off')).toEqual({ citizens: 0, weather: false, troops: 0, guards: 0 });
  });

  it('budget counts only what the pixel scene draws, so a busy mature village keeps its walkers', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    Object.assign(v.buildings, { main: 20, barracks: 20, smithy: 15, wall: 20, wood: 15, iron: 15, farm: 15, statue: 1 });
    const base = sceneModel(g, v.id)!;
    const m = {
      ...base, citizens: 18, guards: 4, troopFigures: 8, ironSmoke: 3, clayActive: true, woodSwingMs: 800,
      barracks: { ...base.barracks, active: true, figures: 8 }, smithy: { ...base.smithy, active: true, twoForges: true },
      main: { messengers: 2 }, statue: { active: true, knightHome: true },
    };
    const b = animationBudget(m, 'full');
    expect(b.guards).toBe(2);
    expect(b.troops).toBe(8);
    expect(b.citizens).toBeGreaterThanOrEqual(12);
    expect(b.citizens + b.troops + b.guards).toBeLessThanOrEqual(30);
  });

  it('alarm: an AI attack < 10 min away hides the villagers', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    const enemy = { ...v, id: 'v:1:1', x: 1, y: 1, ownerId: null, units: { ...v.units }, resources: { ...v.resources }, buildings: { ...v.buildings } };
    g.villages[enemy.id] = enemy;
    g.commands.push({ id: 'c_x', type: 'attack', originId: enemy.id, targetId: v.id, units: { axe: 100 }, departedAt: g.now, arrivesAt: g.now + 5 * MS_PER_MINUTE });
    const m = sceneModel(g, v.id)!;
    expect(m.alarm).toBe(true);
    expect(m.alarmEtaMs).toBe(5 * MS_PER_MINUTE);
    expect(m.citizens).toBe(0);
  });
});

describe('map terrain (cosmetic)', () => {
  it('is deterministic from the seed and gives coherent forest/water patches in sensible proportions', async () => {
    const { terrainAt, tileVariant } = await import('../src/ui/map/terrain');
    const counts: Record<string, number> = {};
    let same = 0;
    for (let x = 0; x < 120; x++) for (let y = 0; y < 120; y++) {
      const t = terrainAt(777, x, y);
      counts[t] = (counts[t] ?? 0) + 1;
      if (x > 0 && terrainAt(777, x - 1, y) === t) same++;
      expect(terrainAt(777, x, y)).toBe(t);
    }
    const total = 120 * 120;
    expect((counts.forest ?? 0) / total).toBeGreaterThan(0.1);
    expect((counts.forest ?? 0) / total).toBeLessThan(0.45);
    expect((counts.water ?? 0) / total).toBeLessThan(0.15);
    expect(same / total).toBeGreaterThan(0.65);
    expect(terrainAt(778, 5, 5) === terrainAt(777, 5, 5) && terrainAt(778, 50, 50) === terrainAt(777, 50, 50) && terrainAt(778, 90, 10) === terrainAt(777, 90, 10)).toBe(false);
    const v = tileVariant(777, 3, 4, 3);
    expect(v.variant).toBeGreaterThanOrEqual(0);
    expect(v.variant).toBeLessThan(3);
  });
});
