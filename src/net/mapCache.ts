import { api } from './api';
import type { MapTile } from './types';

const CHUNK = 50;
const TTL_MS = 60_000;

interface Chunk {
  fetchedAt: number;
  tiles: Map<string, MapTile>; // by "x:y"
}

type FetchChunk = (x0: number, y0: number, x1: number, y1: number) => Promise<MapTile[]>;

const tileKey = (x: number, y: number) => `${x}:${y}`;

/**
 * The map as far as it is loaded: 50×50 chunks aligned to multiples of 50, fetched on demand and kept for a
 * minute. An invalidated chunk keeps showing its old tiles until it is fetched again.
 */
export class MapCache {
  private chunks = new Map<string, Chunk>();
  private inflight = new Map<string, Promise<boolean>>();

  constructor(
    worldId: number,
    private readonly mapSize: number,
    private readonly fetchChunk: FetchChunk = (x0, y0, x1, y1) =>
      api.get<MapTile[]>(`/api/v1/worlds/${worldId}/map?x0=${x0}&y0=${y0}&x1=${x1}&y1=${y1}`),
    private readonly now: () => number = () => Date.now(),
  ) {}

  private chunkKeys(x0: number, y0: number, x1: number, y1: number): [number, number][] {
    const last = this.mapSize - 1;
    const [ax, ay, bx, by] = [Math.max(0, x0), Math.max(0, y0), Math.min(last, x1), Math.min(last, y1)];
    const out: [number, number][] = [];
    for (let cx = Math.floor(ax / CHUNK); cx <= Math.floor(bx / CHUNK); cx++) {
      for (let cy = Math.floor(ay / CHUNK); cy <= Math.floor(by / CHUNK); cy++) out.push([cx, cy]);
    }
    return out;
  }

  /** Loads every chunk of the rectangle that is missing or older than a minute; true when anything was fetched. */
  async ensure(x0: number, y0: number, x1: number, y1: number): Promise<boolean> {
    const loaded = await Promise.all(this.chunkKeys(x0, y0, x1, y1).map(([cx, cy]) => this.load(cx, cy)));
    return loaded.some(Boolean);
  }

  private load(cx: number, cy: number): Promise<boolean> {
    const key = `${cx}:${cy}`;
    const have = this.chunks.get(key);
    if (have && this.now() - have.fetchedAt < TTL_MS) return Promise.resolve(false);
    const pending = this.inflight.get(key);
    if (pending) return pending;
    const x0 = cx * CHUNK, y0 = cy * CHUNK;
    const p = this.fetchChunk(x0, y0, Math.min(this.mapSize - 1, x0 + CHUNK - 1), Math.min(this.mapSize - 1, y0 + CHUNK - 1))
      .then((tiles) => {
        this.chunks.set(key, { fetchedAt: this.now(), tiles: new Map(tiles.map((t) => [tileKey(t.x, t.y), t])) });
        return true;
      })
      .finally(() => this.inflight.delete(key));
    this.inflight.set(key, p);
    return p;
  }

  /** The villages of the rectangle, or null while any of its chunks is not loaded. */
  tilesIn(x0: number, y0: number, x1: number, y1: number): MapTile[] | null {
    const out: MapTile[] = [];
    for (const [cx, cy] of this.chunkKeys(x0, y0, x1, y1)) {
      const chunk = this.chunks.get(`${cx}:${cy}`);
      if (!chunk) return null;
      for (const t of chunk.tiles.values()) if (t.x >= x0 && t.x <= x1 && t.y >= y0 && t.y <= y1) out.push(t);
    }
    return out.sort((a, b) => a.x - b.x || a.y - b.y);
  }

  /** The village at a tile, 'empty' for none, or null when its chunk is not loaded. */
  tileAt(x: number, y: number): MapTile | 'empty' | null {
    const chunk = this.chunks.get(`${Math.floor(x / CHUNK)}:${Math.floor(y / CHUNK)}`);
    if (!chunk) return null;
    return chunk.tiles.get(tileKey(x, y)) ?? 'empty';
  }

  /** Marks the chunks of these villages (ids `v:x:y`) for fetching again; an id of another shape, all chunks. */
  invalidate(villageIds: string[]) {
    for (const id of villageIds) {
      const m = /^v:(\d+):(\d+)$/.exec(id);
      if (!m) {
        for (const c of this.chunks.values()) c.fetchedAt = -Infinity;
        return;
      }
      const c = this.chunks.get(`${Math.floor(Number(m[1]) / CHUNK)}:${Math.floor(Number(m[2]) / CHUNK)}`);
      if (c) c.fetchedAt = -Infinity;
    }
  }
}
