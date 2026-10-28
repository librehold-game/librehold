import { vi } from 'vitest';
import { HUMAN_ID, type GameState } from '../src/engine/types';
import type { HostedView, InstantView, MapTile, MeView } from '../src/net/types';
import { fixtureOf, stateFixture } from './helpers';

export type StubRoute = (body: unknown) => { status: number; body?: unknown };

export interface StubOptions {
  me?: string;
  worldId?: number;
  paused?: boolean;
  /** The instant game's number in the state. */
  gameNumber?: number;
  /** The world is an instant one: the state carries this view. */
  instant?: InstantView;
  /** The world is a player-hosted one: the state carries its host. */
  hosted?: HostedView;
  /** The signed-in account behind GET /api/v1/me (404 when absent). */
  account?: MeView;
  /** Extra answers by "METHOD url", checked first. */
  routes?: Record<string, StubRoute>;
}

/**
 * Answers the game client's requests from a game, the way the server would for its human (`me`): state,
 * reports, news, rankings, profiles, the map and farm targets (those two from the world fixture the game was
 * loaded from, plus the villages a test added). The live socket is a dummy. Returns the requested paths.
 */
export function stubServer(g: GameState, opts: StubOptions = {}): string[] {
  const me = opts.me ?? HUMAN_ID;
  const worldId = opts.worldId ?? 1;
  window.history.replaceState({}, '', `/game/?world=${worldId}`);
  const id = (p: string) => (p === HUMAN_ID ? me : p);
  const back = (p: string) => (p === me ? HUMAN_ID : p);
  const bot = (p: string | null) => !!p && g.players[p]?.isHuman === false;
  const base = `/api/v1/worlds/${worldId}`;
  const playerRows = () => Object.values(g.players).filter((p) => p.status !== 'pending').map((p) => ({
    id: id(p.id), name: p.name, color: p.color, points: p.points, villages: p.villageIds.length,
    att: 0, def: 0, sup: 0, quit: p.status === 'quit', isBot: !p.isHuman,
  }));
  const mapTiles = (x0: number, y0: number, x1: number, y1: number): MapTile[] => {
    const tiles = new Map<string, MapTile>();
    const f = fixtureOf(g);
    for (const t of f?.map ?? []) tiles.set(t.id, { ...t, ownerId: t.ownerId === f!.state.me ? me : t.ownerId });
    for (const v of Object.values(g.villages)) {
      tiles.set(v.id, { id: v.id, x: v.x, y: v.y, name: v.name, points: v.points, ownerId: v.ownerId && id(v.ownerId),
        ...(bot(v.ownerId) ? { ownerIsBot: true } : {}), ...(v.bonus ? { bonus: v.bonus } : {}) });
    }
    return [...tiles.values()].filter((t) => t.x >= x0 && t.x <= x1 && t.y >= y0 && t.y <= y1);
  };
  const routes: Record<string, () => unknown> = {
    [`${base}/state`]: () => ({ ...stateFixture(g, me, worldId), paused: !!opts.paused, ...(opts.instant ? { instant: opts.instant } : {}),
      ...(opts.hosted ? { hosted: opts.hosted } : {}),
      ...(opts.gameNumber ? { gameNumber: opts.gameNumber } : {}) }),
    [`${base}/reports?limit=100`]: () => [...g.reports].reverse(),
    [`${base}/news?limit=50`]: () => [...g.news].reverse(),
    [`${base}/ranking?tab=points`]: playerRows,
    [`${base}/ranking?tab=villages`]: playerRows,
    [`${base}/ranking?tab=od`]: playerRows,
    [`${base}/ranking?tab=daily`]: () => [],
    [`${base}/ranking?tab=conquests`]: () => [],
  };
  const calls: string[] = [];
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    calls.push(key);
    const extra = opts.routes?.[key];
    if (extra) {
      const r = extra(init?.body ? JSON.parse(String(init.body)) : undefined);
      return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
    }
    if (url === '/api/v1/me' && opts.account) return new Response(JSON.stringify(opts.account), { status: 200 });
    if (key === 'POST /api/v1/reports') return new Response(JSON.stringify({ id: 1 }), { status: 201 });
    let body: unknown;
    const players = url.match(/\/players\/([^/?]+)$/);
    const farm = url.match(/\/farm-targets\?village=(.+)$/);
    const region = url.match(/\/map\?x0=(-?\d+)&y0=(-?\d+)&x1=(-?\d+)&y1=(-?\d+)$/);
    if (opts.paused && url.includes('/orders/')) {
      return new Response(JSON.stringify({ error: { code: 'worldPaused', message: 'World paused by an administrator' } }), { status: 423 });
    }
    if (routes[url]) body = routes[url]!();
    else if (players) {
      const p = g.players[back(decodeURIComponent(players[1]!))];
      if (!p) return new Response(JSON.stringify({ error: { code: 'notFound', message: 'Not found' } }), { status: 404 });
      body = {
        player: { id: id(p.id), name: p.name, color: p.color, status: p.status, points: p.points, villageIds: p.villageIds, isBot: !p.isHuman },
        villages: p.villageIds.map((v) => g.villages[v]!).map((v) => ({ id: v.id, name: v.name, x: v.x, y: v.y, points: v.points })),
        history: p.history,
      };
    } else if (farm) {
      body = decodeURIComponent(farm[1]!) === g.activeVillageId ? fixtureOf(g)?.farmTargets ?? [] : [];
    } else if (region) {
      const [x0, y0, x1, y1] = region.slice(1).map(Number) as [number, number, number, number];
      body = mapTiles(x0, y0, x1, y1);
    } else {
      return new Response(JSON.stringify({ error: { code: 'notFound', message: url } }), { status: 404 });
    }
    return new Response(JSON.stringify(body), { status: 200 });
  }));
  const sockets: unknown[] = [];
  (globalThis as unknown as { __sockets?: unknown[] }).__sockets = sockets;
  vi.stubGlobal('WebSocket', class { onmessage = null; onclose = null; constructor() { sockets.push(this); } close() {} });
  return calls;
}
