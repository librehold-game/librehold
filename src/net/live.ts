import { onUnauthenticated } from './api';
import type { LiveMessage } from './types';

const BACKOFF_MS = [1000, 2000, 4000, 8000, 15000];
const CLOSE_UNAUTHENTICATED = 4401;

/**
 * Keeps a live connection to one world: every message goes to onMessage, a lost connection is retried after
 * 1, 2, 4, 8, then every 15 s, and a `hello` (sent on every connect) resets the delay. Returns the disconnect.
 */
export function connectLive(worldId: number, onMessage: (m: LiveMessage) => void): () => void {
  let attempt = 0;
  let stopped = false;
  let socket: WebSocket | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const open = () => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    socket = new WebSocket(`${proto}//${window.location.host}/api/v1/live?world=${worldId}`);
    socket.onmessage = (e) => {
      const m = JSON.parse(String(e.data)) as LiveMessage;
      if (m.type === 'hello') attempt = 0;
      onMessage(m);
    };
    socket.onclose = (e) => {
      if (stopped) return;
      if (e.code === CLOSE_UNAUTHENTICATED) {
        onUnauthenticated();
        return;
      }
      const delay = BACKOFF_MS[Math.min(attempt, BACKOFF_MS.length - 1)]!;
      attempt++;
      timer = setTimeout(open, delay);
    };
  };

  open();
  return () => {
    stopped = true;
    clearTimeout(timer);
    socket?.close();
  };
}
