import { describe, expect, it } from 'vitest';
import { villageFarmCapacity, villageStorageCapacity } from '../src/engine/resources';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { INACTIVE_COLOR, TileArt, villageSize, villageTint } from '../src/ui/map/MapArt';
import { tileAt } from '../src/ui/map/tiles';
import { villageInfo, villageReports, type ForeignVillageInfo, type OwnVillageInfo } from '../src/ui/map/villageInfo';
import { reportVillageOf } from '../src/engine/reports';
import { activeVillage, addCommand, barbarianVillage, botVillage, loadFixture, newTestGame } from './helpers';

describe('village sizes on the map', () => {
  it('point thresholds: <300, 300, 1,000, 3,000, 9,000, 11,000', () => {
    expect(villageSize(0)).toBe(0);
    expect(villageSize(299)).toBe(0);
    expect(villageSize(300)).toBe(1);
    expect(villageSize(999)).toBe(1);
    expect(villageSize(1000)).toBe(2);
    expect(villageSize(2999)).toBe(2);
    expect(villageSize(3000)).toBe(3);
    expect(villageSize(8999)).toBe(3);
    expect(villageSize(9000)).toBe(4);
    expect(villageSize(10999)).toBe(4);
    expect(villageSize(11000)).toBe(5);
    expect(villageSize(1_000_000)).toBe(5);
  });

  it('barbarian villages the server sends are small huts', () => {
    const barbarians = loadFixture('default').map.filter((t) => t.ownerId === null);
    expect(barbarians.length).toBeGreaterThan(10);
    expect([...new Set(barbarians.map((t) => villageSize(t.points)))]).toEqual([0]);
  });
});

describe('village menu on the map (villageInfo)', () => {
  it('empty tile → null', () => {
    const g = newTestGame();
    let found = false;
    for (let x = 0; x < 30 && !found; x++) for (let y = 0; y < 30 && !found; y++) {
      if (tileAt(g, x, y).kind === 'empty') { expect(villageInfo(g, x, y)).toBeNull(); found = true; }
    }
    expect(found).toBe(true);
  });

  it('own village: resources, warehouse, population, build queue, incoming attacks', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    v.buildQueue.push({ id: 'b1', building: 'wood', targetLevel: v.buildings.wood + 1, kind: 'build', startedAt: g.now, completesAt: g.now + 600_000 });
    g.commands.push({ id: 'c-test-in', type: 'attack', originId: 'v:0:0', targetId: v.id, units: { axe: 10 }, departedAt: g.now, arrivesAt: g.now + 90_000 });
    g.commands.push({ id: 'c-test-in2', type: 'attack', originId: 'v:0:1', targetId: v.id, units: { axe: 5 }, departedAt: g.now, arrivesAt: g.now + 30_000 });
    const info = villageInfo(g, v.x, v.y) as OwnVillageInfo;
    expect(info.kind).toBe('own');
    expect(info.active).toBe(true);
    expect(info.resources).toEqual({ wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) });
    expect(info.storage).toBe(villageStorageCapacity(v));
    expect(info.popMax).toBe(villageFarmCapacity(v));
    expect(info.build?.building).toBe('wood');
    expect(info.build?.level).toBe(v.buildings.wood + 1);
    expect(info.queueLength).toBe(1);
    expect(info.incoming).toEqual({ count: 2, firstAt: g.now + 30_000 });
    expect(info.reportCount).toBe(0);
  });

  it('foreign village: distance and no intel; after an attack, a report, intel and the last attack time', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    const { x, y, id } = barbarianVillage(g);
    const before = villageInfo(g, x, y) as ForeignVillageInfo;
    expect(before.kind).toBe('foreign');
    expect(before.distance).toBeGreaterThan(0);
    expect(before.intel).toBeNull();
    expect(before.reports).toEqual([]);
    expect(before.ours).toEqual([]);
    expect(before.threat).toBeNull();

    const cmd = addCommand(g, { type: 'attack', originId: v.id, targetId: id, units: { spear: 120 } });
    const during = villageInfo(g, x, y) as ForeignVillageInfo;
    expect(during.ours).toEqual([{ id: cmd.id, kind: 'attack', arrivesAt: cmd.arrivesAt, units: 120 }]);

    // What the server sends after the battle: the command is gone, a report and intel arrived.
    g.commands = g.commands.filter((c) => c.id !== cmd.id);
    g.now = cmd.arrivesAt;
    g.reports.push({
      id: 'r1', time: cmd.arrivesAt, type: 'attack', originId: v.id, targetId: id, origin: reportVillageOf(g, v.id), target: reportVillageOf(g, id),
      attacker: { units: { spear: 120 }, losses: { spear: 4 } }, defender: { units: {}, losses: {} },
      luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    });
    const intel = { [id]: { intel: { at: cmd.arrivesAt, units: {}, wall: 0, loyalty: 100 }, lastFarmedAt: cmd.arrivesAt } };
    const after = villageInfo(g, x, y, g.activeVillageId, intel) as ForeignVillageInfo;
    expect(after.materialized).toBe(true);
    expect(after.reportCount).toBe(1);
    expect(after.reports[0]!.ours).toBe(true);
    expect(after.reports[0]!.type).toBe('attack');
    expect(after.ours).toEqual([]);
    expect(after.intel).not.toBeNull();
    expect(after.lastAttackAt).toBe(cmd.arrivesAt);
    expect(villageReports(g, after.id).map((r) => r.id)).toEqual([after.reports[0]!.id]);
    expect((villageInfo(g, v.x, v.y) as OwnVillageInfo).reportCount).toBe(1);
  });

  it('an AI attack from a foreign village on us shows up as a threat', () => {
    const g = newTestGame();
    const v = activeVillage(g);
    const { x, y, id } = barbarianVillage(g);
    g.commands.push({ id: 'c-ai', type: 'attack', originId: id, targetId: v.id, units: { axe: 40 }, departedAt: g.now, arrivesAt: g.now + 60_000 });
    const info = villageInfo(g, x, y) as ForeignVillageInfo;
    expect(info.threat).toEqual({ arrivesAt: g.now + 60_000, targetId: v.id, targetName: v.name, units: 40 });
    expect((villageInfo(g, v.x, v.y) as OwnVillageInfo).incoming).toEqual({ count: 1, firstAt: g.now + 60_000 });
  });
});

describe('player colours on the map', () => {
  it('a bot village tile carries the owner\'s colour and status', () => {
    const g = newTestGame('test-bot');
    const v = botVillage(g);
    const bot = g.players[v.ownerId!]!;
    const t = tileAt(g, v.x, v.y);
    expect(t.kind === 'village' && t.ownerColor).toBe(bot.color);
    expect(t.kind === 'village' && t.ownerStatus).toBe('active');
  });

  it('an inactive player\'s village is grey', () => {
    const g = newTestGame('test-bot');
    const v = botVillage(g);
    const bot = g.players[v.ownerId!]!;
    bot.status = 'quit';
    const t = tileAt(g, v.x, v.y);
    expect(villageTint(t)).toBe(INACTIVE_COLOR);
    const svg = renderToStaticMarkup(createElement(TileArt, {
      tw: 53, th: 38,
      village: { owner: 'player', size: villageSize(v.points), bonus: false, active: false, hidden: false, ownerColor: villageTint(t) },
    }));
    expect(svg).toContain(INACTIVE_COLOR);
    expect(svg).not.toContain(bot.color);
  });
});

