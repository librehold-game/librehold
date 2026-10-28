import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import rules from './fixtures/rules.json';
import balance from '../src/data/balance.json';
import { buildingCost, farmCapacity, hideCapacity, storageCapacity } from '../src/engine/buildings';
import { travelTimeMs } from '../src/engine/commands';
import { loyaltyAt, noblesNeededFor } from '../src/engine/conquest';
import { checkBuild } from '../src/engine/queues';
import { villageProductionPerHour } from '../src/engine/resources';
import { nobleLimit, recruitTimeMs, researchBonus, researchCost } from '../src/engine/units';
import { HUMAN_ID, type BuildingId, type GameState, type Resource, type UnitId, type VillageBonusKind, type WorldConfig } from '../src/engine/types';
import { activeVillage, newTestGame } from './helpers';

// The browser's display helpers must give the server's answers: tests/fixtures/rules.json holds Go's outputs for a
// grid of inputs (written by `npm run gen`).

/** The test world's human village with the given world speed and side modifier. */
function human(worldSpeed: number, modifier: number): { g: GameState; cfg: WorldConfig } {
  const g = newTestGame();
  g.config = { ...g.config, worldSpeed, humanModifiers: { resources: modifier, recruitment: modifier, building: modifier } };
  const v = activeVillage(g);
  v.ownerId = HUMAN_ID;
  v.buildQueue = [];
  v.resources = { wood: 1e9, clay: 1e9, iron: 1e9 };
  return { g, cfg: g.config };
}

describe('display helpers give the server\'s answers', () => {
  it('travel time, to the millisecond', () => {
    for (const c of rules.travel) expect(travelTimeMs(c.dist, c.speedMinPerTile, c.unitSpeed), JSON.stringify(c)).toBe(c.ms);
  });

  it('production per hour', () => {
    for (const c of rules.production) {
      const { g, cfg } = human(c.worldSpeed, c.modifier);
      const v = activeVillage(g);
      v.buildings[c.resource as Resource] = c.level;
      v.bonus = c.bonus ? { kind: c.bonus as VillageBonusKind } : undefined;
      expect(villageProductionPerHour(v, c.resource as Resource, cfg), JSON.stringify(c)).toBeCloseTo(c.perHour, 9);
    }
  });

  it('storage, farm and hiding place capacity', () => {
    for (const c of rules.storage) {
      expect(storageCapacity(c.level)).toBe(c.value);
      expect(storageCapacity(c.level, { kind: 'storage' })).toBe(c.bonusValue);
    }
    for (const c of rules.farm) {
      expect(farmCapacity(c.level)).toBe(c.value);
      expect(farmCapacity(c.level, { kind: 'farm' })).toBe(c.bonusValue);
    }
    for (const c of rules.hide) expect(hideCapacity(c.level)).toBe(c.value);
  });

  it('building cost and time', () => {
    for (const c of rules.buildCost) {
      expect(buildingCost(c.building as BuildingId, c.level), JSON.stringify(c)).toEqual({ wood: c.wood, clay: c.clay, iron: c.iron });
    }
    for (const c of rules.buildTime) {
      const { g } = human(c.worldSpeed, c.modifier);
      const v = activeVillage(g);
      for (const b of Object.keys(v.buildings) as BuildingId[]) v.buildings[b] = 0;
      v.buildings.main = c.mainLevel;
      v.buildings[c.building as BuildingId] = c.level - 1;
      expect(checkBuild(g, v, c.building as BuildingId).durationMs, JSON.stringify(c)).toBe(c.ms);
    }
  });

  it('recruitment time and research cost', () => {
    for (const c of rules.recruitTime) {
      const { g, cfg } = human(c.worldSpeed, c.modifier);
      expect(recruitTimeMs(c.unit as UnitId, c.buildingLevel, cfg, activeVillage(g)), JSON.stringify(c)).toBe(c.ms);
    }
    for (const c of rules.researchCost) {
      expect(researchCost(c.unit as UnitId, c.level), JSON.stringify(c)).toEqual({ wood: c.wood, clay: c.clay, iron: c.iron });
    }
  });

  it('loyalty, nobles needed, noble limit and research bonus', () => {
    const { g } = human(1, 1);
    const v = activeVillage(g);
    for (const c of rules.loyalty) {
      v.loyalty = c.loyalty;
      v.loyaltyUpdatedAt = 0;
      expect(loyaltyAt(v, c.elapsedMs, c.worldSpeed), JSON.stringify(c)).toBe(c.value);
    }
    for (const c of rules.noblesNeeded) expect(noblesNeededFor(g.config, false, c.loyalty)).toBe(c.n);
    for (const c of rules.nobleLimit) {
      v.buildings.academy = c.academy;
      expect(nobleLimit(g.config, v)).toBe(c.n);
    }
    for (const c of rules.researchBonus) expect(researchBonus(c.level)).toBeCloseTo(c.bonus, 12);
  });
});

describe('the data files read the generated tables', () => {
  it('every table in balance.json is read by a data file', () => {
    const sources = readdirSync('src/data').filter((f) => f.endsWith('.ts')).map((f) => readFileSync(`src/data/${f}`, 'utf8')).join('\n');
    const imported = new Set([...sources.matchAll(/import\s*\{([^}]*)\}\s*from\s*'\.\/balance\.json'/g)]
      .flatMap((m) => m[1]!.split(',').map((s) => s.trim().split(/\s+as\s+/)[0]!)));
    expect(Object.keys(balance).filter((k) => !imported.has(k))).toEqual([]);
  });
});
