// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api, navigation, order } from '../src/net/api';
import { connectLive } from '../src/net/live';
import { MapCache } from '../src/net/mapCache';
import type { LiveMessage, MapTile } from '../src/net/types';

function respond(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('api', () => {
  it('a 409 with blockers becomes an ApiError with blockers', async () => {
    const blockers = [{ kind: 'resources', missing: { wood: 10 } }];
    vi.stubGlobal('fetch', vi.fn(async () => respond(409, { error: { code: 'blocked', message: 'Blocked', blockers } })));
    const err = await order(3, 'build', { villageId: 'v:1:1', building: 'main' }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 409, code: 'blocked', blockers });
    const [url, init] = vi.mocked(fetch).mock.calls[0]!;
    expect(url).toBe('/api/v1/worlds/3/orders/build');
    expect(init).toMatchObject({ method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } });
    expect(JSON.parse(String(init!.body))).toEqual({ villageId: 'v:1:1', building: 'main' });
  });

  it('a 401 sends the player to login', async () => {
    const go = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    vi.stubGlobal('fetch', vi.fn(async () => respond(401, { error: { code: 'unauthenticated', message: 'Log in' } })));
    await expect(api.get('/api/v1/me')).rejects.toMatchObject({ status: 401, code: 'unauthenticated' });
    expect(go).toHaveBeenCalledWith('/pl/logowanie');
  });

  it('returns the response, and a 204 as undefined', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => (url.endsWith('leave') ? respond(204) : respond(200, { ok: 1 }))));
    expect(await api.get('/x')).toEqual({ ok: 1 });
    expect(await api.post('/api/v1/worlds/1/leave', {})).toBeUndefined();
  });
});

class FakeSocket {
  static all: FakeSocket[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;
  closed = false;
  constructor(readonly url: string) { FakeSocket.all.push(this); }
  close() { this.closed = true; }
  emit(m: LiveMessage) { this.onmessage?.({ data: JSON.stringify(m) }); }
  drop(code = 1006) { this.onclose?.({ code }); }
}

describe('live connection', () => {
  beforeEach(() => {
    FakeSocket.all = [];
    vi.stubGlobal('WebSocket', FakeSocket);
    vi.useFakeTimers();
  });

  it('reconnects with a growing delay', () => {
    const got: LiveMessage[] = [];
    const stop = connectLive(7, (m) => got.push(m));
    expect(FakeSocket.all).toHaveLength(1);
    expect(FakeSocket.all[0]!.url).toMatch(/^ws:\/\/[^/]+\/api\/v1\/live\?world=7$/);
    FakeSocket.all[0]!.emit({ type: 'hello', serverTime: 1, worldNow: 2 });
    expect(got).toEqual([{ type: 'hello', serverTime: 1, worldNow: 2 }]);
    for (const [i, delay] of [1000, 2000, 4000, 8000, 15000, 15000].entries()) {
      FakeSocket.all[i]!.drop();
      vi.advanceTimersByTime(delay - 1);
      expect(FakeSocket.all).toHaveLength(i + 1);
      vi.advanceTimersByTime(1);
      expect(FakeSocket.all).toHaveLength(i + 2);
    }
    FakeSocket.all.at(-1)!.emit({ type: 'hello', serverTime: 1, worldNow: 2 });
    FakeSocket.all.at(-1)!.drop();
    vi.advanceTimersByTime(1000);
    expect(FakeSocket.all).toHaveLength(8);
    stop();
    expect(FakeSocket.all.at(-1)!.closed).toBe(true);
    FakeSocket.all.at(-1)!.drop(1000);
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.all).toHaveLength(8);
  });

  it('close code 4401 sends the player to login instead of reconnecting', () => {
    const go = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    connectLive(7, () => {});
    FakeSocket.all[0]!.drop(4401);
    vi.advanceTimersByTime(60_000);
    expect(go).toHaveBeenCalledWith('/pl/logowanie');
    expect(FakeSocket.all).toHaveLength(1);
  });
});

describe('map cache', () => {
  const tile = (x: number, y: number): MapTile => ({ x, y, id: `v:${x}:${y}`, name: 'Village', ownerId: null, points: 26 });

  function cache() {
    let clock = 0;
    const fetchChunk = vi.fn(async (x0: number, y0: number, x1: number, y1: number) => {
      const out: MapTile[] = [];
      for (const [x, y] of [[3, 4], [60, 4], [120, 130]] as const) if (x >= x0 && x <= x1 && y >= y0 && y <= y1) out.push(tile(x, y));
      return out;
    });
    const c = new MapCache(1, 200, fetchChunk, () => clock);
    return { c, fetchChunk, tick: (ms: number) => { clock += ms; } };
  }

  it('fetches each chunk once a minute', async () => {
    const { c, fetchChunk, tick } = cache();
    expect(c.tilesIn(0, 0, 70, 10)).toBeNull();
    expect(c.tileAt(3, 4)).toBeNull();
    await Promise.all([c.ensure(0, 0, 70, 10), c.ensure(10, 0, 60, 5)]);
    expect(fetchChunk.mock.calls).toEqual([[0, 0, 49, 49], [50, 0, 99, 49]]);
    expect(c.tilesIn(0, 0, 70, 10)!.map((t) => t.id)).toEqual(['v:3:4', 'v:60:4']);
    expect(c.tileAt(3, 4)).toMatchObject({ id: 'v:3:4' });
    expect(c.tileAt(5, 5)).toBe('empty');
    await c.ensure(0, 0, 49, 49);
    expect(fetchChunk).toHaveBeenCalledTimes(2);
    tick(60_000);
    await c.ensure(0, 0, 49, 49);
    expect(fetchChunk).toHaveBeenCalledTimes(3);
    await c.ensure(-20, -20, 10, 10);
    expect(fetchChunk).toHaveBeenCalledTimes(3);
  });

  it('invalidate fetches the chunk again and keeps the old data until then', async () => {
    const { c, fetchChunk } = cache();
    await c.ensure(100, 100, 149, 149);
    c.invalidate(['v:120:130']);
    expect(c.tileAt(120, 130)).toMatchObject({ id: 'v:120:130' });
    await c.ensure(100, 100, 149, 149);
    expect(fetchChunk).toHaveBeenCalledTimes(2);
    await c.ensure(100, 100, 149, 149);
    expect(fetchChunk).toHaveBeenCalledTimes(2);
  });
});
