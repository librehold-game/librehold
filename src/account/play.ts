import { useCallback, useEffect, useRef, useState } from 'react';
import { useT } from '../i18n/i18n';
import { api, ApiError, errorText, type InstantList, type PlayResult } from './api';

/** Where Play sends the browser; a seam for tests. */
export const playNavigation = {
  assign(url: string) {
    window.location.assign(url);
  },
};

const RETRY_TICK_MS = 250;
const REFRESH_MS = 15_000;

/**
 * Play: a guest account first when nobody is signed in, then the running instant games to choose from; with none
 * running it starts a new one. With every game full it counts down to the first free seat and looks again.
 */
export function usePlay(signedIn: boolean) {
  const t = useT();
  const signed = useRef(signedIn);
  signed.current ||= signedIn;
  const [busy, setBusy] = useState(false);
  const [retryAt, setRetryAt] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [error, setError] = useState('');
  const [list, setList] = useState<InstantList | null>(null);

  const wait = useCallback((ms: number) => {
    setNow(Date.now());
    setRetryAt(Date.now() + ms);
  }, []);

  /** Opens the game, or starts the countdown when nothing has a seat; true when the browser is on its way. */
  const settle = useCallback((r: PlayResult): boolean => {
    if (r.worldId) {
      playNavigation.assign(`/game/?world=${r.worldId}`);
      return true;
    }
    wait(r.waitMs ?? 1000);
    return false;
  }, [wait]);

  const refresh = useCallback(async () => {
    try {
      const l = await api.instantGames();
      setList(l);
      if (!l.waitMs) setRetryAt(null);
    } catch {
      // the list stays as it was
    }
  }, []);

  const start = useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      if (!signed.current) {
        await api.guest();
        signed.current = true;
      }
      const l = await api.instantGames();
      if (l.games.length === 0 && l.canStart) {
        if (settle(await api.play())) return;
      } else {
        setList(l);
        if (l.waitMs) wait(l.waitMs);
      }
    } catch (err) {
      setError(errorText(err));
    }
    setBusy(false);
  }, [settle, wait]);

  const act = useCallback(async (what: () => Promise<PlayResult | void>) => {
    setBusy(true);
    setError('');
    try {
      const r = await what();
      if (r && settle(r)) return;
    } catch (err) {
      setError(err instanceof ApiError && err.code === 'worldClosed' ? t('play.gone') : errorText(err));
    }
    await refresh();
    setBusy(false);
  }, [refresh, settle, t]);

  const join = useCallback((id: number) => act(() => api.play(id)), [act]);
  const startNew = useCallback(() => act(() => api.play()), [act]);
  const leave = useCallback((id: number) => act(() => api.leave(id)), [act]);

  useEffect(() => {
    if (retryAt === null) return;
    const id = setInterval(() => {
      const n = Date.now();
      setNow(n);
      if (n >= retryAt) {
        setRetryAt(null);
        void start();
      }
    }, RETRY_TICK_MS);
    return () => clearInterval(id);
  }, [retryAt, start]);

  const showing = list !== null;
  useEffect(() => {
    if (!showing) return;
    const id = setInterval(() => void refresh(), REFRESH_MS);
    return () => clearInterval(id);
  }, [showing, refresh]);

  return {
    start, join, startNew, leave, list, error,
    busy, counting: retryAt !== null, waitMs: retryAt === null ? null : Math.max(0, retryAt - now),
  };
}
