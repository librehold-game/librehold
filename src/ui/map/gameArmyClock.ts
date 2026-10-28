import { useGameStore } from '../../state/gameStore';
import { armyClock } from './armyClock';

/** The player's army clock: the store's `game.now`, ticking with it. */
export const gameArmyClock = armyClock(
  () => useGameStore.getState().game?.now ?? null,
  (onTick) => useGameStore.subscribe((s, prev) => { if (s.game?.now !== prev.game?.now) onTick(); }),
);

export const armyNow = (): number => gameArmyClock.now();
