import { DEFAULT_WORLD_CONFIG, WORLD_LIMITS } from '../data/world.config';

/** World settings as the server stores them; a copy of the game's type so this site needs no game code. */
export interface SideModifiers { resources: number; recruitment: number; building: number }

export interface WorldConfig {
  worldSpeed: number;
  unitSpeed: number;
  mapSize: number;
  barbarianDensity: 'sparse' | 'normal' | 'dense';
  battleLosses: 'normal' | 'reduced';
  humanModifiers: SideModifiers;
  botModifiers: SideModifiers;
  botCount: number;
  botArrival: 'allAtOnce' | 'rampUp';
  rampUpDays: number;
  beginnerProtectionDays: number;
  lastVillage: 'protected' | 'canBeLost';
  difficulty: 'easy' | 'normal' | 'hard' | 'brutal';
  botStance: 'peacefulRetaliate' | 'peacefulReputation' | 'normal' | 'allAgainstHumans';
  noFieldBattles?: boolean;
  sightRadius?: number;
}

/** The server's defaults; it leaves field battles on and the sight radius at 0, which means 1. */
export const DEFAULT_CONFIG: WorldConfig = { ...(DEFAULT_WORLD_CONFIG as WorldConfig), noFieldBattles: false, sightRadius: 1 };

const L = WORLD_LIMITS;

export interface FieldSpec {
  key: keyof WorldConfig;
  creationOnly: boolean;
  /** Choice values; their words are admin.choice.<key>.<value> (map sizes are shown as numbers). */
  choices?: readonly string[];
  min?: number;
  max?: number;
  step?: number;
  /** An on/off setting, shown as a checkbox. */
  flag?: boolean;
}

/** Every setting in the order the forms show them; creation-only ones are fixed once the world exists. */
export const FIELDS: FieldSpec[] = [
  { key: 'worldSpeed', creationOnly: false, min: L.worldSpeed.min, max: L.worldSpeed.max, step: 0.5 },
  { key: 'unitSpeed', creationOnly: false, min: L.unitSpeed.min, max: L.unitSpeed.max, step: 0.1 },
  { key: 'mapSize', creationOnly: true, choices: L.mapSizes.map(String) },
  { key: 'barbarianDensity', creationOnly: true, choices: ['sparse', 'normal', 'dense'] },
  { key: 'botCount', creationOnly: true, min: L.botCount.min, max: L.botCount.max, step: 1 },
  { key: 'botArrival', creationOnly: true, choices: ['allAtOnce', 'rampUp'] },
  { key: 'rampUpDays', creationOnly: true, min: L.rampUpDays.min, max: L.rampUpDays.max, step: 1 },
  { key: 'beginnerProtectionDays', creationOnly: false, min: L.beginnerProtectionDays.min, max: L.beginnerProtectionDays.max, step: 1 },
  { key: 'lastVillage', creationOnly: false, choices: ['protected', 'canBeLost'] },
  { key: 'difficulty', creationOnly: false, choices: ['easy', 'normal', 'hard', 'brutal'] },
  { key: 'botStance', creationOnly: false, choices: ['peacefulRetaliate', 'peacefulReputation', 'normal', 'allAgainstHumans'] },
  { key: 'battleLosses', creationOnly: false, choices: ['normal', 'reduced'] },
  { key: 'noFieldBattles', creationOnly: false, flag: true },
  // 0 is "no sight", a separate switch in the hosted form.
  { key: 'sightRadius', creationOnly: true, min: 1, max: L.sightRadius.max, step: 1 },
  { key: 'humanModifiers', creationOnly: false, min: L.modifier.min, max: L.modifier.max, step: 0.05 },
  { key: 'botModifiers', creationOnly: false, min: L.modifier.min, max: L.modifier.max, step: 0.05 },
];

export const MODIFIERS: (keyof SideModifiers)[] = ['resources', 'recruitment', 'building'];
