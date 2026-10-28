// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { navigation } from '../src/net/api';
import type { LiveMessage, PlayerState } from '../src/net/types';
import { HUMAN_ID } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
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
    'GET /api/v1/worlds/3/reports?limit=100': () => ({ status: 200, body: [] }),
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

  it('an update on the reports topic fetches only reports', async () => {
    await useGameStore.getState().boot();
    FakeSocket.all[0]!.emit({ type: 'hello', serverTime: 0, worldNow: 0 });
    calls = [];
    FakeSocket.all[0]!.emit({ type: 'update', topics: ['reports'], serverTime: 0, worldNow: 0 });
    await vi.waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]!.key).toBe('GET /api/v1/worlds/3/reports?limit=100');
    const rev = useGameStore.getState().rankingRev;
    FakeSocket.all[0]!.emit({ type: 'update', topics: ['ranking'], serverTime: 0, worldNow: 0 });
    expect(useGameStore.getState().rankingRev).toBe(rev + 1);
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
