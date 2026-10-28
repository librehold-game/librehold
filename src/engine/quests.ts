import { QUESTS, QUEST_BY_ID, type QuestDef } from '../data/quests.data';
import type { GameState } from './types';

// Which quests are active and claimable, as list filters over the server's quest state; the conditions and
// the bars' progress are the server's.

/** Active quests: all requirements completed, the quest itself not yet. */
export function activeQuests(state: GameState): QuestDef[] {
  const done = new Set(state.quests.completed);
  return QUESTS.filter((q) => !done.has(q.id) && q.requires.every((r) => done.has(r)));
}

export function claimableQuests(state: GameState): QuestDef[] {
  const claimed = new Set(state.quests.claimed);
  return state.quests.completed.filter((id) => !claimed.has(id)).map((id) => QUEST_BY_ID[id]).filter((q): q is QuestDef => !!q);
}
