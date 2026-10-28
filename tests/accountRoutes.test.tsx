// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };
type Handler = (body: unknown) => Route;

let routes: Record<string, Handler>;

const alice = { id: 2, username: 'ala', isAdmin: false, isGuest: false, locale: 'en' };
const guest = { id: 9, username: 'Player_fox', isAdmin: false, isGuest: true, locale: 'en' };
const signedOut = () => ({ status: 401, body: { error: { code: 'unauthenticated' } } });

beforeEach(() => {
  routes = {
    'GET /api/v1/me': signedOut,
    'GET /api/v1/public/stats': () => ({ status: 200, body: { visible: false } }),
    'GET /api/v1/worlds': () => ({ status: 200, body: { mine: [], open: [] } }),
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    const r = routes[key]?.(body) ?? { status: 404, body: { error: { code: 'notFound' } } };
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  document.cookie = 'lang=; Max-Age=0; Path=/';
  setLocale('pl');
});

const at = (path: string) => history.replaceState(null, '', path);

describe('account routes', () => {
  it('pages not built yet show the 404 page', async () => {
    for (const path of ['/how-to-play/nope', '/units/dragon',
      '/worlds/3', '/nope']) {
      at(path);
      render(<App />);
      expect(await screen.findByRole('heading', { name: 'Page not found' }), path).toBeTruthy();
      cleanup();
    }
  });

  it("a player world's results open by their code", async () => {
    const code = 'resultscoderesultsco';
    routes[`GET /api/v1/public/games/${code}`] = () => ({ status: 200, body: {
      worldId: 7, name: 'Friday night', kind: 'player', startedAt: 0, endedAt: 1, winnerName: 'Ala', winnerIsBot: false,
      summary: { endReason: 'host', winnerId: 'h1', humans: 1, bots: 0, conquestsHumans: 0, conquestsBots: 0, attacksSent: 0,
        humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
      rows: [{ rank: 1, playerId: 'h1', name: 'Ala', isBot: false, points: 10, villages: 1, conquered: 0, killed: 0 }] } });
    at(`/games/${code}`);
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Friday night' })).toBeTruthy();
    cleanup();
    at('/games/nosuchcodenosuchcode');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeTruthy();
  });

  it('a registered account on the front page goes to the lobby', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: alice });
    at('/');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/lobby'));
  });

  it('a guest stays on the front page and sees Continue your game', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: guest });
    at('/');
    render(<App />);
    expect(await screen.findAllByRole('button', { name: 'Continue your game' }).then((b) => b[0]!)).toBeTruthy();
    expect(location.pathname).toBe('/');
  });

  it('signed-out visitors of the lobby and admin land on login', async () => {
    at('/lobby');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/login'));
    cleanup();
    at('/pl/admin');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/pl/logowanie'));
  });

  it("login lands in the lobby of the account's language", async () => {
    routes['POST /api/v1/auth/login'] = () => ({ status: 200, body: { account: { ...alice, locale: 'pl' }, token: 't' } });
    at('/login');
    render(<App />);
    fireEvent.change(await screen.findByLabelText('Username'), { target: { value: 'ala' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.click(screen.getByRole('button', { name: 'Log in' }));
    await waitFor(() => expect(location.pathname).toBe('/pl/lobby'));
  });

  it("the account's language moves the lobby to its twin", async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: alice });
    at('/pl/lobby');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/lobby'));
    await waitFor(() => expect(document.documentElement.lang).toBe('en'));
  });

  it('the language switch goes to the twin', async () => {
    at('/privacy');
    render(<App />);
    await screen.findByRole('heading', { name: 'Privacy' });
    fireEvent.click(screen.getByRole('link', { name: 'PL' }));
    expect(location.pathname).toBe('/pl/prywatnosc');
    expect(await screen.findByRole('heading', { name: 'Prywatność' })).toBeTruthy();
    expect(document.cookie).toContain('lang=pl');
  });

  it('also-in-Polish note', async () => {
    // Node's own localStorage shadows happy-dom's and throws without a backing file.
    const stored = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => stored.get(k) ?? null,
      setItem: (k: string, v: string) => void stored.set(k, v),
      clear: () => stored.clear(),
    });
    const lang = vi.spyOn(navigator, 'language', 'get').mockReturnValue('pl-PL');
    at('/privacy');
    render(<App />);
    const note = await screen.findByRole('link', { name: 'Ta strona jest też po polsku →' });
    expect(note.getAttribute('href')).toBe('/pl/prywatnosc');
    fireEvent.click(screen.getByRole('button', { name: 'Zamknij' }));
    expect(screen.queryByRole('link', { name: 'Ta strona jest też po polsku →' })).toBeNull();
    cleanup();
    render(<App />);
    await screen.findByRole('heading', { name: 'Privacy' });
    expect(screen.queryByRole('link', { name: 'Ta strona jest też po polsku →' })).toBeNull();
    cleanup();
    localStorage.clear();
    document.cookie = 'lang=en; Path=/';
    render(<App />);
    await screen.findByRole('heading', { name: 'Privacy' });
    expect(screen.queryByRole('link', { name: 'Ta strona jest też po polsku →' })).toBeNull();
    cleanup();
    document.cookie = 'lang=; Max-Age=0; Path=/';
    lang.mockReturnValue('en-US');
    render(<App />);
    await screen.findByRole('heading', { name: 'Privacy' });
    expect(screen.queryByRole('link', { name: /also in|po polsku/ })).toBeNull();
  });

  it('links use real addresses', async () => {
    at('/');
    render(<App />);
    await screen.findAllByRole('button', { name: 'Play now' }).then((b) => b[0]!);
    for (const a of screen.getAllByRole('link', { name: 'Create an account' })) expect(a.getAttribute('href')).toBe('/register');
    expect(screen.getByRole('link', { name: 'Privacy' }).getAttribute('href')).toBe('/privacy');
    cleanup();
    at('/pl/');
    render(<App />);
    await screen.findAllByRole('button', { name: 'Graj teraz' }).then((b) => b[0]!);
    for (const a of screen.getAllByRole('link', { name: 'Załóż konto' })) expect(a.getAttribute('href')).toBe('/pl/rejestracja');
    cleanup();

    routes['GET /api/v1/public/profiles/Lis'] = () => ({ status: 200, body: {
      username: 'Lis', isGuest: false, tally: { games: 3, wins: 1, conquered: 7, killed: 4200 },
      games: [{ worldId: 7, name: 'Game #7', endedAt: 1_700_000_000_000, rank: 1, players: 20, points: 900, conquered: 4, killed: 2000 }] } });
    at('/players/Lis');
    render(<App />);
    expect((await screen.findByRole('link', { name: 'Game #7' })).getAttribute('href')).toBe('/games/7');
    cleanup();

    routes['GET /api/v1/public/games/7'] = () => ({ status: 200, body: {
      worldId: 7, name: 'Game #7', startedAt: 0, endedAt: 3_600_000, winnerName: 'Lis', winnerIsBot: false,
      summary: { endReason: 'time', winnerId: 'h9', humans: 1, bots: 0, conquestsHumans: 0, conquestsBots: 0, attacksSent: 0,
        humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
      rows: [{ rank: 1, playerId: 'h9', name: 'Lis', isBot: false, points: 10, villages: 1, conquered: 0, killed: 0 }] } });
    at('/games/7');
    render(<App />);
    expect((await screen.findByRole('link', { name: 'Lis' })).getAttribute('href')).toBe('/players/Lis');
  });
});
