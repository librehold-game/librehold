import type { ReportInput } from '../net/report';
import type { WorldConfig } from './worldConfig';
import { getLocale, hasKey, t, type Locale } from '../i18n/i18n';

export interface Account {
  id: number; username: string; isAdmin: boolean; isGuest: boolean; locale: Locale; tutorialDone: boolean;
}
export interface WorldCard { id: number; name: string; gameNumber?: number | null; humans: number; bots: number; paused: boolean; settings: WorldConfig }
export interface LobbyView {
  mine: WorldCard[]; open: WorldCard[];
  /** The player worlds the account plays in, its own hosted world, what it may host and the games it hosted. */
  playing?: WorldCard[]; hosted?: HostedCard | null; host?: HostLimits; pastHosted?: GameRef[];
}
/** A player-hosted world's settings as the host chooses them; the server holds each to its range. */
export interface HostForm {
  name: string; durationMin: number; seats: number; bots: number; worldSpeed: number; unitSpeed: number;
  startVillage: 'grown' | 'fresh'; barbarians: 'few' | 'normal' | 'many'; difficulty: string; botStance: string;
  fieldBattles: boolean; sightRadius: number; protectionMin: number; dominationPct: number; lateJoinMin: number;
  registeredOnly: boolean;
}
export interface HostLimits { canHost: boolean; maxSeats: number; maxBots: number; full: boolean }
export interface HostSettings { maxSeats: number; maxBots: number }
export interface HostedPlayer { id: string; name: string; isGuest: boolean }
export interface HostedCard {
  worldId: number; name: string; status: 'waiting' | 'running' | 'paused' | 'ended'; humans: number; seats: number; bots: number;
  inviteCode: string; resultsCode?: string; registeredOnly: boolean; pauseEndsAt?: number; pauseLeftMs: number;
  players: HostedPlayer[]; settings: WorldConfig;
}
export type HostAction = 'start' | 'pause' | 'resume' | 'end' | 'kick';
export interface AdminAccount extends Account { createdAt: string }
export interface AdminWorld {
  id: number; name: string; status: 'open' | 'closed'; humans: number; bots: number; paused: boolean; startedAt: number;
  settings: WorldConfig;
}
export type Quadrant = 'random' | 'nw' | 'ne' | 'sw' | 'se';

/** Where Play puts the player: a world to open, or how long until a seat frees up. */
export interface PlayResult { worldId?: number; waitMs?: number }
/** A running instant game in the list a player picks from; `endsAt` is wall ms. */
export interface InstantGame {
  id: number; gameNumber: number; endsAt: number; humans: number; seats: number; leader: string; leaderPoints: number; mine: boolean;
}
export interface InstantList { games: InstantGame[]; canStart: boolean; waitMs?: number }
export interface PublicEvent {
  at: number; kind: 'conquest' | 'battle'; worldId: number; worldName: string; gameNumber?: number; instant: boolean;
  attacker: string; defender: string; attackerBot: boolean; defenderBot: boolean; troops: number; x: number; y: number;
}
export interface PublicWorldCard {
  id: number; name: string; gameNumber?: number; instant: boolean; status: 'open' | 'closed'; paused: boolean;
  startedAt: number; endsAt?: number; day?: number; humans: number; bots: number; speed: number; openSeats?: number;
}
export interface FinishedGameCard { worldId: number; name: string; gameNumber?: number; endedAt: number }
export interface PublicWorldList { running: PublicWorldCard[]; long: PublicWorldCard[]; finished: FinishedGameCard[] }
export interface PublicSettings {
  worldSpeed: number; unitSpeed: number; mapSize: number; beginnerProtectionDays: number; lastVillage: string;
  fieldBattles: boolean; durationMin?: number;
}
export interface PublicRankRow { rank: number; name: string; isBot: boolean; points: number; villages: number }
export interface PublicWorld extends PublicWorldCard { ended: boolean; settings: PublicSettings; top: PublicRankRow[]; events: PublicEvent[] }
export interface PublicMapPlayer { name: string; color: string; isBot: boolean }
/** Villages as [x, y, index into players or -1 for barbarians]. */
export interface PublicMap { size: number; players: PublicMapPlayer[]; villages: [number, number, number][] }
export interface ProblemReport {
  id: number; createdAt: string; kind: string; text: string; email: string; accountId?: number; username: string;
  worldId?: number; worldName: string; screen: string; build: string; userAgent: string; status: 'new' | 'read' | 'done';
  closedAt?: string;
}
export interface LiveStrip { games: number; humans: number; bots: number; openSeats: number; lastBattleAt: number | null; events: PublicEvent[] }
export interface StatTotals { games: number; players: number; conquests: number; unitsKilled: number }
export interface PublicStats {
  visible: boolean; allTime: StatTotals; last24h: StatTotals; open?: { seats: number; closesInMs: number };
}
export interface ArchiveSummary {
  endReason: 'time' | 'domination' | 'abandoned'; winnerId: string; humans: number; guests: number; bots: number; conquestsHumans: number;
  conquestsBots: number; attacksSent: number; humanUnitsKilled: number; humanUnitsLost: number; largestBattleUnits: number;
}
/** A line of a finished instant game's leaderboard; a former player has an empty name. */
export interface LeaderRow {
  rank: number; playerId: string; name: string; isBot: boolean; accountId?: number; points: number; villages: number;
  conquered: number; killed: number;
}
export interface ArchivedGame {
  worldId: number; name: string; gameNumber?: number | null; startedAt: number; endedAt: number; winnerName: string;
  winnerIsBot: boolean;
  /** A live instant game: no winner yet, `endsAt` is when it ends (wall ms). */
  running?: boolean;
  endsAt?: number;
  summary: ArchiveSummary; rows: LeaderRow[];
}
export interface GameRef {
  worldId: number; name: string; gameNumber?: number | null; endedAt: number; rank: number; players: number; points: number; conquered: number; killed: number;
  /** A player world's game is reached by its results code. */
  kind?: string; code?: string;
}
export interface PublicProfile {
  username: string; isGuest: boolean; tally: { games: number; wins: number; conquered: number; killed: number }; games: GameRef[];
}

/** A refused request: a code with the values its message needs, worded by errorText. */
export class ApiError extends Error {
  constructor(readonly status: number, readonly code: string, readonly params?: Record<string, unknown>, readonly field?: string) {
    super(code);
  }
}

export async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin' };
  if (method !== 'GET') {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body ?? {});
  }
  let res: Response;
  try {
    res = await fetch(path, init);
  } catch {
    throw new ApiError(0, 'network');
  }
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const e = data?.error ?? {};
    throw new ApiError(res.status, e.code ?? 'internal', e.params, e.field);
  }
  return data as T;
}

/** The reader's words for an error from the server. */
export function errorText(err: unknown): string {
  if (!(err instanceof ApiError)) return t('errors.unknown');
  if (err.status === 429 && !hasKey(`errors.${err.code}`)) return t('errors.tooManyAttempts');
  if (err.status === 401 && err.code !== 'unauthenticated') return t('errors.badCredentials');
  return hasKey(`errors.${err.code}`) ? t(`errors.${err.code}`, err.params as Record<string, string | number> | undefined) : t('errors.unknown');
}

export const api = {
  me: () => call<Account>('GET', '/api/v1/me'),
  updateMe: (patch: { locale?: Locale; tutorialDone?: boolean; username?: string }) => call<Account>('PATCH', '/api/v1/me', patch),
  guest: () => call<{ account: Account }>('POST', '/api/v1/auth/guest', { locale: getLocale() }),
  claim: (username: string, password: string, passwordConfirm: string) =>
    call<Account>('POST', '/api/v1/auth/claim', { username, password, passwordConfirm }),
  instantGames: () => call<InstantList>('GET', '/api/v1/instant/games'),
  play: (worldId?: number) => call<PlayResult>('POST', '/api/v1/instant/play', worldId === undefined ? undefined : { worldId }),
  leave: (worldId: number) => call<void>('POST', `/api/v1/worlds/${worldId}/leave`),
  publicStats: () => call<PublicStats>('GET', '/api/v1/public/stats'),
  publicLive: () => call<LiveStrip>('GET', '/api/v1/public/live'),
  report: (input: ReportInput) => call<{ id: number }>('POST', '/api/v1/reports', input),
  publicWorlds: () => call<PublicWorldList>('GET', '/api/v1/public/worlds'),
  publicWorld: (id: number) => call<PublicWorld>('GET', `/api/v1/public/worlds/${id}`),
  publicMap: (id: number) => call<PublicMap>('GET', `/api/v1/public/worlds/${id}/villages`),
  publicGame: (ref: string) => call<ArchivedGame>('GET', `/api/v1/public/games/${encodeURIComponent(ref)}`),
  publicProfile: (username: string) => call<PublicProfile>('GET', `/api/v1/public/profiles/${encodeURIComponent(username)}`),
  login: (username: string, password: string) =>
    call<{ account: Account }>('POST', '/api/v1/auth/login', { username, password }),
  register: (username: string, password: string, passwordConfirm: string) =>
    call<{ account: Account }>('POST', '/api/v1/auth/register', { username, password, passwordConfirm, locale: getLocale() }),
  logout: () => call<void>('POST', '/api/v1/auth/logout'),
  lobby: () => call<LobbyView>('GET', '/api/v1/worlds'),
  join: (worldId: number, quadrant: Quadrant) => call<{ playerId: string }>('POST', `/api/v1/worlds/${worldId}/join`, { quadrant }),
};
