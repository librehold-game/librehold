// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { act } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { playNavigation } from '../src/account/play';
import { navigate } from '../src/account/router';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };
type Handler = (body: unknown) => Route;

let routes: Record<string, Handler>;
let calls: { key: string; body: unknown }[];

const hiddenStats = { visible: false, allTime: { games: 3, players: 2, conquests: 4, unitsKilled: 50 },
  last24h: { games: 3, players: 2, conquests: 4, unitsKilled: 50 } };
const guestAccount = { id: 9, username: 'Player_fox', isAdmin: false, isGuest: true, locale: 'en', tutorialDone: false };

beforeEach(() => {
  setLocale('en');
  calls = [];
  routes = {
    'GET /api/v1/me': () => ({ status: 401, body: { error: { code: 'unauthenticated' } } }),
    'GET /api/v1/public/stats': () => ({ status: 200, body: hiddenStats }),
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ key, body });
    const r = routes[key]?.(body) ?? { status: 404, body: { error: { code: 'notFound' } } };
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const at = (path: string) => history.replaceState(null, '', path);

describe('front door', () => {
  it('Play creates a guest and opens the game', async () => {
    const assign = vi.spyOn(playNavigation, 'assign').mockImplementation(() => {});
    routes['POST /api/v1/auth/guest'] = () => ({ status: 201, body: { account: guestAccount, token: 'x' } });
    routes['POST /api/v1/instant/play'] = () => ({ status: 200, body: { worldId: 5 } });
    render(<App />);
    fireEvent.click(await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=5'));
    const keys = calls.map((c) => c.key);
    expect(keys.indexOf('POST /api/v1/auth/guest')).toBeGreaterThan(-1);
    expect(keys.indexOf('POST /api/v1/auth/guest')).toBeLessThan(keys.indexOf('POST /api/v1/instant/play'));
    expect(calls.find((c) => c.key === 'POST /api/v1/auth/guest')!.body).toEqual({ locale: 'en' });
    expect(screen.getAllByRole('link', { name: 'Create an account' }).length).toBeGreaterThanOrEqual(1);
  });

  it('Play shows a countdown when full', async () => {
    const assign = vi.spyOn(playNavigation, 'assign').mockImplementation(() => {});
    routes['GET /api/v1/me'] = () => ({ status: 200, body: { ...guestAccount, isGuest: false } });
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: { mine: [], open: [] } });
    let plays = 0;
    routes['POST /api/v1/instant/play'] = () => ({ status: 200, body: ++plays === 1 ? { waitMs: 1000 } : { worldId: 6 } });
    at('/lobby');
    render(<App />);
    fireEvent.click(await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!));
    expect(await screen.findByText(/All games are full/)).toBeTruthy();
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=6'), { timeout: 4000 });
    expect(calls.filter((c) => c.key === 'POST /api/v1/auth/guest')).toHaveLength(0);
    expect(plays).toBe(2);
  });

  it('Play waits until the account is known', async () => {
    let answer: (r: Response) => void = () => {};
    vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/v1/me'
      ? new Promise<Response>((r) => { answer = r; })
      : Promise.resolve(new Response(JSON.stringify(hiddenStats), { status: 200 })))));
    render(<App />);
    const play = await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!);
    expect(play.hasAttribute('disabled')).toBe(true);
    answer(new Response(JSON.stringify({ error: { code: 'unauthenticated' } }), { status: 401 }));
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Play now' })[0]!.hasAttribute('disabled')).toBe(false));
  });

  it('stats stay hidden until visible', async () => {
    render(<App />);
    await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!);
    await waitFor(() => expect(calls.map((c) => c.key)).toContain('GET /api/v1/public/stats'));
    expect(screen.queryByRole('region', { name: 'Instant games so far' })).toBeNull();
    cleanup();
    routes['GET /api/v1/public/stats'] = () => ({ status: 200, body: { ...hiddenStats, visible: true,
      allTime: { games: 25, players: 31, conquests: 1234, unitsKilled: 56789 } } });
    render(<App />);
    const stats = await screen.findByRole('region', { name: 'Instant games so far' });
    expect(stats.textContent).toContain('1,234');
    expect(stats.textContent).toContain('56,789');
  });

  it('a guest sees only the instant card and a claim form in the lobby', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: guestAccount });
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: { mine: [], open: [{ id: 1, name: 'Long', humans: 1, bots: 3, paused: false, settings: {} }] } });
    routes['POST /api/v1/auth/claim'] = () => ({ status: 200, body: { ...guestAccount, isGuest: false, username: 'Lis' } });
    at('/lobby');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Instant game' })).toBeTruthy();
    expect(screen.queryByText('Long')).toBeNull();
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'Lis' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.change(screen.getByLabelText('Repeat the password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('Long')).toBeTruthy();
    expect(calls.find((c) => c.key === 'POST /api/v1/auth/claim')!.body).toEqual({ username: 'Lis', password: 'dlugie-haslo', passwordConfirm: 'dlugie-haslo' });
  });

  it('profile and game pages render archive data', async () => {
    routes['GET /api/v1/public/profiles/Lis'] = () => ({ status: 200, body: {
      username: 'Lis', isGuest: false, tally: { games: 3, wins: 1, conquered: 7, killed: 4200 },
      games: [{ worldId: 7, name: 'Instant #7', endedAt: 1_700_000_000_000, rank: 1, players: 20, points: 900, conquered: 4, killed: 2000 }] } });
    routes['GET /api/v1/public/games/7'] = () => ({ status: 200, body: {
      worldId: 7, name: 'Instant #7', startedAt: 1_699_996_400_000, endedAt: 1_700_000_000_000, winnerName: 'Lis', winnerIsBot: false,
      summary: { endReason: 'domination', winnerId: 'h9', humans: 1, bots: 1, conquestsHumans: 4, conquestsBots: 1, attacksSent: 30,
        humanUnitsKilled: 2000, humanUnitsLost: 500, largestBattleUnits: 800 },
      rows: [
        { rank: 1, playerId: 'h9', name: 'Lis', isBot: false, accountId: 9, points: 900, villages: 5, conquered: 4, killed: 2000 },
        { rank: 2, playerId: 'bot_3', name: 'Borko', isBot: true, points: 300, villages: 1, conquered: 1, killed: 100 },
        { rank: 3, playerId: 'h12', name: '', isBot: false, points: 100, villages: 1, conquered: 0, killed: 5 },
      ] } });
    at('/players/Lis');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Lis' })).toBeTruthy();
    const tally = screen.getByRole('region', { name: 'Instant games' });
    expect(tally.textContent).toContain('4,200');
    const link = screen.getByRole('link', { name: 'Instant #7' });
    expect(link.getAttribute('href')).toBe('/games/7');

    act(() => navigate('/games/7'));
    const board = await screen.findByRole('table', { name: 'Leaderboard' });
    const rows = within(board).getAllByRole('row');
    expect(rows).toHaveLength(4);
    expect(within(rows[2]!).getByText('bot')).toBeTruthy();
    expect(within(rows[3]!).getByText('Former player')).toBeTruthy();
    expect(within(rows[1]!).getByRole('link', { name: 'Lis' }).getAttribute('href')).toBe('/players/Lis');
    expect(screen.getByText(/won early by holding most of the map/)).toBeTruthy();
  });

  it('instant games are named by their number in the page language', async () => {
    routes['GET /api/v1/public/profiles/Lis'] = () => ({ status: 200, body: {
      username: 'Lis', isGuest: false, tally: { games: 1, wins: 1, conquered: 0, killed: 0 },
      games: [{ worldId: 7, name: 'Game #7', gameNumber: 7, endedAt: 1_700_000_000_000, rank: 1, players: 2, points: 9, conquered: 0, killed: 0 }] } });
    routes['GET /api/v1/public/games/7'] = () => ({ status: 200, body: {
      worldId: 7, name: 'Game #7', gameNumber: 7, startedAt: 0, endedAt: 3_600_000, winnerName: 'Lis', winnerIsBot: false,
      summary: { endReason: 'time', winnerId: 'h9', humans: 1, bots: 0, conquestsHumans: 0, conquestsBots: 0, attacksSent: 0,
        humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
      rows: [{ rank: 1, playerId: 'h9', name: 'Lis', isBot: false, points: 10, villages: 1, conquered: 0, killed: 0 }] } });
    at('/pl/gracze/Lis');
    render(<App />);
    fireEvent.click(await screen.findByRole('link', { name: 'Gra #7' }));
    expect(await screen.findByRole('heading', { name: 'Gra #7' })).toBeTruthy();
    expect(location.pathname).toBe('/pl/gry/7');
  });

  it('a forgotten winner reads as a former player', async () => {
    routes['GET /api/v1/public/games/8'] = () => ({ status: 200, body: {
      worldId: 8, name: 'Instant #8', startedAt: 0, endedAt: 3_600_000, winnerName: '', winnerIsBot: false,
      summary: { endReason: 'time', winnerId: 'h9', humans: 1, guests: 1, bots: 0, conquestsHumans: 0, conquestsBots: 0, attacksSent: 0,
        humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
      rows: [{ rank: 1, playerId: 'h9', name: '', isBot: false, points: 10, villages: 1, conquered: 0, killed: 0 }] } });
    at('/games/8');
    render(<App />);
    expect(await screen.findByText('Time ran out. Former player finished with the most points.')).toBeTruthy();
  });

  it('privacy page is linked from the landing page', async () => {
    render(<App />);
    await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!);
    const link = screen.getByRole('link', { name: 'Privacy' });
    expect(link.getAttribute('href')).toBe('/privacy');
    fireEvent.click(link);
    expect(location.pathname).toBe('/privacy');
    expect(await screen.findByRole('heading', { name: 'Privacy' })).toBeTruthy();
    expect(await screen.findByText(/random player ID/)).toBeTruthy();
  });
});
