import { HUMAN_ID, type GameState, type Player, type PlayerId, type Village } from './types';

/** The only place that knows how village ownership is stored — the rest of the game asks here. */

export function isHumanVillage(v: Village): boolean {
  return v.ownerId === HUMAN_ID;
}

export function isBarbarianVillage(v: Village): boolean {
  return v.ownerId === null;
}

export function humanPlayer(state: GameState): Player {
  const h = state.players[HUMAN_ID];
  if (!h) throw new Error('the game state has no human player');
  return h;
}

export function ownerOf(state: GameState, v: Village): Player | null {
  return v.ownerId === null ? null : state.players[v.ownerId] ?? null;
}

export function playerVillages(state: GameState, id: PlayerId): Village[] {
  const out: Village[] = [];
  for (const vid of state.players[id]?.villageIds ?? []) {
    const v = state.villages[vid];
    if (v) out.push(v);
  }
  return out;
}

export function isProtected(state: GameState, playerId: PlayerId): boolean {
  return state.now < (state.players[playerId]?.protectedUntil ?? 0);
}

/** Build queue slots of the village owner (barbarians don't build). */
export function buildQueueSlotsFor(state: GameState, v: Village): number {
  return ownerOf(state, v)?.buildQueueSlots ?? 0;
}

export function humanVillageIds(state: GameState): readonly string[] {
  return humanPlayer(state).villageIds;
}

export function humanVillages(state: GameState): Village[] {
  return playerVillages(state, HUMAN_ID);
}

export function humanCoins(state: GameState): number {
  return humanPlayer(state).coins;
}

export function humanNoblesTrained(state: GameState): number {
  return humanPlayer(state).noblesTrained;
}

/** Player points = sum of their villages' points. */
export function playerPoints(state: GameState, id: PlayerId): number {
  let sum = 0;
  for (const v of playerVillages(state, id)) sum += v.points;
  return sum;
}

export function humanPoints(state: GameState): number {
  let sum = 0;
  for (const v of humanVillages(state)) sum += v.points;
  return sum;
}
