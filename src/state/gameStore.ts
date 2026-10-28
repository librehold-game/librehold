import type { Locale } from '../i18n/i18n';
import { setLocale as applyLocale, getLocale, t } from '../i18n/i18n';
import { pathFor } from '../account/routes';
import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';
import { errorMessage, newsText } from '../i18n/names';
import type { UnitCounts } from '../engine/commands';
import type { FarmTarget } from '../engine/farm';
import { humanVillageIds } from '../engine/players';
import { accrue } from '../engine/resources';
import { settleRecruitQueues } from '../engine/units';
import type {
  BuildingId, BuildTemplate, FarmFilters, GameState, Report, Resource, ResourceSet, UnitId, Village, VillageGroup,
} from '../engine/types';
import { ApiError, api, navigation, order } from '../net/api';
import { toClientId, toClientRef, toServerId } from '../net/ids';
import { connectLive } from '../net/live';
import { MapCache } from '../net/mapCache';
import { toGameState } from '../net/toGameState';
import type {
  ConquestRow, DailyRow, HostedView, InstantView, Intel, LiveMessage, MapTile, MeView, NewsEntry, PlayerRow, PlayerState, Profile,
} from '../net/types';
import { recallActiveVillage, rememberActiveVillage } from './uiStore';

export type LoadPhase = 'booting' | 'ready';
export type RankingTab = 'points' | 'villages' | 'od' | 'daily' | 'conquests';
export type RankingRows = PlayerRow[] | DailyRow[] | ConquestRow[];

export interface GameStore {
  /** The player's view of the world in the engine's shape; the server's word, ticked locally for display. */
  game: GameState | null;
  phase: LoadPhase;
  worldId: number | null;
  worldName: string;
  gameNumber: number | null;
  /** Bumped with every state from the server; views that fetch their own data refetch on it. */
  stateRev: number;
  /** What the player knows of other villages (scouting, farming), by village id. */
  intel: Record<string, Intel>;
  devMode: boolean;
  /** How far the last tick moved world time — the animation layer drops its beats after a jump. */
  lastSyncSpanMs: number;
  /** Grows with every tick and every new state, so the UI can tell them apart. */
  syncCounter: number;
  /** Bumped when rankings change; ranking views refetch on it. */
  rankingRev: number;
  /** Bumped when map tiles change; the map redraws on it. */
  mapRev: number;
  /** The instant world's clock and ending; null in a long world. */
  instant: InstantView | null;
  /** A player world's host and waiting room; null in every other world. */
  hosted: HostedView | null;
  /** The signed-in account; null until it is known. */
  account: MeView | null;
  /** The create-account dialog is open. */
  claimOpen: boolean;
  /** The player closed the results of an ended instant world to look around; the bar brings them back. */
  resultsHidden: boolean;
  setResultsHidden: (hidden: boolean) => void;
  openClaim: () => void;
  closeClaim: () => void;
  /** Turns the guest into a player with a name and a password; returns the error or null. */
  claimAccount: (username: string, password: string, passwordConfirm: string) => Promise<string | null>;
  /** A new nickname for the guest, in the account and this world; returns the error or null. */
  renameGuest: (username: string) => Promise<string | null>;
  finishTutorial: () => void;
  /** Quits the world for good and goes back to the account site; returns the error or null. */
  leaveWorld: () => Promise<string | null>;

  boot: () => Promise<void>;
  /** Stops the clock and the live connection (tests, leaving the world). */
  shutdown: () => void;
  tick: () => void;
  loadProfile: (playerId: string) => Promise<Profile | null>;
  loadRanking: (tab: RankingTab) => Promise<RankingRows | null>;
  /** New map tiles arrived: redraw the map. */
  mapLoaded: () => void;
  /** The farm assistant's targets around one of the player's villages, after their filters. */
  loadFarmTargets: (villageId: string) => Promise<FarmTarget[] | null>;
  /** Ends the session and leaves for the account site. */
  logout: () => Promise<void>;

  setActiveVillage: (id: string) => void;
  switchVillage: (delta: number) => void;
  claimQuestReward: (questId: string, villageId?: string) => Promise<string | null>;
  build: (villageId: string, building: BuildingId) => Promise<string | null>;
  demolish: (villageId: string, building: BuildingId) => Promise<string | null>;
  cancelBuildJob: (villageId: string, jobId: string) => Promise<string | null>;
  recruit: (villageId: string, unit: UnitId, count: number) => Promise<string | null>;
  cancelRecruitJob: (villageId: string, jobId: string) => Promise<string | null>;
  research: (villageId: string, unit: UnitId) => Promise<string | null>;
  cancelResearchJob: (villageId: string, jobId: string) => Promise<string | null>;
  send: (originId: string, x: number, y: number, units: UnitCounts, type: 'attack' | 'support', catapultTarget?: BuildingId) => Promise<string | null>;
  /** A send scheduled to the world millisecond (`mode`: send at / arrive at); returns an error or null. */
  scheduleSend: (originId: string, x: number, y: number, units: UnitCounts, type: 'attack' | 'support', mode: 'sendAt' | 'arriveAt', at: number, catapultTarget?: BuildingId) => Promise<string | null>;
  cancelScheduled: (id: string) => Promise<string | null>;
  /** Dodge an attack (move the troops out and back just after the hit); returns an error or null. */
  dodge: (commandId: string) => Promise<string | null>;
  /** Snipe a hetman train: support between the 1st and 2nd noble; returns an error or null. */
  snipe: (trainCommandIds: string[], fromVillageId: string, units: UnitCounts) => Promise<string | null>;
  cancelCmd: (commandId: string) => Promise<string | null>;
  /** Sends troops to meet another player's army on the road; returns an error or null. */
  intercept: (originId: string, commandId: string, units: UnitCounts, scout: boolean) => Promise<string | null>;
  withdraw: (hostId: string, homeId: string, units?: UnitCounts) => Promise<string | null>;
  setTemplate: (key: 'A' | 'B' | 'C' | 'D', units: UnitCounts) => Promise<string | null>;
  sendTemplate: (key: 'A' | 'B' | 'C' | 'D', x: number, y: number) => Promise<string | null>;
  lastError: string | null;
  clearError: () => void;
  notice: string | null;
  pushNotice: (text: string) => void;
  clearNotice: () => void;
  trade: (originId: string, targetId: string, amount: ResourceSet) => Promise<string | null>;
  acceptTraderOffer: (villageId: string) => Promise<string | null>;
  applyBuildTemplate: (villageId: string, templateId: string) => Promise<string | null>;
  clearPlan: (villageId: string) => Promise<string | null>;
  saveBuildTemplate: (tpl: BuildTemplate) => Promise<string | null>;
  deleteBuildTemplate: (id: string) => Promise<string | null>;
  setFarmFilters: (patch: Partial<FarmFilters>) => Promise<string | null>;
  setAutoFarm: (enabled: boolean, villageIds?: string[]) => Promise<string | null>;
  sendTemplateC: (x: number, y: number) => Promise<string | null>;
  saveGroup: (group: VillageGroup) => Promise<string | null>;
  deleteGroup: (id: string) => Promise<string | null>;
  /** Number of villages where a build was queued. */
  massBuild: (villageIds: string[], building: BuildingId) => Promise<number>;
  /** Number of units ordered. */
  massRecruit: (villageIds: string[], unit: UnitId, upTo: number) => Promise<number>;
  mint: (villageId: string) => Promise<string | null>;
  renameVillage: (villageId: string, name: string) => Promise<string | null>;
  /** Stores the player's language on the account; the switch itself already changed it locally. */
  setLocale: (locale: Locale) => void;
  markReportRead: (id: string, read?: boolean) => Promise<string | null>;
  markAllReportsRead: () => Promise<string | null>;
  deleteReadReports: () => Promise<string | null>;
  deleteReport: (id: string) => Promise<string | null>;
}

const TICK_MS = 250;

/** What the server last said: the state, when it arrived, and the reports and news fetched beside it. */
const server = {
  state: null as PlayerState | null,
  receivedAt: 0,
  reports: [] as Report[],
  news: [] as NewsEntry[],
  disconnect: null as (() => void) | null,
  ticker: null as ReturnType<typeof setInterval> | null,
  greeted: false,
  map: null as MapCache | null,
};
let bootPromise: Promise<void> | null = null;

/** The claim dialog opens by itself once per world: at a guest's first conquest there. */
function offerClaimOnce(worldId: number): boolean {
  const key = `claimOffered:${worldId}`;
  try {
    if (localStorage.getItem(key)) return false;
    localStorage.setItem(key, '1');
  } catch {
    if (claimOffered.has(key)) return false;
  }
  claimOffered.add(key);
  return true;
}
const claimOffered = new Set<string>();

/** The map tiles as loaded so far; null before boot. */
export function mapCache(): MapCache | null {
  return server.map;
}

/** The player's words for a refused request, from its code, values and blockers. */
export function errorText(e: unknown): string {
  if (e instanceof ApiError) return errorMessage(e.code, e.params, e.blockers);
  return e instanceof Error ? e.message : String(e);
}

function worldPath(path: string): string {
  return `/api/v1/worlds/${server.state?.worldId ?? useGameStore.getState().worldId}${path}`;
}

function recruitedUnits(g: GameState | null, ids: string[], unit: UnitId): number {
  let n = 0;
  for (const id of ids) {
    for (const q of Object.values(g?.villages[id]?.recruitQueues ?? {})) for (const j of q) if (j.unit === unit) n += j.total - j.done;
  }
  return n;
}

export const useGameStore = create<GameStore>()(
  immer((set, get) => {
    /** Rebuilds `game` from the server's last word, keeping the active village and settling to now. */
    const rebuild = () => {
      const st = server.state;
      if (!st) return;
      set((s) => {
        const prev = s.game;
        const game = toGameState(st, server.reports, server.news);
        const remembered = prev?.activeVillageId ?? recallActiveVillage(st.worldId);
        if (remembered && game.villages[remembered]) game.activeVillageId = remembered;
        s.game = game;
        s.intel = st.intel;
        s.worldName = st.worldName;
        s.gameNumber = st.gameNumber ?? null;
        s.stateRev += 1;
        s.syncCounter += 1;
        s.lastSyncSpanMs = prev ? Math.max(0, game.now - prev.now) : 0;
        s.instant = st.instant ?? null;
        s.hosted = st.hosted ?? null;
        if (prev && game.stats.villagesConquered > prev.stats.villagesConquered) {
          s.notice = t('notice.conquered');
          if (s.account?.isGuest && s.instant && offerClaimOnce(st.worldId)) s.claimOpen = true;
        } else if (prev && game.quests.completed.length > prev.quests.completed.length) s.notice = t('notice.questDone');
      });
      get().tick();
    };

    const applyState = (st: PlayerState) => {
      if (server.state && st.serverTime < server.state.serverTime) return;
      server.state = st;
      server.receivedAt = Date.now();
      rebuild();
    };

    const fetchState = async () => applyState(await api.get<PlayerState>(worldPath('/state')));
    const fetchReports = async () => {
      server.reports = await api.get<Report[]>(worldPath('/reports?limit=100'));
      rebuild();
    };
    const fetchNews = async () => {
      const before = new Set(server.news.map((n) => n.id));
      server.news = await api.get<NewsEntry[]>(worldPath('/news?limit=50'));
      const alert = server.news.find((n) => n.alert && !before.has(n.id));
      if (alert && before.size > 0) set((s) => { s.notice = `🗣️ ${newsText(alert)}`; });
      rebuild();
    };
    const quietly = (p: Promise<unknown>) => { p.catch(() => undefined); };

    const onLive = (m: LiveMessage) => {
      if (m.type === 'hello') {
        // The first hello follows the boot's own fetches; later ones come after a reconnect that may have
        // missed updates.
        if (server.greeted) {
          quietly(fetchState());
          quietly(fetchReports());
          quietly(fetchNews());
        }
        server.greeted = true;
        return;
      }
      if (m.type !== 'update') return;
      for (const topic of m.topics) {
        if (topic === 'state') quietly(fetchState());
        if (topic === 'reports') quietly(fetchReports());
        if (topic === 'news') quietly(fetchNews());
        if (topic === 'map') {
          server.map?.invalidate(m.villageIds ?? ['*']);
          set((s) => { s.mapRev += 1; });
        }
        if (topic === 'ranking') set((s) => { s.rankingRev += 1; });
      }
    };

    /** Sends an order and takes the state it returns; on refusal sets and returns the error text. */
    const run = async (name: string, body: object, notice?: string): Promise<string | null> => {
      const worldId = get().worldId;
      if (worldId === null) return t('notice.notLoaded');
      try {
        applyState(await order(worldId, name, body));
        if (notice) set((s) => { s.notice = notice; });
        return null;
      } catch (e) {
        const msg = errorText(e);
        set((s) => { s.lastError = msg; });
        return msg;
      }
    };
    const runThenReports = async (name: string, body: object) => {
      const err = await run(name, body);
      if (!err) quietly(fetchReports());
      return err;
    };
    const active = () => get().game?.activeVillageId ?? '';

    return {
      game: null,
      phase: 'booting',
      worldId: null,
      worldName: '',
      gameNumber: null,
      stateRev: 0,
      intel: {},
      devMode: typeof location !== 'undefined' && new URLSearchParams(location.search).get('dev') === '1',
      lastSyncSpanMs: 0,
      syncCounter: 0,
      rankingRev: 0,
      mapRev: 0,
      instant: null,
      hosted: null,
      account: null,
      claimOpen: false,
      resultsHidden: false,

      boot: () => {
        // StrictMode runs effects twice — one boot per app.
        bootPromise ??= (async () => {
          const worldId = Number(new URLSearchParams(window.location.search).get('world'));
          if (!Number.isInteger(worldId) || worldId <= 0) {
            navigation.assign(pathFor('lobby', getLocale()));
            return;
          }
          set((s) => { s.worldId = worldId; });
          quietly(api.get<MeView>('/api/v1/me').then((me) => {
            applyLocale(me.locale);
            set((s) => { s.account = me; });
          }));
          try {
            const [st, reports, news] = await Promise.all([
              api.get<PlayerState>(worldPath('/state')),
              api.get<Report[]>(worldPath('/reports?limit=100')),
              api.get<NewsEntry[]>(worldPath('/news?limit=50')),
            ]);
            server.reports = reports;
            server.news = news;
            server.map = new MapCache(worldId, st.config.mapSize, (x0, y0, x1, y1) =>
              api.get<MapTile[]>(worldPath(`/map?x0=${x0}&y0=${y0}&x1=${x1}&y1=${y1}`))
                .then((tiles) => tiles.map((t) => ({ ...t, ownerId: toClientRef(t.ownerId, st.me) }))));
            applyState(st);
            set((s) => { s.phase = 'ready'; });
          } catch (e) {
            if (e instanceof ApiError && e.status === 403) navigation.assign(pathFor('lobby', getLocale()));
            else if (!(e instanceof ApiError && e.status === 401)) set((s) => { s.lastError = errorText(e); });
            return;
          }
          server.disconnect = connectLive(worldId, onLive);
          server.ticker = setInterval(() => get().tick(), TICK_MS);
        })();
        return bootPromise;
      },

      shutdown: () => {
        server.disconnect?.();
        if (server.ticker) clearInterval(server.ticker);
        Object.assign(server, { state: null, receivedAt: 0, reports: [], news: [], disconnect: null, ticker: null, greeted: false, map: null });
        bootPromise = null;
        set((s) => {
          s.game = null;
          s.phase = 'booting';
          s.worldId = null;
          s.intel = {};
          s.instant = null;
          s.hosted = null;
          s.account = null;
          s.claimOpen = false;
          s.resultsHidden = false;
        });
      },

      // Display only: world time runs on from the last state and own villages produce and recruit on screen;
      // the server settles everything for real.
      tick: () => {
        const st = server.state;
        if (!st || !get().game) return;
        const now = st.paused ? st.now : st.now + Math.max(0, Date.now() - server.receivedAt);
        set((s) => {
          const g = s.game!;
          s.lastSyncSpanMs = Math.max(0, now - g.now);
          g.now = now;
          for (const v of Object.values(g.villages)) {
            accrue(v, now, g.config);
            settleRecruitQueues(v, now);
          }
          s.syncCounter += 1;
        });
      },

      loadProfile: async (playerId) => {
        const me = server.state?.me;
        if (!me) return null;
        try {
          const p = await api.get<Profile>(worldPath(`/players/${encodeURIComponent(toServerId(playerId, me))}`));
          return { ...p, player: { ...p.player, id: toClientId(p.player.id, me) } };
        } catch (e) {
          set((s) => { s.lastError = errorText(e); });
          return null;
        }
      },

      loadRanking: async (tab) => {
        const me = server.state?.me;
        if (!me) return null;
        try {
          const rows = await api.get<RankingRows>(worldPath(`/ranking?tab=${tab}`));
          if (tab === 'conquests') {
            return (rows as ConquestRow[]).map((r) => ({ ...r, toId: toClientId(r.toId, me), fromId: toClientRef(r.fromId, me) }));
          }
          return (rows as (PlayerRow | DailyRow)[]).map((r) => ({ ...r, id: toClientId(r.id, me) })) as RankingRows;
        } catch (e) {
          set((s) => { s.lastError = errorText(e); });
          return null;
        }
      },

      mapLoaded: () => set((s) => { s.mapRev += 1; }),

      loadFarmTargets: async (villageId) => {
        if (!server.state) return null;
        try {
          return await api.get<FarmTarget[]>(worldPath(`/farm-targets?village=${encodeURIComponent(villageId)}`));
        } catch {
          return null;
        }
      },

      openClaim: () => set((s) => { s.claimOpen = true; }),
      setResultsHidden: (hidden) => set((s) => { s.resultsHidden = hidden; }),
      closeClaim: () => set((s) => { s.claimOpen = false; }),
      claimAccount: async (username, password, passwordConfirm) => {
        try {
          const me = await api.post<MeView>('/api/v1/auth/claim', { username, password, passwordConfirm });
          set((s) => { s.account = me; s.claimOpen = false; });
          quietly(fetchState());
          return null;
        } catch (e) {
          return errorText(e);
        }
      },
      renameGuest: async (username) => {
        try {
          const me = await api.patch<MeView>('/api/v1/me', { username });
          set((s) => { s.account = me; });
          quietly(fetchState());
          return null;
        } catch (e) {
          return errorText(e);
        }
      },
      finishTutorial: () => {
        set((s) => { if (s.account) s.account.tutorialDone = true; });
        quietly(api.patch('/api/v1/me', { tutorialDone: true }));
      },
      leaveWorld: async () => {
        const worldId = get().worldId;
        if (worldId === null) return t('notice.notLoaded');
        try {
          await api.post(`/api/v1/worlds/${worldId}/leave`);
        } catch (e) {
          return errorText(e);
        }
        get().shutdown();
        navigation.assign(pathFor('home', getLocale()));
        return null;
      },

      logout: async () => {
        await api.post('/api/v1/auth/logout').catch(() => undefined);
        get().shutdown();
        navigation.assign(pathFor('home', getLocale()));
      },

      setActiveVillage: (id) => {
        const g = get().game;
        if (!g?.villages[id] || !humanVillageIds(g).includes(id)) return;
        set((s) => { s.game!.activeVillageId = id; });
        if (g && server.state) rememberActiveVillage(server.state.worldId, id);
      },

      switchVillage: (delta) => {
        const g = get().game;
        if (!g) return;
        const ids = humanVillageIds(g);
        const i = ids.indexOf(g.activeVillageId);
        const next = ids[((i < 0 ? 0 : i) + delta + ids.length * 1000) % ids.length];
        if (next) get().setActiveVillage(next);
      },

      claimQuestReward: (questId, villageId) =>
        run('claim-quest', { questId, villageId: villageId ?? active() }, '📜 Nagroda odebrana'),
      build: (villageId, building) => run('build', { villageId, building }),
      demolish: (villageId, building) => run('demolish', { villageId, building }),
      cancelBuildJob: (villageId, jobId) => run('cancel-build', { villageId, jobId }),
      recruit: (villageId, unit, count) => run('recruit', { villageId, unit, count }),
      cancelRecruitJob: (villageId, jobId) => run('cancel-recruit', { villageId, jobId }),
      research: (villageId, unit) => run('research', { villageId, unit }),
      cancelResearchJob: (villageId, jobId) => run('cancel-research', { villageId, jobId }),

      lastError: null,
      clearError: () => set((s) => { s.lastError = null; }),
      notice: null,
      pushNotice: (text) => set((s) => { s.notice = text; }),
      clearNotice: () => set((s) => { s.notice = null; }),

      send: (originId, x, y, units, type, catapultTarget) =>
        run('send', { originId, x, y, units, type, ...(catapultTarget ? { catapultTarget } : {}) }),
      scheduleSend: (originId, x, y, units, type, mode, at, catapultTarget) =>
        run('schedule-send', { originId, x, y, units, type, mode, at, ...(catapultTarget ? { catapultTarget } : {}) }),
      cancelScheduled: (id) => run('cancel-scheduled', { id }),
      dodge: (commandId) => run('dodge', { commandId }, t('notice.dodge')),
      snipe: (trainCommandIds, fromVillageId, units) =>
        run('snipe', { commandIds: trainCommandIds, fromVillageId, units }, t('notice.snipe')),
      cancelCmd: (commandId) => run('cancel-command', { commandId }),
      intercept: (originId, commandId, units, scout) => run('intercept', { originId, commandId, units, ...(scout ? { scout } : {}) }),
      withdraw: (hostId, homeId, units) => run('withdraw-support', { hostId, homeId, ...(units ? { units } : {}) }),
      setTemplate: (key, units) => run('set-farm-template', { key, units }),
      sendTemplate: (key, x, y) => run('send-farm-template', { originId: active(), key, x, y }),

      trade: (originId, targetId, amount) => run('trade', { originId, targetId, amount }),
      acceptTraderOffer: (villageId) => run('accept-trader', { villageId }),
      applyBuildTemplate: (villageId, templateId) => run('apply-build-template', { villageId, templateId }),
      clearPlan: (villageId) => run('clear-build-plan', { villageId }),
      saveBuildTemplate: (tpl) => run('save-build-template', tpl),
      deleteBuildTemplate: (id) => run('delete-build-template', { id }),
      setFarmFilters: (patch) => run('set-farm-filters', patch),
      setAutoFarm: (enabled, villageIds) =>
        run('set-auto-farm', { enabled, villageIds: villageIds ?? get().game?.farmAssistant.autoFarmVillages ?? [] }),
      sendTemplateC: (x, y) => run('send-template-c', { originId: active(), x, y }),
      saveGroup: (group) => run('save-group', group),
      deleteGroup: (id) => run('delete-group', { id }),

      massBuild: async (villageIds, building) => {
        const queued = (g: GameState | null) => villageIds.reduce((n, id) => n + (g?.villages[id]?.buildQueue.length ?? 0), 0);
        const before = queued(get().game);
        await run('mass-build', { villageIds, building });
        return Math.max(0, queued(get().game) - before);
      },
      massRecruit: async (villageIds, unit, upTo) => {
        const before = recruitedUnits(get().game, villageIds, unit);
        await run('mass-recruit', { villageIds, unit, upTo });
        return Math.max(0, recruitedUnits(get().game, villageIds, unit) - before);
      },

      mint: (villageId) => run('mint', { villageId }),
      renameVillage: (villageId, name) => run('rename-village', { villageId, name }),
      setLocale: (locale) => { void api.patch('/api/v1/me', { locale }).catch(() => undefined); },
      markReportRead: (id, read = true) => runThenReports('mark-reports-read', { ids: [id], read }),
      markAllReportsRead: () => runThenReports('mark-all-reports-read', {}),
      deleteReadReports: () => runThenReports('delete-read-reports', {}),
      deleteReport: (id) => runThenReports('delete-report', { id }),
    };
  }),
);

// ---- Helper selectors ----

export const selectGame = (s: GameStore): GameState => {
  if (!s.game) throw new Error('the game is not loaded');
  return s.game;
};

export const selectActiveVillage = (s: GameStore): Village => {
  const g = selectGame(s);
  const v = g.villages[g.activeVillageId];
  if (!v) throw new Error('no active village');
  return v;
};

export const selectResource = (r: Resource) => (s: GameStore): number => selectActiveVillage(s).resources[r];

