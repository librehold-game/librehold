import type { HostedCard } from '../account/api';
import type { Locale } from '../i18n/i18n';
import type {
  BuildTemplate, BuildingId, Command, FarmAssistantState, NewsItem, Player, PlayerId, PlayerStats, PlayerStatus, QuestState, Report,
  ScheduledSend, TraderState, UnitId, Village, VillageBonus, VillageGroup, VillageIntel, WorldConfig,
} from '../engine/types';

/** One player's knowledge of one village (the engine's Village.intel, farmTargetFlags and lastFarmedAt). */
export interface Intel {
  intel?: VillageIntel;
  farmTargetFlags?: { hasWall: boolean; scoutedAt?: number };
  lastFarmedAt?: number;
}

/** The parts of the engine's GameState that belong to one human. */
export interface HumanState {
  stats: PlayerStats;
  quests: QuestState;
  farmAssistant: FarmAssistantState;
  trader: TraderState;
  villageGroups: VillageGroup[];
  buildTemplates: BuildTemplate[];
  aggressorUntil: number;
}

/** What everyone may know of a player. */
export interface PublicPlayer {
  id: PlayerId;
  name: string;
  color: string;
  status: PlayerStatus;
  points: number;
  villageIds: string[];
  isBot: boolean;
}

/** A command as this player sees it: someone else's command coming at them has no `units`. */
export type CommandView = Omit<Command, 'units'> & { units?: Command['units'] };

/** The signed-in account, as GET /api/v1/me gives it. */
export interface MeView {
  id: number; username: string; isAdmin: boolean; isGuest: boolean; locale: Locale; tutorialDone: boolean;
}

export interface EndedView {
  reason: 'time' | 'domination' | 'host'; at: number; winnerId: string; winnerName: string; winnerIsBot: boolean;
  /** What the results page is reached by: the world id, or a player world's results code. */
  resultsRef?: string;
}

/** A village on the god view's map: stored or still generated. ownerId '' is a barbarian village. */
export interface WatchVillage { id: string; x: number; y: number; ownerId: PlayerId | ''; points: number }

/** One village in full for the host, settled to now. */
export interface WatchedVillage {
  id: string; name: string; x: number; y: number; ownerId: PlayerId | ''; ownerName?: string; ownerIsBot?: boolean; points: number;
  buildings: Partial<Record<BuildingId, number>>; unitsHome: Partial<Record<UnitId, number>>;
}

/** The whole player world as its host watches it: every village, every army on the road with its units. */
export interface SpectatorState {
  worldId: number;
  worldName: string;
  now: number;
  serverTime: number;
  paused: boolean;
  config: WorldConfig;
  hosted: HostedCard | null;
  instant?: InstantView;
  players: Record<PlayerId, PublicPlayer>;
  villages: WatchVillage[];
  commands: CommandView[];
  ranking: PlayerRow[];
}

/** A player world's host and waiting room; resumesAt is wall ms. */
export interface HostedView { hostName: string; waiting: boolean; humans: number; seats: number; pausedByHost: boolean; resumesAt?: number }

/** An instant world's end and join window in world time, and how it ended once it has. */
export interface InstantView { endsAt: number; joinUntil: number; ended?: EndedView }

export interface PlayerState {
  me: PlayerId;
  worldId: number;
  worldName: string;
  gameNumber?: number | null;
  now: number;
  serverTime: number;
  clockOriginLocalMs: number;
  paused: boolean;
  config: WorldConfig;
  villages: Record<string, Village>;
  /** The player's own full record. */
  player: Omit<Player, 'brain' | 'isHuman'>;
  players: Record<PlayerId, PublicPlayer>;
  commands: CommandView[];
  intel: Record<string, Intel>;
  scheduled: ScheduledSend[];
  human: HumanState;
  unreadReports: number;
  instant?: InstantView;
  hosted?: HostedView;
}

export interface MapTile {
  x: number;
  y: number;
  id: string;
  name: string;
  ownerId: PlayerId | null;
  ownerIsBot?: boolean;
  points: number;
  bonus?: VillageBonus;
}

export type NewsEntry = NewsItem & { for?: PlayerId };
export type { Report };

export interface PlayerRow {
  id: PlayerId; name: string; color: string; points: number; villages: number; att: number; def: number; sup: number; quit: boolean;
  isBot: boolean;
}
export interface DailyRow { id: PlayerId; name: string; att: number; def: number; loot: number; farm: number; isBot: boolean }
export interface ConquestRow {
  t: number; x: number; y: number; toId: PlayerId; to: string; fromId: PlayerId | null; from: string | null; toIsBot: boolean; fromIsBot: boolean;
}

export interface Profile {
  player: PublicPlayer;
  villages: { id: string; name: string; x: number; y: number; points: number }[];
  history: { hourly: number[]; daily: number[] };
}

export type Topic = 'state' | 'reports' | 'map' | 'ranking' | 'news';

export type LiveMessage =
  | { type: 'hello'; serverTime: number; worldNow: number }
  | { type: 'update'; topics: Topic[]; villageIds?: string[]; serverTime: number; worldNow: number }
  | { type: 'ping' };
