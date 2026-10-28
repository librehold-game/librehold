// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import type { GameState } from '../src/engine/types';
import { setLocale } from '../src/i18n/i18n';
import type { HostedView, InstantView } from '../src/net/types';
import { useGameStore } from '../src/state/gameStore';
import { App } from '../src/ui/App';
import { newTestGame } from './helpers';
import { stubServer, type StubOptions } from './serverStub';

const MIN = 60_000;
let g: GameState;

const hosted = (over: Partial<HostedView> = {}): HostedView =>
  ({ hostName: 'ala', waiting: false, humans: 3, seats: 10, pausedByHost: false, ...over });

async function boot(opts: StubOptions) {
  stubServer(g, opts);
  render(<App />);
  await screen.findByRole('banner');
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  setLocale('pl');
});

describe('player worlds in the game', () => {
  it('players wait for the host with the count', async () => {
    const instant: InstantView = { endsAt: g.now + 60 * MIN, joinUntil: g.now + 10 * MIN };
    await boot({ instant, paused: true, hosted: hosted({ waiting: true }) });
    const waiting = await screen.findByRole('dialog', { name: 'Waiting for the start' });
    expect(within(waiting).getByText('Waiting for ala to start · 3 players here')).toBeTruthy();
    expect(within(waiting).getByRole('timer').textContent).toBe('1:00:00');
    expect(screen.queryByText('The world is paused by the administrator')).toBeNull();
  });

  it('a host pause shows when the game resumes', async () => {
    const instant: InstantView = { endsAt: g.now + 30 * MIN, joinUntil: g.now };
    await boot({ instant, paused: true, hosted: hosted({ pausedByHost: true, resumesAt: Date.now() + 5 * MIN }) });
    const line = await screen.findByText(/^Paused by ala · resumes in /);
    expect(line.textContent).toMatch(/resumes in 0:0(5:00|4:5\d)$/);
    expect(screen.queryByRole('dialog', { name: 'Waiting for the start' })).toBeNull();
  });

  it('results link to the results code of a player world', async () => {
    const code = 'resultscoderesultsco';
    const instant: InstantView = { endsAt: g.now, joinUntil: g.now,
      ended: { reason: 'host', resultsRef: code, at: g.now, winnerId: 'h0', winnerName: 'Ala', winnerIsBot: false } };
    await boot({ instant, hosted: hosted(), routes: {
      [`GET /api/v1/public/games/${code}`]: () => ({ status: 200, body: {
        worldId: 1, name: 'Friday night', startedAt: 0, endedAt: 1, winnerName: 'Ala', winnerIsBot: false,
        summary: { endReason: 'host', winnerId: 'h0', humans: 1, bots: 0, conquestsHumans: 0, conquestsBots: 0, attacksSent: 0,
          humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
        rows: [{ rank: 1, playerId: 'h0', name: 'Ala', isBot: false, points: 10, villages: 1, conquered: 0, killed: 0 }] } }),
    } });
    const results = await screen.findByRole('dialog', { name: 'The game is over' });
    expect(within(results).getByText('The host ended the game. Ala finished with the most points.')).toBeTruthy();
    await within(results).findByRole('table', { name: 'Leaderboard' });
    expect(within(results).getByRole('link', { name: 'Full results' }).getAttribute('href')).toBe(`/games/${code}`);
    expect(within(results).getByRole('link', { name: 'Watch the map' }).getAttribute('href')).toBe(`/games/${code}#watch`);
  });
});
