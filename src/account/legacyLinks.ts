import { getLocale, type Locale } from '../i18n/i18n';
import { pathFor, type RouteId } from './routes';

const PLAIN: Record<string, RouteId> = {
  '#/login': 'login',
  '#/register': 'register',
  '#/lobby': 'lobby',
  '#/admin': 'admin',
  '#/privacy': 'privacy',
};

/** Where an address from the old hash-routed site lives now. */
export function legacyHashPath(hash: string, locale: Locale): string | null {
  const plain = PLAIN[hash];
  if (plain) return pathFor(plain, locale);
  const game = /^#\/game\/(\d+)$/.exec(hash);
  if (game) return pathFor('game', locale, { id: game[1]! });
  const player = /^#\/player\/(.+)$/.exec(hash);
  if (player) {
    try {
      return pathFor('player', locale, { name: decodeURIComponent(player[1]!) });
    } catch {
      return null;
    }
  }
  return null;
}

export function rewriteLegacyHash(): boolean {
  const path = legacyHashPath(window.location.hash, getLocale());
  if (!path) return false;
  history.replaceState(null, '', path + window.location.search);
  return true;
}
