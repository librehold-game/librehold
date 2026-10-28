import { useCallback, useEffect, useRef, useState } from 'react';
import { api, errorText } from './api';

/** Where Play sends the browser; a seam for tests. */
export const playNavigation = {
  assign(url: string) {
    window.location.assign(url);
  },
};

const RETRY_TICK_MS = 250;

/**
 * Play: a guest account first when nobody is signed in, then a seat in an instant game. With every game full it
 * counts down to the first free seat and asks again.
 */
export function usePlay(signedIn: boolean) {
  const signed = useRef(signedIn);
  signed.current ||= signedIn;
  const [busy, setBusy] = useState(false);
  const [retryAt, setRetryAt] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [error, setError] = useState('');

  const start = useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      if (!signed.current) {
        await api.guest();
        signed.current = true;
      }
      const r = await api.play();
      if (r.worldId) {
        playNavigation.assign(`/game/?world=${r.worldId}`);
        return;
      }
      setNow(Date.now());
      setRetryAt(Date.now() + (r.waitMs ?? 1000));
    } catch (err) {
      setError(errorText(err));
    }
    setBusy(false);
  }, []);

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

  return { start, busy: busy || retryAt !== null, waitMs: retryAt === null ? null : Math.max(0, retryAt - now), error };
}
