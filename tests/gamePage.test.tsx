// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { setLocale } from '../src/i18n/i18n';

const NOW = 1_700_000_000_000;

beforeEach(() => {
  setLocale('en');
  vi.spyOn(Date, 'now').mockReturnValue(NOW);
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url === '/api/v1/public/games/12') {
      return new Response(JSON.stringify({
        worldId: 12, name: 'Game #12', gameNumber: 12, startedAt: NOW - 600_000, endedAt: 0, endsAt: NOW + 25 * 60_000,
        running: true, winnerName: '', winnerIsBot: false,
        summary: { endReason: '', winnerId: '', humans: 2, guests: 1, bots: 6, conquestsHumans: 1, conquestsBots: 0,
          attacksSent: 4, humanUnitsKilled: 30, humanUnitsLost: 10, largestBattleUnits: 40 },
        rows: [{ rank: 1, playerId: 'h1', name: 'Lis', isBot: false, points: 90, villages: 2, conquered: 1, killed: 30 }],
      }), { status: 200 });
    }
    return new Response(JSON.stringify({ error: { code: 'unauthenticated' } }), { status: 401 });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

describe('game page', () => {
  it('running game shows time left and no winner', async () => {
    history.replaceState(null, '', '/games/12');
    render(<App />);
    expect(await screen.findByText('Still running · 25 min left')).toBeTruthy();
    expect(screen.queryByText(/finished with the most points|won early/)).toBeNull();
    expect(screen.getByRole('table', { name: 'Leaderboard' })).toBeTruthy();
    await waitFor(() => expect(document.title).toBe('Game #12 — live now · 2 players, 6 bots'));
  });
});
