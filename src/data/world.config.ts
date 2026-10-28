import {
  COMMAND_BALANCE as commandBalance, DEFAULT_WORLD_CONFIG as defaultWorldConfig, FARM_BALANCE as farmBalance, MAP as map,
  WORLD_LIMITS as worldLimits,
} from './balance.json';
import type { BarbarianDensity, WorldConfig } from '../engine/types';

// World settings and constants from the server (src/data/balance.json). The defaults are in the server's shape:
// the bot stance is `allAgainstHumans` there, and pausing is a world's state, not a setting.

export const DEFAULT_WORLD_CONFIG = defaultWorldConfig as Omit<WorldConfig, 'paused' | 'botStance'> & { botStance: string };

export const WORLD_LIMITS = worldLimits;

export const MAP = map as Omit<typeof map, 'density'> & { density: Record<BarbarianDensity, number> };

export const COMMAND_BALANCE = commandBalance;
export const FARM_BALANCE = farmBalance;
