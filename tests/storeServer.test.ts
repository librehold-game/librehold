// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { navigation } from '../src/net/api';
import type { LiveMessage, PlayerState } from '../src/net/types';
import { HUMAN_ID, type Report } from '../src/engine/types';
import { mergeNewestReports, useGameStore } from '../src/state/gameStore';
import { newTestGame, stateFixture } from './helpers';

class FakeSocket {
  static all: FakeSocket[] = [];
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;
  constructor(readonly url: string) { FakeSocket.all.push(this); }
  close() {}
  emit(m: LiveMessage) { this.onmessage?.({ data: JSON.stringify(m) }); }
}

type Handler = (body: unknown) => { status: number; body?: unknown };
let routes: Record<string, Handler>;
let calls: { key: string; body: unknown }[];
let fixture: PlayerState;

beforeEach(() => {
  window.history.replaceState({}, '', '/game/?world=3');
  fixture = stateFixture(newTestGame(), 'h7', 3);
  calls = [];
  routes = {
    'GET /api/v1/worlds/3/state': () => ({ status: 200, body: fixture }),
    'GET /api/v1/worlds/3/reports?limit=500': () => ({ status: 200, body: [] }),
    'GET /api/v1/worlds/3/news?limit=50': () => ({ status: 200, body: [] }),
  };
  FakeSocket.all = [];
  vi.stubGlobal('WebSocket', FakeSocket);
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ key, body });
    const r = routes[key]?.(body) ?? { status: 404, body: { error: { code: 'notFound', message: key } } };
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const own = () => Object.values(fixture.villages)[0]!;

describe('game store backed by the server', () => {
  it('boot builds the game state from the player view; the own player is human', async () => {
    await useGameStore.getState().boot();
    const g = useGameStore.getState().game!;
    expect(useGameStore.getState().phase).toBe('ready');
    expect(g.players[HUMAN_ID]).toMatchObject({ isHuman: true, buildQueueSlots: fixture.player.buildQueueSlots });
    expect(g.players.h7).toBeUndefined();
    expect(Object.values(g.villages).every((v) => v.ownerId === HUMAN_ID)).toBe(true);
    expect(g.villages[g.activeVillageId]).toBeDefined();
    expect(g.worldSeed).toBe(0);
    expect(g.clock.originLocalMs).toBe(fixture.clockOriginLocalMs);
    expect(FakeSocket.all[0]!.url).toContain('/api/v1/live?world=3');
  });

  it('the profile of player human asks for players/h7', async () => {
    await useGameStore.getState().boot();
    routes['GET /api/v1/worlds/3/players/h7'] = () => ({ status: 200, body: {
      player: { ...fixture.players.h7, id: 'h7' }, villages: [], history: { hourly: [], daily: [] } } });
    const profile = await useGameStore.getState().loadProfile(HUMAN_ID);
    expect(calls.at(-1)!.key).toBe('GET /api/v1/worlds/3/players/h7');
    expect(profile!.player.id).toBe(HUMAN_ID);
  });

  it('build sends the order and replaces the state', async () => {
    await useGameStore.getState().boot();
    const v = own();
    routes['POST /api/v1/worlds/3/orders/build'] = () => {
      const next = structuredClone(fixture);
      next.villages[v.id]!.name = 'After the build';
      return { status: 200, body: next };
    };
    const err = await useGameStore.getState().build(v.id, 'wood');
    expect(err).toBeNull();
    expect(calls.at(-1)).toEqual({ key: 'POST /api/v1/worlds/3/orders/build', body: { villageId: v.id, building: 'wood' } });
    expect(useGameStore.getState().game!.villages[v.id]!.name).toBe('After the build');
  });

  it('a 409 sets the description of the blockers', async () => {
    await useGameStore.getState().boot();
    routes['POST /api/v1/worlds/3/orders/build'] = () => ({ status: 409, body: { error: {
      code: 'blocked', message: 'Blocked', blockers: [{ kind: 'resources' }, { kind: 'queueFull' }] } } });
    const err = await useGameStore.getState().build(own().id, 'wood');
    expect(err).toBe('Nie można tego zrobić: za mało surowców, kolejka jest pełna');
    expect(useGameStore.getState().lastError).toBe('Nie można tego zrobić: za mało surowców, kolejka jest pełna');
  });

  it('an update on the reports topic fetches only the newest reports', async () => {
    await useGameStore.getState().boot();
    FakeSocket.all[0]!.emit({ type: 'hello', serverTime: 0, worldNow: 0 });
    routes['GET /api/v1/worlds/3/reports?limit=100'] = () => ({ status: 200, body: [] });
    calls = [];
    FakeSocket.all[0]!.emit({ type: 'update', topics: ['reports'], serverTime: 0, worldNow: 0 });
    await vi.waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]!.key).toBe('GET /api/v1/worlds/3/reports?limit=100');
    const rev = useGameStore.getState().rankingRev;
    FakeSocket.all[0]!.emit({ type: 'update', topics: ['ranking'], serverTime: 0, worldNow: 0 });
    expect(useGameStore.getState().rankingRev).toBe(rev + 1);
  });

  it('a new report merges the newest page into the whole inbox loaded at boot', async () => {
    routes['GET /api/v1/worlds/3/reports?limit=500'] = () => ({ status: 200, body: burst(150, 1) });
    await useGameStore.getState().boot();
    FakeSocket.all[0]!.emit({ type: 'hello', serverTime: 0, worldNow: 0 });
    routes['GET /api/v1/worlds/3/reports?limit=100'] = () => ({ status: 200, body: burst(151, 52) });
    FakeSocket.all[0]!.emit({ type: 'update', topics: ['reports'], serverTime: 0, worldNow: 0 });
    await vi.waitFor(() => expect(inbox()).toEqual(burst(151, 1).map((r) => r.id)));
  });

  it('marking one report read changes it in place; marking all reads the whole inbox again', async () => {
    routes['GET /api/v1/worlds/3/reports?limit=500'] = () => ({ status: 200, body: [report('r2', 2), report('r1', 1)] });
    routes['POST /api/v1/worlds/3/orders/mark-reports-read'] = () => ({ status: 200, body: fixture });
    routes['POST /api/v1/worlds/3/orders/mark-all-reports-read'] = () => ({ status: 200, body: fixture });
    await useGameStore.getState().boot();
    calls = [];
    expect(await useGameStore.getState().markReportRead('r1')).toBeNull();
    expect(useGameStore.getState().game!.reports.find((r) => r.id === 'r1')!.read).toBe(true);
    expect(calls.map((c) => c.key)).toEqual(['POST /api/v1/worlds/3/orders/mark-reports-read']);
    calls = [];
    await useGameStore.getState().markAllReportsRead(false);
    await vi.waitFor(() => expect(calls.map((c) => c.key)).toContain('GET /api/v1/worlds/3/reports?limit=500'));
  });

  it('a burst of state updates fetches the state once more after the fetch in flight, not once per update', async () => {
    await useGameStore.getState().boot();
    FakeSocket.all[0]!.emit({ type: 'hello', serverTime: 0, worldNow: 0 });
    const replies: (() => void)[] = [];
    let stateFetches = 0;
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url === '/api/v1/worlds/3/state') {
        stateFetches++;
        await new Promise<void>((resolve) => replies.push(resolve));
      }
      return new Response(JSON.stringify(fixture), { status: 200 });
    }));
    for (let i = 0; i < 5; i++) FakeSocket.all[0]!.emit({ type: 'update', topics: ['state'], serverTime: 0, worldNow: 0 });
    await vi.waitFor(() => expect(replies).toHaveLength(1));
    replies.shift()!();
    await vi.waitFor(() => expect(replies).toHaveLength(1));
    replies.shift()!();
    await new Promise((r) => setTimeout(r, 20));
    expect(stateFetches).toBe(2);
  });

  it('a 401 sends the player to the login page', async () => {
    const go = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    routes['GET /api/v1/worlds/3/state'] = () => ({ status: 401, body: { error: { code: 'unauthenticated', message: 'Log in' } } });
    await useGameStore.getState().boot();
    expect(go).toHaveBeenCalledWith('/pl/logowanie');
  });

  it('without a world in the address it goes back to the lobby', async () => {
    const go = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    window.history.replaceState({}, '', '/game/');
    await useGameStore.getState().boot();
    expect(go).toHaveBeenCalledWith('/pl/lobby');
  });

  it('the clock moves resources between server replies', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(1_800_000_000_000);
    await useGameStore.getState().boot();
    const v = own();
    const before = useGameStore.getState().game!.villages[v.id]!.resources.wood;
    vi.setSystemTime(1_800_000_000_000 + 60 * 60_000);
    useGameStore.getState().tick();
    const g = useGameStore.getState().game!;
    expect(g.now).toBe(fixture.now + 60 * 60_000);
    expect(g.villages[v.id]!.resources.wood).toBeGreaterThan(before);
  });
});

const report = (id: string, time: number) =>
  ({ id, time, read: false, origin: { ownerId: 'b1' }, target: { ownerId: 'b2' } }) as unknown as Report;
/** Reports rep{from} down to rep{to}, newest first, all in one tick like an auto-farm burst. */
const burst = (from: number, to: number) => Array.from({ length: from - to + 1 }, (_, i) => report(`rep${from - i}`, 7));
const inbox = () => [...useGameStore.getState().game!.reports].reverse().map((r) => r.id);

describe('mergeNewestReports', () => {
  const held = [report('r3', 3), report('r2', 2), report('r1', 1)];
  const ids = (reports: Report[] | null) => reports?.map((r) => r.id) ?? null;

  it('takes a page shorter than a full one as the whole inbox, so reports deleted elsewhere stay gone', () => {
    expect(ids(mergeNewestReports(held, [report('r4', 4), report('r2', 2)], 100))).toEqual(['r4', 'r2']);
    expect(mergeNewestReports(held, [], 100)).toEqual([]);
  });

  it('keeps the held reports older than a full page that holds the newest held report', () => {
    expect(ids(mergeNewestReports(held, [report('r5', 5), report('r4', 4), report('r3', 3)], 3))).toEqual(['r5', 'r4', 'r3', 'r2', 'r1']);
  });

  it('orders by id as the server does: a longer id is newer', () => {
    const tens = [report('rep10', 1), report('rep9', 1), report('rep8', 1)];
    expect(ids(mergeNewestReports(tens, [report('rep11', 1), report('rep10', 1)], 2))).toEqual(['rep11', 'rep10', 'rep9', 'rep8']);
  });

  it('reads the whole inbox again when a full page misses the newest held report, even at the same time', () => {
    const sameTick = [report('r3', 5), report('r2', 5), report('r1', 5)];
    expect(mergeNewestReports(sameTick, [report('r6', 5), report('r5', 5), report('r4', 5)], 3)).toBeNull();
  });
});
