import type { LeaderRow } from '../api';
import type { TimelineFile } from './types';

export interface TimelapseOwner { playerId: string; name: string /* '' = former player */; color: string; bot: boolean; rank: number }

export interface Timelapse { mapSize: number; endAt: number; owners: TimelapseOwner[]; barbarians: [number, number][]; events: number[][] }

export interface Frame {
  t: number;
  villages: { x: number; y: number; owner: number }[];
  flashes: { x: number; y: number; age: number }[];
  lines: { fx: number; fy: number; x: number; y: number; size: number; age: number }[];
  legend: { owner: number; villages: number }[];
}

const VILLAGE = 0, OWNER = 1, BATTLE = 2;
const LEGEND_SIZE = 8;

/** The timeline with each owner's name and final rank from the game's results; a purged player has no name. */
export function buildTimelapse(file: TimelineFile, rows: LeaderRow[]): Timelapse {
  const byId = new Map(rows.map((r) => [r.playerId, r]));
  const owners = file.owners.map((o) => {
    const r = byId.get(o.playerId);
    return { playerId: o.playerId, name: r?.name ?? '', color: o.color, bot: !!o.bot, rank: r?.rank ?? Infinity };
  });
  return { mapSize: file.mapSize, endAt: file.endAt, owners, barbarians: file.barbarians, events: file.events };
}

/** The map at world time t: every village with its owner then, the conquests and battles of the last effectMs. */
export function frameAt(tl: Timelapse, t: number, effectMs: number): Frame {
  const villages = new Map<string, { x: number; y: number; owner: number }>();
  for (const [x, y] of tl.barbarians) villages.set(`${x},${y}`, { x, y, owner: 0 });
  const flashes: Frame['flashes'] = [];
  const lines: Frame['lines'] = [];
  for (const e of tl.events) {
    const at = e[0]!;
    if (at > t) break;
    const age = (t - at) / effectMs;
    const recent = t - at < effectMs;
    if (e[1] === VILLAGE || e[1] === OWNER) {
      const [x, y, owner] = [e[2]!, e[3]!, e[4]!];
      villages.set(`${x},${y}`, { x, y, owner });
      if (e[1] === OWNER && recent) flashes.push({ x, y, age });
    } else if (e[1] === BATTLE && recent) {
      lines.push({ fx: e[2]!, fy: e[3]!, x: e[4]!, y: e[5]!, size: e[8]!, age });
    }
  }
  const counts = new Map<number, number>();
  for (const v of villages.values()) if (v.owner > 0) counts.set(v.owner, (counts.get(v.owner) ?? 0) + 1);
  const rank = (owner: number) => tl.owners[owner - 1]?.rank ?? Infinity;
  const legend = [...counts].map(([owner, n]) => ({ owner, villages: n }))
    .sort((a, b) => b.villages - a.villages || rank(a.owner) - rank(b.owner))
    .slice(0, LEGEND_SIZE);
  return { t, villages: [...villages.values()], flashes, lines, legend };
}
