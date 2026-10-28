import { HUMAN_ID, type PlayerId } from '../engine/types';

// The UI and the engine helpers it uses call the player at the screen HUMAN_ID; the server calls them by
// their own id. These translate at the boundary so the call sites keep comparing with HUMAN_ID.

export function toClientId(id: PlayerId, me: PlayerId): PlayerId {
  return id === me ? HUMAN_ID : id;
}

export function toClientRef(id: PlayerId | null, me: PlayerId): PlayerId | null {
  return id === null ? null : toClientId(id, me);
}

export function toServerId(id: PlayerId, me: PlayerId): PlayerId {
  return id === HUMAN_ID ? me : id;
}
