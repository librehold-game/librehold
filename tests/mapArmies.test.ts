import { describe, it, expect } from 'vitest';
import { CROWD, armiesOnMap, armyPosition, crowdOf, figuresFor, groundAt, type Ground, type MapArmy } from '../src/ui/map/armies';
import { BIOME, SUB_X, SUB_Y, biomeAt, biomeGrid, rawVertex, tileBiome, type Biome } from '../src/ui/map/biomes';
import type { Command, GameState } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';

function cmd(g: GameState, c: Partial<Command> & Pick<Command, 'id' | 'type' | 'originId' | 'targetId'>): Command {
  const full: Command = { units: {}, departedAt: g.now, arrivesAt: g.now + 60_000, ...c };
  g.commands.push(full);
  return full;
}

function world() {
  const g = newTestGame();
  const home = activeVillage(g);
  const bot = makeBotVillage(g);
  const bot2 = makeBotVillage(g);
  return { g, home, bot, bot2 };
}

const kinds = (as: MapArmy[]) => Object.fromEntries(as.map((a) => [a.id, a.kind]));

describe('armies on the map', () => {
  it('armiesOnMap classifies own, incoming and trade commands', () => {
    const { g, home, bot, bot2 } = world();
    cmd(g, { id: 'a', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    cmd(g, { id: 's', type: 'support', originId: home.id, targetId: bot.id, units: { spear: 5 } });
    cmd(g, { id: 'r', type: 'return', originId: bot.id, targetId: home.id, units: { axe: 8 } });
    cmd(g, { id: 'ia', type: 'attack', originId: bot.id, targetId: home.id });
    cmd(g, { id: 'is', type: 'support', originId: bot.id, targetId: home.id });
    cmd(g, { id: 't', type: 'trade', originId: home.id, targetId: bot.id, merchants: 2 });
    const far = makeBotVillage(g, home.x + 30, home.y + 30);
    const far2 = makeBotVillage(g, home.x + 40, home.y + 30);
    cmd(g, { id: 'x', type: 'attack', originId: far.id, targetId: far2.id });
    void bot2;
    const armies = armiesOnMap(g);
    expect(kinds(armies)).toEqual({ a: 'ownAttack', s: 'ownSupport', r: 'ownReturn', ia: 'incomingAttack', is: 'incomingSupport', t: 'trade' });
    const byId = Object.fromEntries(armies.map((a) => [a.id, a]));
    expect(byId.a!.units).toEqual({ axe: 10 });
    expect(byId.ia!.units).toBeNull();
    expect(byId.is!.units).toBeNull();
    expect(byId.a!.from).toEqual({ x: home.x + 0.5, y: home.y + 0.5 });
    expect(byId.a!.to).toEqual({ x: bot.x + 0.5, y: bot.y + 0.5 });
  });

  it('places field armies: own intercepts, threats to my armies, passing armies and returns from the road', () => {
    const { g, home, bot, bot2 } = world();
    cmd(g, { id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    cmd(g, { id: 'oi', type: 'intercept', originId: home.id, targetId: '', point: { x: home.x + 2, y: home.y }, units: { lightCav: 5 } });
    cmd(g, { id: 'th', type: 'intercept', originId: bot.id, targetId: '', point: { x: home.x + 1, y: home.y }, units: { axe: 9 } });
    cmd(g, { id: 'pa', type: 'attack', originId: bot.id, targetId: bot2.id, units: { axe: 9 } });
    g.config.sightRadius = 3;
    cmd(g, { id: 'rf', type: 'return', originId: '', targetId: home.id, point: { x: home.x + 3, y: home.y }, units: { axe: 4 } });
    const byId = Object.fromEntries(armiesOnMap(g).map((a) => [a.id, a]));
    expect(byId.oi!.kind).toBe('ownIntercept');
    expect(byId.oi!.units).toEqual({ lightCav: 5 });
    expect(byId.oi!.to).toEqual({ x: home.x + 2.5, y: home.y + 0.5 });
    expect(byId.th!.kind).toBe('threat');
    expect(byId.th!.units).toBeNull();
    expect(byId.pa!.kind).toBe('passingAttack');
    expect(byId.pa!.units).toBeNull();
    expect(byId.rf!.kind).toBe('ownReturn');
    expect(byId.rf!.from).toEqual({ x: home.x + 3.5, y: home.y + 0.5 });
  });

  it('a passing army out of sight of every own village is not shown', () => {
    const { g, home } = world();
    const far = makeBotVillage(g, home.x + 30, home.y + 30);
    const far2 = makeBotVillage(g, home.x + 40, home.y + 30);
    cmd(g, { id: 'pa', type: 'attack', originId: far.id, targetId: far2.id });
    expect(armiesOnMap(g)).toEqual([]);
    g.config.sightRadius = 50;
    expect(armiesOnMap(g).map((a) => a.kind)).toEqual(['passingAttack']);
  });

  it('incoming attack never carries units even if the state has them', () => {
    const { g, home, bot } = world();
    cmd(g, { id: 'ia', type: 'attack', originId: bot.id, targetId: home.id, units: { axe: 100 } });
    expect(armiesOnMap(g)[0]!.units).toBeNull();
  });

  it('armiesOnMap skips commands with unknown endpoints', () => {
    const { g, home } = world();
    const off = g.config.mapSize + 5;
    cmd(g, { id: 'far', type: 'attack', originId: `v:${off}:${off}`, targetId: home.id });
    cmd(g, { id: 'far2', type: 'attack', originId: home.id, targetId: 'nowhere', units: { axe: 1 } });
    expect(() => armiesOnMap(g)).not.toThrow();
    expect(armiesOnMap(g)).toEqual([]);
  });

  it('a foreign village the client does not hold is placed by its id', () => {
    const { g, home, bot } = world();
    cmd(g, { id: 'a', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    cmd(g, { id: 'ia', type: 'attack', originId: bot.id, targetId: home.id });
    delete g.villages[bot.id];
    const byId = Object.fromEntries(armiesOnMap(g).map((a) => [a.id, a]));
    expect(byId.a!.to).toEqual({ x: bot.x + 0.5, y: bot.y + 0.5 });
    expect(byId.ia!.kind).toBe('incomingAttack');
    expect(byId.ia!.from).toEqual({ x: bot.x + 0.5, y: bot.y + 0.5 });
  });

  const line: MapArmy = { id: 'l', kind: 'ownAttack', originId: 'o', targetId: 't', from: { x: 0, y: 0 }, to: { x: 10, y: 0 }, departedAt: 0, arrivesAt: 1000, units: { axe: 1 } };

  it('armyPosition moves linearly', () => {
    expect(armyPosition(line, 250)).toEqual({ x: 2.5, y: 0, p: 0.25 });
  });

  it('armyPosition clamps before departure and after arrival', () => {
    expect(armyPosition(line, -50)).toEqual({ x: 0, y: 0, p: 0 });
    expect(armyPosition(line, 5000)).toEqual({ x: 10, y: 0, p: 1 });
  });

  it('figuresFor is one per unit up to 24, then grows slowly', () => {
    expect(figuresFor(0)).toBe(0);
    expect(figuresFor(1)).toBe(1);
    expect(figuresFor(24)).toBe(24);
    expect(figuresFor(50)).toBe(41);
    expect(figuresFor(5000)).toBe(147);
  });

  it('crowdOf keeps every present type and respects the cap', () => {
    const crowd = crowdOf({ axe: 5000, lightCav: 3000, noble: 1 }, 240);
    const f = Object.fromEntries(crowd.map((c) => [c.unit, c.figures]));
    expect(crowd.reduce((s, c) => s + c.figures, 0)).toBeLessThanOrEqual(240);
    expect(f.noble).toBeGreaterThanOrEqual(1);
    expect(f.axe).toBeGreaterThan(f.lightCav!);
    expect(crowdOf({ axe: 5000, spear: 0 }).map((c) => c.unit)).toEqual(['axe']);
    expect(CROWD.maxPerArmy).toBe(240);
  });

  it('groundAt follows the drawn terrain, not just the tile centre', () => {
    const seed = 777;
    let probe: [number, number] | null = null;
    const g = biomeGrid(seed, 3, 2, 450, 300, () => false);
    for (let vy = 2; vy < 300 && !probe; vy++) for (let vx = 3; vx < 450 && !probe; vx++) {
      const tx = Math.floor(vx / SUB_X), ty = Math.floor(vy / SUB_Y);
      if (biomeAt(g, vx, vy) === BIOME.water && tileBiome(seed, tx, ty) === BIOME.grass) probe = [vx, vy];
    }
    expect(probe).not.toBeNull();
    expect(groundAt(seed, probe![0] / SUB_X, probe![1] / SUB_Y)).toBe('water');
  });

  it('groundAt is the biome the map draws, without the specks and edges the drawing smooths away', () => {
    const seed = 777;
    const g = biomeGrid(seed, 0, 0, 300, 200, () => false);
    const name: Record<number, Ground> = { [BIOME.grass]: 'meadow', [BIOME.water]: 'water', [BIOME.forest]: 'forest', [BIOME.mountain]: 'mountain' };
    let smoothed = 0;
    for (let vy = 0; vy < 200; vy += 3) for (let vx = 0; vx < 300; vx += 2) {
      if (rawVertex(seed, vx, vy).biome !== biomeAt(g, vx, vy)) smoothed++;
      expect(groundAt(seed, vx / SUB_X, vy / SUB_Y)).toBe(name[biomeAt(g, vx, vy)]);
    }
    expect(smoothed).toBeGreaterThan(0);
  });

  it('groundAt keeps a village clearing on land', () => {
    const seed = 777;
    let lake: [number, number] | null = null;
    for (let y = 1; y < 200 && !lake; y++) for (let x = 1; x < 200 && !lake; x++) {
      if (groundAt(seed, x + 0.5, y + 0.5) === 'water') lake = [x, y];
    }
    expect(lake).not.toBeNull();
    const [x, y] = lake!;
    const isVillage = (vx: number, vy: number) => vx === x && vy === y;
    expect(groundAt(seed, x + 0.5, y + 0.5, isVillage)).toBe('meadow');
  });

  it('groundAt maps biomes', () => {
    const seed = 777;
    const find = (b: Biome): [number, number] => {
      for (let y = 0; y < 200; y++) for (let x = 0; x < 200; x++) if (tileBiome(seed, x, y) === b) return [x, y];
      throw new Error(`no biome ${b}`);
    };
    const cases: [Biome, Ground][] = [[BIOME.water, 'water'], [BIOME.forest, 'forest'], [BIOME.mountain, 'mountain'], [BIOME.grass, 'meadow']];
    for (const [b, ground] of cases) {
      const [x, y] = find(b);
      expect(groundAt(seed, x + 1 / SUB_X, y + 1 / SUB_Y)).toBe(ground);
    }
  });
});
