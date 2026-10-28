import { create } from 'zustand';
import { pathFor } from '../../account/routes';
import { getLocale, setLocale } from '../../i18n/i18n';
import { ApiError, api, navigation } from '../../net/api';
import { connectLive } from '../../net/live';
import type { MeView, SpectatorState, WatchedVillage } from '../../net/types';

/** At most one request a second however many updates come; the poll covers changes only bots make. */
const MIN_GAP_MS = 1000;
const POLL_MS = 5000;

export interface WatchSelection { kind: 'village' | 'army'; id: string }

interface WatchStore {
  worldId: number | null;
  state: SpectatorState | null;
  /** Wall ms when `state` came, to move its clock on. */
  receivedAt: number;
  failed: boolean;
  selected: WatchSelection | null;
  /** The selected village in full, once it has come. */
  village: WatchedVillage | null;
  boot: (worldId: number) => void;
  refresh: () => Promise<void>;
  select: (s: WatchSelection | null) => void;
  stop: () => void;
}

let disconnect: (() => void) | null = null;
let poll: ReturnType<typeof setInterval> | undefined;
let trailing: ReturnType<typeof setTimeout> | undefined;
let inFlight = false;
let lastAt = 0;

const base = (id: number) => `/api/v1/worlds/${id}/watch`;

/** The account does not host the world (any more), or the world is gone: back to the lobby. */
function leftWorld(err: unknown, statuses: number[]): boolean {
  if (!(err instanceof ApiError) || !statuses.includes(err.status)) return false;
  navigation.assign(pathFor('lobby', getLocale()));
  return true;
}

/** The host's whole world, kept fresh by the live socket and a slow poll; nothing in it can be ordered. */
export const useWatchStore = create<WatchStore>((set, get) => ({
  worldId: null,
  state: null,
  receivedAt: 0,
  failed: false,
  selected: null,
  village: null,

  boot: (worldId) => {
    get().stop();
    set({ worldId, state: null, failed: false, selected: null, village: null });
    api.get<MeView>('/api/v1/me').then((me) => setLocale(me.locale), () => undefined);
    void get().refresh();
    disconnect = connectLive(worldId, (m) => { if (m.type === 'update') void get().refresh(); });
    poll = setInterval(() => void get().refresh(), POLL_MS);
  },

  refresh: async () => {
    const id = get().worldId;
    if (id === null) return;
    const wait = lastAt + MIN_GAP_MS - Date.now();
    if (inFlight || wait > 0) {
      trailing ??= setTimeout(() => { trailing = undefined; void get().refresh(); }, Math.max(wait, 50));
      return;
    }
    inFlight = true;
    lastAt = Date.now();
    try {
      const state = await api.get<SpectatorState>(base(id));
      if (get().worldId === id) set({ state, receivedAt: Date.now(), failed: false });
      const sel = get().selected;
      if (sel?.kind === 'village') await loadVillage(id, sel.id);
    } catch (err) {
      if (!leftWorld(err, [403, 404])) set({ failed: true });
    } finally {
      inFlight = false;
    }
  },

  select: (selected) => {
    set({ selected, village: null });
    const id = get().worldId;
    if (selected?.kind === 'village' && id !== null) void loadVillage(id, selected.id);
  },

  stop: () => {
    disconnect?.();
    disconnect = null;
    clearInterval(poll);
    clearTimeout(trailing);
    poll = trailing = undefined;
    inFlight = false;
    lastAt = 0;
    set({ worldId: null });
  },
}));

async function loadVillage(worldId: number, villageId: string): Promise<void> {
  try {
    const village = await api.get<WatchedVillage>(`${base(worldId)}/villages/${encodeURIComponent(villageId)}`);
    const { selected } = useWatchStore.getState();
    if (selected?.kind === 'village' && selected.id === villageId) useWatchStore.setState({ village });
  } catch (err) {
    leftWorld(err, [403]);
  }
}
