import { QUESTS as quests } from './balance.json';
import type { ResourceSet } from '../engine/types';

// The quests' conditions and progress bars are the server's; the browser lists them with their rewards.

export interface QuestReward {
  resources?: ResourceSet;
  /** +1 build queue slot (up to the WORLD_LIMITS cap). */
  buildQueueSlot?: boolean;
  /** Unlocks auto-farm. */
  autoFarm?: boolean;
}

export interface QuestDef {
  id: string;
  /** Quests that must be completed before this one becomes active. */
  requires: string[];
  reward: QuestReward;
}

export const QUESTS: readonly QuestDef[] = quests;

export const QUEST_BY_ID: Record<string, QuestDef> = Object.fromEntries(QUESTS.map((q) => [q.id, q]));
