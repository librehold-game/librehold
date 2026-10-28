// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { legacyHashPath, rewriteLegacyHash } from '../src/account/legacyLinks';
import { setLocale } from '../src/i18n/i18n';

describe('old hash links', () => {
  it.each([
    ['#/login', 'en', '/login'],
    ['#/register', 'pl', '/pl/rejestracja'],
    ['#/lobby', 'en', '/lobby'],
    ['#/admin', 'pl', '/pl/admin'],
    ['#/privacy', 'pl', '/pl/prywatnosc'],
    ['#/player/kind%20otter', 'en', '/players/kind%20otter'],
    ['#/game/41', 'pl', '/pl/gry/41'],
  ] as const)('%s in %s → %s', (hash, locale, path) => {
    expect(legacyHashPath(hash, locale)).toBe(path);
  });

  it.each(['#/game/abc', '#/shop', '', '#screens'])('%s is not an old link', (hash) => {
    expect(legacyHashPath(hash, 'en')).toBeNull();
  });

  it('rewriteLegacyHash replaces the address', () => {
    setLocale('en');
    history.replaceState(null, '', '/?x=1#/game/7');
    const before = history.length;
    expect(rewriteLegacyHash()).toBe(true);
    expect(location.pathname).toBe('/games/7');
    expect(location.search).toBe('?x=1');
    expect(location.hash).toBe('');
    expect(history.length).toBe(before);
  });
});
