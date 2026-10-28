/**
 * Game data model. All times are milliseconds of WORLD TIME (not real time),
 * except `lastRealTime`, which is the real `Date.now()` at the last sync.
 */

export type Resource = 'wood' | 'clay' | 'iron';
export const RESOURCES: readonly Resource[] = ['wood', 'clay', 'iron'];
export type ResourceSet = Record<Resource, number>;

export type BuildingId =
  | 'main' | 'barracks' | 'stable' | 'workshop' | 'academy' | 'smithy'
  | 'statue' | 'market' | 'wood' | 'clay' | 'iron' | 'farm'
  | 'storage' | 'hide' | 'wall';

export const BUILDING_IDS: readonly BuildingId[] = [
  'main', 'barracks', 'stable', 'workshop', 'academy', 'smithy',
  'statue', 'market', 'wood', 'clay', 'iron', 'farm', 'storage', 'hide', 'wall',
];

export type UnitId =
  | 'spear' | 'sword' | 'axe' | 'archer'
  | 'scout' | 'lightCav' | 'mountedArcher' | 'heavyCav'
  | 'ram' | 'catapult' | 'paladin' | 'noble';

export const UNIT_IDS: readonly UnitId[] = [
  'spear', 'sword', 'axe', 'archer',
  'scout', 'lightCav', 'mountedArcher', 'heavyCav',
  'ram', 'catapult', 'paladin', 'noble',
];

/** Buildings with their own parallel recruitment queue (the Pedestal recruits the Knight). */
export type RecruitBuilding = 'barracks' | 'stable' | 'workshop' | 'academy' | 'statue';
export const RECRUIT_BUILDINGS: readonly RecruitBuilding[] = ['barracks', 'stable', 'workshop', 'academy', 'statue'];

export type VillageBonusKind =
  | 'wood' | 'clay' | 'iron' | 'allResources'
  | 'infantry' | 'cavalry' | 'farm' | 'storage';

export interface VillageBonus {
  kind: VillageBonusKind;
}

export interface BuildJob {
  id: string;
  building: BuildingId;
  /** Level after completion (for demolition: current − 1). */
  targetLevel: number;
  kind: 'build' | 'demolish';
  startedAt: number;
  completesAt: number;
}

export interface RecruitJob {
  id: string;
  unit: UnitId;
  total: number;
  done: number;
  perUnitMs: number;
  startedAt: number;
  completesAt: number;
}

export interface ResearchJob {
  id: string;
  unit: UnitId;
  targetLevel: number;
  startedAt: number;
  completesAt: number;
}

export type CommandType = 'attack' | 'support' | 'return' | 'trade' | 'intercept';

/**
 * Troop movement. For 'attack'/'support'/'trade': originId = home, targetId = target.
 * For 'return': originId = where they return from, targetId = home.
 */
export interface Command {
  id: string;
  type: CommandType;
  originId: string;
  targetId: string;
  units: Partial<Record<UnitId, number>>;
  haul?: ResourceSet;
  catapultTarget?: BuildingId;
  /** Trade: number of merchants on the way; `returning` = merchants heading home empty. */
  merchants?: number;
  returning?: boolean;
  departedAt: number;
  arrivesAt: number;
  /** Where an intercept marches to, or where a return with no origin village starts. */
  point?: FieldPoint;
  intercept?: InterceptOrder;
  /** Intercepts on their way to this army. */
  threats?: string[];
  /** The army survived a field battle; its owner may turn it back. */
  fieldHit?: boolean;
}

export interface FieldPoint { x: number; y: number }

/** A command's route as it was when an interception was sent at it. */
export interface Leg { originId: string; targetId: string; departedAt: number; arrivesAt: number }

export interface InterceptOrder { commandId: string; scout?: boolean; leg: Leg }

export interface FieldReport { x: number; y: number; targetType: string; missed?: boolean; targetCommandId: string }

export type ReportType = 'attack' | 'support' | 'scout' | 'conquest' | 'field' | 'fieldScout';

export interface ReportSide {
  units: Partial<Record<UnitId, number>>;
  losses: Partial<Record<UnitId, number>>;
}

export interface ScoutIntel {
  resources: ResourceSet;
  buildings: Record<BuildingId, number>;
  units: Partial<Record<UnitId, number>>;
  loyalty: number;
}

/** A village as a report remembers it; an empty name is the default one, worded by the client. */
export interface ReportVillage {
  id: string;
  name: string;
  x: number;
  y: number;
  ownerId?: string;
  ownerName?: string;
  ownerIsBot?: boolean;
}

export interface Report {
  id: string;
  time: number;
  type: ReportType;
  originId: string;
  targetId: string;
  /** Villages as they were at the time of the report. */
  origin: ReportVillage;
  target: ReportVillage;
  attacker: ReportSide;
  defender: ReportSide;
  /** Result of a successful scouting. */
  scout?: ScoutIntel;
  luck: number;
  morale: number;
  wallBefore: number;
  wallAfter: number;
  buildingDamage?: { building: BuildingId; from: number; to: number };
  haul?: ResourceSet;
  haulCapacity?: number;
  loyaltyBefore?: number;
  loyaltyAfter?: number;
  conquered?: boolean;
  outcome: 'attackerWon' | 'defenderWon';
  read: boolean;
  field?: FieldReport;
}

export const HUMAN_ID = 'human';
export type PlayerId = string;
export type PlayerStatus = 'pending' | 'active' | 'quit' | 'eliminated';

export type Difficulty = 'easy' | 'normal' | 'hard' | 'brutal';
export type BotStyle = 'farmer' | 'turtle' | 'aggressor' | 'pointBuilder' | 'opportunist';

/** A bot's daily rhythm — all times in minutes of the local day; `sleepMin` may exceed 1440 (past midnight). */
export interface Routine {
  wakeMin: number;
  sleepMin: number;
  checkIns: { atMin: number; lenMin: number }[];
  eveningStartMin: number;
  eveningLenMin: number;
  weekendWakeShiftMin: number;
}

/**
 * What a bot knows about a village — only from observation (its own attacks and scouting) and from the map (owner, points). Compact,
 * without full reports, so the save doesn't bloat. The bot doesn't copy generator barbarians — it reads them from the map.
 */
export interface KnownVillage {
  id: string;
  x: number;
  y: number;
  ownerId: PlayerId | null;
  points: number;
  seenAt: number;
  units?: Partial<Record<UnitId, number>>;
  wall?: number;
  /** Hideout and Storehouse capacity (from scouting) — for judging a "dried-up" farm. */
  hide?: number;
  storage?: number;
  /** Building levels from scouting (catapult targets). */
  buildings?: Partial<Record<BuildingId, number>>;
  /** Last 3 loot-to-capacity ratios. */
  hauls: number[];
  lastAttackAt?: number;
  losses?: boolean;
  deadSince?: number;
}

/** State of a bot's "head". Later stages add fields (memory, mood, grudges…). */
export interface BotBrain {
  skill: number;
  style: BotStyle;
  /** Resilience to being overwhelmed (strong bots rarely quit). */
  resilience: number;
  /** Caution: how often it checks the game outside a session while something is incoming. */
  caution: number;
  routine: Routine;
  online: boolean;
  sessionEndsAt: number;
  /** Times of the last actions (max 50) — for rhythm tests and inspection. */
  actionLog: number[];
  /** Village role (after taking a second one): `mixed` / `off` / `def`. */
  roles: Record<string, 'mixed' | 'off' | 'def'>;
  memory: { known: Record<string, KnownVillage>; discoveredAt: number };
  /** Overwhelm (grows with losses and attacks, decays with every session) — above the threshold the bot quits. */
  overwhelm: number;
  /** Counters since the previous session (reset when it starts). */
  lastSessionStats: { villagesLost: number; incomingAttacks: number };
  /** Barbarian village the bot is currently taking over. */
  conquestTarget?: string;
  /** Grudges against players (last attack and how many there were) — fade after `BOT.grudgeFadeDays`. */
  grudges: Record<PlayerId, { lastAt: number; level: number }>;
  /** Phase for the next few days and the campaign target (with progress — dropped when it stalls). */
  /** Opponents' habits: dodge minutes and intervals between their attacks on the bot (`lastHitAt` — last attack). */
  habits: Record<PlayerId, { dodgeReturnMins: number[]; farmIntervalsMin: number[]; lastHitAt?: number }>;
  mood: { phase: 'build' | 'campaign' | 'consolidate'; until: number; targetPlayer?: PlayerId; targetVillage?: string; failures: number; progressAt?: number; opAt?: number };
}

/** A world player: human or bot — the same entity. Villages without an owner are barbarians. */
export interface Player {
  id: PlayerId;
  name: string;
  isHuman: boolean;
  color: string;
  villageIds: string[];
  /** Hetman coins (global per player). */
  coins: number;
  /** Nobles trained — they raise the cost of further coins. */
  noblesTrained: number;
  buildQueueSlots: number;
  /** End of beginner protection (world time). */
  protectedUntil: number;
  joinAt: number;
  status: PlayerStatus;
  /** Bots only. */
  brain?: BotBrain;
  /** Points from the last hourly ranking recount (public). */
  points: number;
  /** When the player quit (for rumours). */
  quitAt?: number;
  /** Public points history: hourly (last 168) and at local midnight (last 120). */
  history: { hourly: number[]; daily: number[] };
}

export interface Village {
  id: string;
  x: number;
  y: number;
  name: string;
  /** Owner; `null` = barbarian village. Changed only through `setOwner`. */
  ownerId: PlayerId | null;
  points: number;

  resources: ResourceSet;
  resourcesUpdatedAt: number;

  buildings: Record<BuildingId, number>;
  units: Record<UnitId, number>;
  supportFrom: Record<string, Partial<Record<UnitId, number>>>;

  loyalty: number;
  loyaltyUpdatedAt: number;

  buildQueue: BuildJob[];
  recruitQueues: Record<RecruitBuilding, RecruitJob[]>;
  researchQueue: ResearchJob[];
  /** Research level per unit (0 = locked, 1..3). Research is per village. */
  research: Record<UnitId, number>;

  bonus?: VillageBonus;
  lastFarmedAt?: number;
  farmTargetFlags?: { hasWall: boolean; scoutedAt?: number };
  /** Build plan (template): next buildings are pushed to the queue when a slot frees up and resources suffice. */
  buildPlan?: BuildingId[];
  /** Time the player conquered it. */
  conqueredAt?: number;
  /** The player's latest knowledge of the village (after an attack/scouting). */
  intel?: VillageIntel;
}

/** Class of the slowest unit in an incoming wave — inferred from the march time, the way players do it. */
export type ThreatClass = 'scout' | 'cavalry' | 'infantry' | 'siege' | 'noble';

export interface NewsItem {
  id: string;
  time: number;
  kind: 'world';
  /** Catalogue key and the values its text needs. */
  key: string;
  params?: Record<string, unknown>;
  /** Message worth an in-game notification (toast + sound). */
  alert?: boolean;
  villageId?: string;
}

export interface VillageIntel {
  at: number;
  /** Defender units seen before the battle / by scouting. */
  units: Partial<Record<UnitId, number>>;
  wall: number;
  loyalty: number;
  lastHaul?: ResourceSet;
  lastHaulCapacity?: number;
  /** How many units we lost in the last attack. */
  lastLosses?: number;
  lastOutcome?: 'attackerWon' | 'defenderWon';
}

/** Multipliers for one side of the world (human or bots) — 1 = unchanged. */
export interface SideModifiers {
  resources: number;
  recruitment: number;
  building: number;
}

export type BarbarianDensity = 'sparse' | 'normal' | 'dense';
export type BattleLossMode = 'normal' | 'reduced';

export interface WorldConfig {
  worldSpeed: number;
  unitSpeed: number;
  mapSize: number;
  barbarianDensity: BarbarianDensity;
  battleLosses: BattleLossMode;
  humanModifiers: SideModifiers;
  botModifiers: SideModifiers;
  /** Number of bot players in the world (fixed at creation). */
  botCount: number;
  /** Bots on the map from the start, or joining gradually over `rampUpDays`. */
  botArrival: 'allAtOnce' | 'rampUp';
  rampUpDays: number;
  /** Beginner protection in days since joining the world; 0 = off. */
  beginnerProtectionDays: number;
  /** Whether a player's last village can be conquered (then the human starts over, the bot drops out). */
  lastVillage: 'protected' | 'canBeLost';
  /** Minimum share of strong bots (`DIFFICULTY_FLOOR`). */
  difficulty: Difficulty;
  /** How bots treat the human: peaceful (retaliation / reputation), normal, or everyone against them. */
  botStance: 'peacefulRetaliate' | 'peacefulReputation' | 'normal' | 'allAgainstMe';
  paused: boolean;
  /** Field battles are on unless switched off. */
  noFieldBattles?: boolean;
  /** Tiles from a village within which a passing army is seen; missing means 1. */
  sightRadius?: number;
  /** Nobody sees other players' armies pass. */
  noSight?: boolean;
  /** Instant worlds: a nobleman needs no coins and coins cannot be minted. */
  noCoins?: boolean;
  /** Instant worlds: the nobleman's price, in place of the unit table's. */
  nobleCost?: ResourceSet;
  /** Instant worlds: the loyalty a nobleman takes from a village with an owner. */
  ownedNobleDropMin?: number;
  ownedNobleDropMax?: number;
  /** Instant worlds: an academy holds this many nobles more than its level. */
  academyExtraNobles?: number;
}

export interface QuestState {
  /** Quests whose condition has been met (detected by the server). */
  completed: string[];
  /** ids of currently active quests. */
  active: string[];
  /** Quests whose reward the player has claimed. */
  claimed: string[];
  /** Each active quest's bar value (0..1), sent by the server; quests without a bar are absent. */
  progress?: Record<string, number>;
}

export interface FarmTemplate {
  units: Partial<Record<UnitId, number>>;
}

export interface FarmFilters {
  /** Target search radius (fields). */
  radius: number;
  /** Hide targets attacked within the last N world minutes. */
  minMinutesSinceAttack: number;
  /** Hide targets with a wall > X. */
  maxWall: number;
  /** Hide targets where there were recent losses. */
  hideLosses: boolean;
  /** Hide targets that already have an attack on the way. */
  hideInbound: boolean;
}

export interface FarmAssistantState {
  templates: Record<'A' | 'B' | 'C' | 'D', FarmTemplate>;
  autoFarmUnlocked: boolean;
  autoFarmEnabled: boolean;
  /** Villages with auto-farm enabled. */
  autoFarmVillages: string[];
  filters: FarmFilters;
}

export interface TraderOffer {
  give: Resource;
  giveAmount: number;
  get: Resource;
  getAmount: number;
}

export interface TraderState {
  offerIndex: number;
  offer: TraderOffer;
  expiresAt: number;
  accepted: boolean;
}

export interface VillageGroup {
  id: string;
  name: string;
  villageIds: string[];
}

export interface BuildTemplate {
  id: string;
  name: string;
  steps: BuildingId[];
}

export interface PlayerStats {
  attacksSent: number;
  /** Daily counters of the farm assistant. */
  attacksToday: { day: number; count: number };
  lootedToday: { day: number; total: number };
  resourcesLooted: ResourceSet;
  unitsKilled: number;
  unitsLost: number;
  villagesConquered: number;
  /** Points samples over time [world_time, points]. */
  pointsHistory: [number, number][];
  battlesWon: number;
  scoutsSent: number;
  supportsSent: number;
  tradesSent: number;
  coinsMinted: number;
  researchCompleted: number;
  /** Repelled attacks on the player's villages. */
  attacksRepelled: number;
}

/** Send scheduled for an exact millisecond: "send at" or "arrive at" (then `sendAt` is derived from the march). */
export interface ScheduledSend {
  id: string;
  ownerId: PlayerId;
  originId: string;
  x: number;
  y: number;
  units: Partial<Record<UnitId, number>>;
  type: 'attack' | 'support';
  catapultTarget?: BuildingId;
  mode: 'sendAt' | 'arriveAt';
  at: number;
  sendAt: number;
  /** Scheduled cancel (dodge): the command turns back at this moment. */
  cancelAt?: number;
}

export interface GameState {
  worldSeed: number;
  /** Current world time in ms. */
  now: number;
  /** Real `Date.now()` at the moment `now` was current. */
  lastRealTime: number;
  config: WorldConfig;
  villages: Record<string, Village>;
  players: Record<PlayerId, Player>;
  activeVillageId: string;
  commands: Command[];
  /** Hash of the `commands` change history (additions and removals) — part of the command index stamp in `commands.ts`. */
  commandsRev: number;
  reports: Report[];
  quests: QuestState;
  farmAssistant: FarmAssistantState;
  trader: TraderState;
  news: NewsItem[];
  villageGroups: VillageGroup[];
  buildTemplates: BuildTemplate[];
  stats: PlayerStats;
  /** Local wall-clock time (ms since epoch, with the zone offset) corresponding to world time 0. */
  clock: { originLocalMs: number };
  scheduled: ScheduledSend[];
  /** Until when the human is seen as an aggressor (attacked active bots) — for "peaceful — reputation". */
  humanAggressorUntil: number;
}

export const MS_PER_SECOND = 1000;
export const MS_PER_MINUTE = 60_000;
export const MS_PER_HOUR = 3_600_000;
export const MS_PER_DAY = 86_400_000;

export function emptyResources(): ResourceSet {
  return { wood: 0, clay: 0, iron: 0 };
}

export function emptyBuildings(): Record<BuildingId, number> {
  const out = {} as Record<BuildingId, number>;
  for (const id of BUILDING_IDS) out[id] = 0;
  return out;
}

export function emptyUnits(): Record<UnitId, number> {
  const out = {} as Record<UnitId, number>;
  for (const id of UNIT_IDS) out[id] = 0;
  return out;
}

export function emptyRecruitQueues(): Record<RecruitBuilding, RecruitJob[]> {
  return { barracks: [], stable: [], workshop: [], academy: [], statue: [] };
}

export function villageIdFor(x: number, y: number): string {
  return `v:${x}:${y}`;
}

/** Coordinates from a village id (`v:x:y`) — when the village itself isn't in the state (the client only knows its own). */
export function coordsOfVillageId(id: string): { x: number; y: number } | null {
  const m = /^v:(\d+):(\d+)$/.exec(id);
  return m ? { x: Number(m[1]), y: Number(m[2]) } : null;
}

