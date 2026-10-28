import { pathFor } from '../account/routes';
import { getLocale } from '../i18n/i18n';
import type { PlayerState } from './types';

export type Blocker = { kind: string } & Record<string, unknown>;

/** A refused request: a code with the values its message needs; the client words it (src/i18n/names). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly params?: Record<string, unknown>,
    readonly blockers?: Blocker[],
    readonly field?: string,
  ) {
    super(code);
  }
}

/** Where the client goes; a seam for tests. */
export const navigation = {
  assign(url: string) {
    window.location.href = url;
  },
};

/** The session is gone: back to the login page of the account site. */
export function onUnauthenticated() {
  navigation.assign(pathFor('login', getLocale()));
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
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
    if (res.status === 401) onUnauthenticated();
    throw new ApiError(res.status, e.code ?? 'internal', e.params, e.blockers, e.field);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
};

/** Sends an order to the world and returns the player's state after it. */
export function order(worldId: number, name: string, body: object): Promise<PlayerState> {
  return api.post<PlayerState>(`/api/v1/worlds/${worldId}/orders/${name}`, body);
}
