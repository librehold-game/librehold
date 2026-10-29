// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { GameState } from '../src/engine/types';
import { setLocale } from '../src/i18n/i18n';
import type { InstantView, MeView } from '../src/net/types';
import { useGameStore } from '../src/state/gameStore';
import { App } from '../src/ui/App';
import { stateFixture, newTestGame } from './helpers';
import { stubServer, type StubOptions, type StubRoute } from './serverStub';

const MIN = 60_000;
let g: GameState;
let calls: string[];
let bodies: Record<string, unknown[]>;

const guest: MeView = { id: 4, username: 'Player_cat', isAdmin: false, isGuest: true, locale: 'en', tutorialDone: true };

function running(left: number): InstantView {
  return { endsAt: g.now + left, joinUntil: g.now };
}

/** Records the bodies of the extra routes, keyed like the routes. */
function recorded(routes: Record<string, StubRoute>): Record<string, StubRoute> {
  const out: Record<string, StubRoute> = {};
  for (const [k, fn] of Object.entries(routes)) {
    out[k] = (body) => {
      (bodies[k] ??= []).push(body);
      return fn(body);
    };
  }
  return out;
}

async function boot(opts: StubOptions) {
  calls = stubServer(g, { ...opts, routes: recorded(opts.routes ?? {}) });
  render(<App />);
  await screen.findByRole('banner');
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  bodies = {};
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  setLocale('pl');
});

describe('instant game in the client', () => {
  it('shows remaining time and the create-account button for guests', async () => {
    await boot({ instant: running(30 * MIN), account: guest });
    const timer = await screen.findByRole('timer', { name: 'Time left' });
    expect(timer.textContent).toMatch(/^0:(30:00|29:5\d)$/);
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));
    const dialog = await screen.findByRole('dialog', { name: 'Keep this village' });
    expect((within(dialog).getByLabelText('Username') as HTMLInputElement).value).toBe('Player_cat');
    expect(screen.getByRole('button', { name: 'Leave world' })).toBeTruthy();
  });

  it('hides the create-account button for players and outside instant worlds', async () => {
    await boot({ instant: running(30 * MIN), account: { ...guest, isGuest: false } });
    await screen.findByRole('timer', { name: 'Time left' });
    expect(screen.queryByRole('button', { name: 'Create account' })).toBeNull();
    cleanup();
    useGameStore.getState().shutdown();
    await boot({ account: guest });
    await waitFor(() => expect(calls).toContain('GET /api/v1/me'));
    expect(screen.queryByRole('timer', { name: 'Time left' })).toBeNull();
  });

  it('claiming keeps the game and hides the button without a reload', async () => {
    await boot({ instant: running(30 * MIN), account: guest, routes: {
      'POST /api/v1/auth/claim': () => ({ status: 200, body: { ...guest, isGuest: false, username: 'Kot' } }),
    } });
    fireEvent.click(await screen.findByRole('button', { name: 'Create account' }));
    const dialog = await screen.findByRole('dialog', { name: 'Keep this village' });
    fireEvent.change(within(dialog).getByLabelText('Username'), { target: { value: 'Kot' } });
    fireEvent.change(within(dialog).getByLabelText('Password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.change(within(dialog).getByLabelText('Repeat the password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Create account' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Keep this village' })).toBeNull());
    expect(bodies['POST /api/v1/auth/claim']).toEqual([{ username: 'Kot', password: 'dlugie-haslo', passwordConfirm: 'dlugie-haslo' }]);
    expect(screen.queryByRole('button', { name: 'Create account' })).toBeNull();
  });

  it('a guest renames themselves from the bar', async () => {
    await boot({ instant: running(30 * MIN), account: guest, routes: {
      'PATCH /api/v1/me': (b) => ({ status: 200, body: { ...guest, ...(b as object) } }),
    } });
    fireEvent.click(await screen.findByRole('button', { name: 'Player_cat' }));
    const input = screen.getByLabelText('Nickname');
    fireEvent.change(input, { target: { value: 'Lis' } });
    fireEvent.submit(input);
    expect(await screen.findByRole('button', { name: 'Lis' })).toBeTruthy();
    expect(bodies['PATCH /api/v1/me']).toEqual([{ username: 'Lis' }]);
  });

  it('leaving asks first, then posts leave and goes home', async () => {
    const { navigation } = await import('../src/net/api');
    const assign = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    await boot({ instant: running(30 * MIN), account: guest, routes: {
      'POST /api/v1/worlds/1/leave': () => ({ status: 200 }),
    } });
    fireEvent.click(await screen.findByRole('button', { name: 'Leave world' }));
    expect(calls).not.toContain('POST /api/v1/worlds/1/leave');
    fireEvent.click(screen.getByRole('button', { name: 'Yes, leave' }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/'));
    expect(calls).toContain('POST /api/v1/worlds/1/leave');
  });

  it('tutorial can be skipped and is not shown again', async () => {
    await boot({ instant: running(50 * MIN), account: { ...guest, tutorialDone: false }, routes: {
      'PATCH /api/v1/me': (b) => ({ status: 200, body: { ...guest, ...(b as object) } }),
    } });
    const tutorial = await screen.findByRole('dialog', { name: 'Tutorial' });
    expect(within(tutorial).getByRole('heading').textContent).toBe('Your army');
    fireEvent.click(within(tutorial).getByRole('button', { name: 'Next' }));
    expect(within(tutorial).getByRole('heading').textContent).toBe('The barbarian next door');
    fireEvent.click(within(tutorial).getByRole('button', { name: 'Skip tutorial' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tutorial' })).toBeNull());
    expect(bodies['PATCH /api/v1/me']).toEqual([{ tutorialDone: true }]);
    cleanup();
    render(<App />);
    await screen.findByRole('banner');
    expect(screen.queryByRole('dialog', { name: 'Tutorial' })).toBeNull();
  });

  it('first conquest offers the claim dialog once', async () => {
    const v = Object.values(g.villages).find((x) => x.ownerId === 'human')!;
    await boot({ instant: running(50 * MIN), account: guest, routes: {
      'POST /api/v1/worlds/1/orders/build': () => ({ status: 200, body: { ...stateFixture(g), instant: running(50 * MIN) } }),
    } });
    await screen.findByRole('timer', { name: 'Time left' });
    expect(screen.queryByRole('dialog', { name: 'Keep this village' })).toBeNull();
    g.stats.villagesConquered += 1;
    await act(() => useGameStore.getState().build(v.id, 'main'));
    const dialog = await screen.findByRole('dialog', { name: 'Keep this village' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Not now' }));
    g.stats.villagesConquered += 1;
    await act(() => useGameStore.getState().build(v.id, 'main'));
    expect(screen.queryByRole('dialog', { name: 'Keep this village' })).toBeNull();
  });

  it('results overlay explains a game that ended because everyone left', async () => {
    const ended: InstantView = { ...running(0), ended: { reason: 'abandoned', at: g.now, winnerId: 'bot_1', winnerName: 'Borko', winnerIsBot: true } };
    await boot({ instant: ended, account: guest, routes: {} });
    const results = await screen.findByRole('dialog', { name: 'The game is over' });
    expect(within(results).getByText(/Everyone left, so the game ended early\. Borko finished with the most points\./)).toBeTruthy();
  });

  it('results overlay appears when the world ends', async () => {
    const ended: InstantView = { ...running(0), ended: { reason: 'time', at: g.now, winnerId: 'bot_1', winnerName: 'Borko', winnerIsBot: true } };
    await boot({ instant: ended, account: guest, routes: {
      'GET /api/v1/public/games/1': () => ({ status: 200, body: {
        worldId: 1, name: 'Instant #1', startedAt: 0, endedAt: 3_600_000, winnerName: 'Borko', winnerIsBot: true,
        summary: { endReason: 'time', winnerId: 'bot_1', humans: 1, bots: 1, conquestsHumans: 1, conquestsBots: 2, attacksSent: 9,
          humanUnitsKilled: 120, humanUnitsLost: 40, largestBattleUnits: 300 },
        rows: [
          { rank: 1, playerId: 'bot_1', name: 'Borko', isBot: true, points: 900, villages: 3, conquered: 2, killed: 300 },
          { rank: 2, playerId: 'h4', name: 'Player_cat', isBot: false, accountId: 4, points: 400, villages: 2, conquered: 1, killed: 120 },
        ] } }),
    } });
    const results = await screen.findByRole('dialog', { name: 'The game is over' });
    const board = await within(results).findByRole('table', { name: 'Leaderboard' });
    expect(within(board).getByText('Player_cat')).toBeTruthy();
    expect(board.querySelectorAll('.bot-tag')).toHaveLength(1);
    expect(within(results).getByText(/Borko/, { selector: 'p' }).querySelector('.bot-tag')).toBeTruthy();
    expect(within(results).getByRole('link', { name: 'Full results' }).getAttribute('href')).toBe('/games/1');
    expect(within(results).getByRole('link', { name: 'Play again' }).getAttribute('href')).toBe('/?play');
    expect(within(results).getByRole('button', { name: 'Create account' })).toBeTruthy();
    expect(screen.queryByRole('timer', { name: 'Time left' })?.textContent ?? '0:00:00').toBe('0:00:00');

    fireEvent.click(within(results).getByRole('button', { name: 'Back to the village' }));
    expect(screen.queryByRole('dialog', { name: 'The game is over' })).toBeNull();
    expect(screen.getAllByRole('navigation').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Results' }));
    expect(await screen.findByRole('dialog', { name: 'The game is over' })).toBeTruthy();
  });
});
