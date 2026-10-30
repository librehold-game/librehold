import { describe, expect, it } from 'vitest';
import { BUILDINGS, BUILDING_ORDER } from '../src/data/buildings.data';
import { BASE_TIMING, requirementChain, researchRows, unlocksOf } from '../src/shared/encyclopedia/facts';
import type { BuildingId } from '../src/engine/types';

const levels = (over: Partial<Record<BuildingId, number>>) =>
  Object.fromEntries(BUILDING_ORDER.map((b) => [b, over[b] ?? 0])) as Record<BuildingId, number>;
const flat = (u: { kind: string; id: string; level: number }[]) => u.map((x) => `${x.kind}:${x.id}:${x.level}`);

describe('requirementChain', () => {
  it('without village levels lists the whole chain with nothing marked', () => {
    const [barracks, smithy] = requirementChain(BUILDINGS.stable.requirements);
    expect(barracks).toMatchObject({ building: 'barracks', level: 5, have: null, met: false });
    expect(barracks!.children.map((c) => c.building)).toEqual(['main']);
    expect(smithy).toMatchObject({ building: 'smithy', level: 5, have: null, met: false });
    expect(smithy!.children.map((c) => `${c.building}:${c.level}`)).toEqual(['main:5', 'barracks:1']);
  });

  it('with village levels marks met and missing, expanding only what is missing', () => {
    const [barracks, smithy] = requirementChain(BUILDINGS.stable.requirements, levels({ main: 4, barracks: 3 }));
    expect(barracks).toMatchObject({ building: 'barracks', level: 5, have: 3, met: false });
    expect(barracks!.children).toEqual([{ building: 'main', level: 3, have: 4, met: true, children: [] }]);
    expect(smithy).toMatchObject({ building: 'smithy', level: 5, have: 0, met: false });
    expect(smithy!.children.map((c) => `${c.building}:${c.have}:${c.met}`)).toEqual(['main:4:false', 'barracks:3:true']);
  });

  it('a met requirement has no children', () => {
    const [main] = requirementChain([{ building: 'main', level: 3 }], levels({ main: 3 }));
    expect(main).toMatchObject({ have: 3, met: true, children: [] });
  });
});

describe('unlocksOf', () => {
  it('the stable recruits the mounted units', () => {
    const u = unlocksOf('stable');
    expect(flat(u)).toEqual(['unit:scout:1', 'unit:lightCav:3', 'unit:mountedArcher:5', 'unit:heavyCav:10']);
    expect(u.every((x) => x.recruitedHere)).toBe(true);
  });

  it('the smithy gates units and buildings it does not recruit', () => {
    const u = unlocksOf('smithy');
    expect(u).toContainEqual({ kind: 'unit', id: 'axe', level: 1, recruitedHere: false });
    expect(u).toContainEqual({ kind: 'unit', id: 'scout', level: 5, recruitedHere: false });
    expect(u).toContainEqual({ kind: 'building', id: 'stable', level: 5, recruitedHere: false });
    expect(u).toContainEqual({ kind: 'building', id: 'academy', level: 20, recruitedHere: false });
  });

  it('a building nothing needs unlocks nothing', () => {
    for (const b of ['wood', 'clay', 'iron', 'farm', 'hide'] as const) expect(unlocksOf(b), b).toEqual([]);
  });
});

describe('researchRows', () => {
  it('applies the research bonus to attack and the three defences', () => {
    const rows = researchRows('lightCav', BASE_TIMING);
    expect(rows.map((r) => r.attack)).toEqual([130, Math.round(130 * 1.25), 182]);
    expect(rows[1]).toMatchObject({ defense: Math.round(30 * 1.25), defenseCavalry: 50, defenseArcher: Math.round(30 * 1.25), bonus: 0.25 });
    expect(rows[0]!.cost).toEqual({ wood: 1250, clay: 1000, iron: 2500 });
    expect(rows[1]!.cost).toEqual({ wood: 3750, clay: 3000, iron: 7500 });
    expect(rows.some((r) => r.free)).toBe(false);
  });

  it('units that need no unlock research say level 1 is free', () => {
    for (const u of ['spear', 'ram', 'catapult', 'paladin'] as const) {
      expect(researchRows(u, BASE_TIMING)[0]!.free, u).toBe(true);
      expect(researchRows(u, BASE_TIMING)[1]!.free, u).toBe(false);
    }
  });

  it('the noble is never researched', () => {
    expect(researchRows('noble', BASE_TIMING)).toEqual([]);
  });
});
