import { describe, expect, it } from 'vitest';
import type { LeaderRow } from '../src/account/api';
import { buildTimelapse, frameAt } from '../src/account/timelapse/model';
import { advance, effectMsFor, initialPlayback } from '../src/account/timelapse/playback';
import type { TimelineFile } from '../src/account/timelapse/types';

const MIN = 60_000;
const END = 60 * MIN;

const file: TimelineFile = {
  v: 1, mapSize: 40, endAt: END,
  owners: [{ playerId: 'h1', color: '#ff0000' }, { playerId: 'bot_1', color: '#00ff00', bot: true }, { playerId: 'h2', color: '#0000ff' }],
  barbarians: [[1, 1], [2, 2], [3, 3], [4, 4]],
  events: [
    [0, 0, 10, 10, 1], [0, 0, 20, 20, 2], [5 * MIN, 0, 30, 30, 3],
    [10 * MIN, 1, 2, 2, 1],
    [20 * MIN, 2, 10, 10, 20, 20, 1, 2, 3],
  ],
};
const row = (playerId: string, name: string, rank: number, isBot = false): LeaderRow =>
  ({ rank, playerId, name, isBot, points: 0, villages: 0, conquered: 0, killed: 0 });
const rows = [row('h1', 'Ala', 1), row('bot_1', 'Wilk', 2, true), row('h2', 'Ola', 3)];
const at = (f: ReturnType<typeof frameAt>, x: number, y: number) => f.villages.find((v) => v.x === x && v.y === y)?.owner;

describe('timelapse model', () => {
  it('buildTimelapse joins names and ranks from the leaderboard', () => {
    const tl = buildTimelapse(file, rows);
    expect(tl.owners.map((o) => [o.name, o.rank, o.bot])).toEqual([['Ala', 1, false], ['Wilk', 2, true], ['Ola', 3, false]]);
    expect(tl.mapSize).toBe(40);
    expect(tl.barbarians).toHaveLength(4);
  });

  it('frameAt at 0 shows barbarians and spawns only', () => {
    const f = frameAt(buildTimelapse(file, rows), 0, MIN);
    expect(f.villages).toHaveLength(6);
    expect(at(f, 10, 10)).toBe(1);
    expect(at(f, 2, 2)).toBe(0);
    expect(at(f, 30, 30)).toBeUndefined();
    expect(f.flashes).toEqual([]);
    expect(f.lines).toEqual([]);
  });

  it('at 10 min the village has its new owner and flashes', () => {
    const f = frameAt(buildTimelapse(file, rows), 10 * MIN + 15_000, MIN);
    expect(at(f, 2, 2)).toBe(1);
    expect(f.flashes).toEqual([{ x: 2, y: 2, age: 0.25 }]);
  });

  it('the flash is gone after effectMs', () => {
    const f = frameAt(buildTimelapse(file, rows), 11 * MIN, MIN);
    expect(f.flashes).toEqual([]);
    expect(at(f, 2, 2)).toBe(1);
  });

  it("the battle line has the event's size", () => {
    const f = frameAt(buildTimelapse(file, rows), 20 * MIN, MIN);
    expect(f.lines).toEqual([{ fx: 10, fy: 10, x: 20, y: 20, size: 3, age: 0 }]);
  });

  it('legend lists at most eight owners by villages, ties by final rank', () => {
    const owners = Array.from({ length: 10 }, (_, i) => ({ playerId: `p${i}`, color: '#000000' }));
    const events = owners.flatMap((_, i) => Array.from({ length: i % 3 === 0 ? 2 : 1 }, (_, k) => [0, 0, i, k, i + 1]));
    const tl = buildTimelapse({ ...file, owners, events, barbarians: [] }, owners.map((o, i) => row(o.playerId, o.playerId, 10 - i)));
    const legend = frameAt(tl, 0, MIN).legend;
    expect(legend).toHaveLength(8);
    expect(legend.slice(0, 4).map((l) => l.owner)).toEqual([10, 7, 4, 1]);
    expect(legend.slice(0, 4).every((l) => l.villages === 2)).toBe(true);
    expect(legend.slice(4).map((l) => l.owner)).toEqual([9, 8, 6, 5]);
  });

  // Review focus 5: villages only, and a player purged since the game.
  it('frameAt handles a game without battles and a former player', () => {
    const quiet = { ...file, events: file.events.filter((e) => e[1] !== 2) };
    const tl = buildTimelapse(quiet, [row('h1', 'Ala', 1), row('bot_1', 'Wilk', 2, true)]);
    expect(tl.owners[2]).toEqual({ playerId: 'h2', name: '', color: '#0000ff', bot: false, rank: Infinity });
    const f = frameAt(tl, END, MIN);
    expect(f.lines).toEqual([]);
    expect(at(f, 30, 30)).toBe(3);
    expect(f.legend.map((l) => l.owner)).toContain(3);
  });
});

describe('timelapse playback', () => {
  it('advance plays the whole game in the chosen length', () => {
    const p = { playing: true, length: 30 as const, t: 0 };
    const half = advance(p, 15_000, END);
    expect(half.t).toBe(END / 2);
    expect(half.playing).toBe(true);
    const done = advance(p, 31_000, END);
    expect(done.t).toBe(END);
    expect(done.playing).toBe(false);
    expect(effectMsFor(30, END)).toBe(END * 0.6 / 30);
  });

  it('reduced motion opens paused', () => {
    expect(initialPlayback(true)).toEqual({ playing: false, length: 60, t: 0 });
    expect(initialPlayback(false)).toEqual({ playing: true, length: 60, t: 0 });
  });
});
