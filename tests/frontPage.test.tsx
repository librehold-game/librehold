// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from '../src/account/App';
import facts from '../src/account/content/facts.json';
import '../src/account/pagesEager';
import { playNavigation } from '../src/account/play';
import { RegisterPage } from '../src/account/RegisterPage';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };
type Handler = (body: unknown) => Route;

let routes: Record<string, Handler>;
let calls: { key: string; body: unknown }[];

const guest = { id: 9, username: 'Player_fox', isAdmin: false, isGuest: true, locale: 'en', tutorialDone: true };
const alice = { id: 2, username: 'ala', isAdmin: false, isGuest: false, locale: 'en' as const, tutorialDone: true };
const signedOut = () => ({ status: 401, body: { error: { code: 'unauthenticated' } } });
const card = (over: Record<string, unknown>) => ({ id: 3, name: 'Kraina', instant: false, status: 'open', paused: false,
  startedAt: Date.UTC(2026, 9, 1), day: 21, humans: 4, bots: 30, speed: 8, ...over });
const live = { games: 1, humans: 2, bots: 18, openSeats: 10, lastBattleAt: null, events: [] };
const visibleStats = { visible: true, allTime: { games: 30, players: 20, conquests: 40, unitsKilled: 500 },
  last24h: { games: 3, players: 2, conquests: 4, unitsKilled: 50 } };

function stubWorlds(long: unknown[]) {
  routes['GET /api/v1/public/worlds'] = () => ({ status: 200, body: { running: [], long, finished: [] } });
}

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  calls = [];
  routes = {
    'GET /api/v1/me': signedOut,
    'GET /api/v1/public/stats': () => ({ status: 200, body: visibleStats }),
    'GET /api/v1/public/live': () => ({ status: 200, body: live }),
  };
  stubWorlds([card({})]);
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ key, body });
    const r = routes[key]?.(body) ?? { status: 404, body: { error: { code: 'notFound' } } };
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const before = (a: Element, b: Element) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;

describe('front page', () => {
  it('sections come in the designed order', async () => {
    render(<App />);
    const stats = await screen.findByRole('region', { name: 'Instant games so far' });
    const order = [
      screen.getByRole('heading', { level: 1 }),
      screen.getByRole('region', { name: 'Live now' }),
      screen.getByRole('heading', { name: 'In the game' }),
      await screen.findByRole('region', { name: 'Open worlds' }),
      stats,
      screen.getByRole('heading', { name: 'FAQ' }),
    ];
    for (let i = 1; i < order.length; i++) expect(before(order[i - 1]!, order[i]!), `section ${i}`).toBe(true);
  });

  it('the headline names the genre and Play now is the only primary button', async () => {
    render(<App />);
    const h1 = await screen.findByRole('heading', { level: 1 });
    expect(h1.textContent).toBe('Village wars in your browser');
    const primary = [...document.querySelectorAll('button.cta, a.cta')];
    expect(primary).toHaveLength(1);
    expect(primary[0]!.textContent).toBe('Play now');
    expect(screen.getAllByRole('link', { name: 'Create an account' }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('No sign-up. Works on your phone.')).toBeTruthy();
  });

  it('the hero gives the instant game\'s numbers from the facts', async () => {
    render(<App />);
    const pitch = (await screen.findByRole('heading', { level: 1 })).parentElement!;
    for (const part of [`${facts.instantSeats} players`, `${facts.instantBots} bots`, `${facts.instantStartAxe} axemen`,
      `${facts.instantStartLightCav} light cavalry`, `${facts.instantDominationPct}%`]) expect(pitch.textContent).toContain(part);
  });

  it('a guest sees Continue your game', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: guest });
    render(<App />);
    expect((await screen.findAllByRole('button', { name: 'Continue your game' })).length).toBeGreaterThanOrEqual(1);
  });

  it('the game is described by its features', async () => {
    render(<App />);
    const section = (await screen.findByRole('heading', { name: 'In the game' })).parentElement!;
    expect(within(section).getAllByRole('heading', { level: 3 }).map((h) => h.textContent))
      .toEqual(['Village', 'Army', 'Reports', 'Incoming attacks']);
  });

  it('each promise is said once', async () => {
    render(<App />);
    await screen.findByRole('heading', { name: 'FAQ' });
    const text = document.body.textContent!;
    expect(text.split(/never resources, speed or troops/i).length - 1).toBe(1);
    expect(text.split(/phone/i).length - 1).toBe(1);
  });

  // Review focus 4: closed worlds and paused ones take no new players.
  it('open worlds show only open long worlds and hide when there are none', async () => {
    stubWorlds([card({}), card({ id: 4, name: 'Closed', status: 'closed' }), card({ id: 5, name: 'Dormant', paused: true })]);
    render(<App />);
    const region = await screen.findByRole('region', { name: 'Open worlds' });
    expect(region.textContent).toContain('Kraina');
    expect(region.textContent).not.toContain('Closed');
    expect(region.textContent).not.toContain('Dormant');
    expect(region.textContent).toContain('4 players · 30 bots');
    expect(within(region).getByRole('link', { name: 'Kraina' }).getAttribute('href')).toBe('/worlds/3');
    cleanup();
    stubWorlds([card({ status: 'closed' })]);
    render(<App />);
    await screen.findByRole('heading', { name: 'FAQ' });
    await waitFor(() => expect(calls.filter((c) => c.key === 'GET /api/v1/public/worlds')).toHaveLength(2));
    expect(screen.queryByRole('region', { name: 'Open worlds' })).toBeNull();
  });

  it('the Polish front page links the world title to the Polish world page', async () => {
    stubWorlds([card({})]);
    history.replaceState(null, '', '/pl/');
    render(<App />);
    const region = await screen.findByRole('region', { name: 'Otwarte światy' });
    expect(within(region).getByRole('link', { name: 'Kraina' }).getAttribute('href')).toBe('/pl/swiaty/3');
  });

  it('Join while signed out goes to registration with the world', async () => {
    render(<App />);
    const join = await screen.findByRole('link', { name: 'Join Kraina' });
    expect(join.getAttribute('href')).toBe('/register?world=3');
  });

  it('Join while signed in joins and opens the game', async () => {
    const assign = vi.spyOn(playNavigation, 'assign').mockImplementation(() => {});
    routes['GET /api/v1/me'] = () => ({ status: 200, body: alice });
    routes['POST /api/v1/worlds/3/join'] = () => ({ status: 200, body: { playerId: 'h1' } });
    history.replaceState(null, '', '/');
    render(<App />);
    // A registered account is sent from the front page to the lobby; the card works the same wherever it is shown.
    await waitFor(() => expect(location.pathname).toBe('/lobby'));
    cleanup();
    const { OpenWorlds } = await import('../src/account/OpenWorlds');
    render(<OpenWorlds me={alice} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Join Kraina' }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=3'));
    expect(calls.find((c) => c.key === 'POST /api/v1/worlds/3/join')!.body).toEqual({ quadrant: 'random' });
  });

  it('registering with a world joins it straight away', async () => {
    const assign = vi.spyOn(playNavigation, 'assign').mockImplementation(() => {});
    routes['POST /api/v1/auth/register'] = () => ({ status: 201, body: { account: alice, token: 't' } });
    routes['POST /api/v1/worlds/3/join'] = () => ({ status: 200, body: { playerId: 'h1' } });
    history.replaceState(null, '', '/register?world=3');
    const onLogin = vi.fn();
    render(<RegisterPage onLogin={onLogin} />);
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'ala' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.change(screen.getByLabelText('Repeat the password'), { target: { value: 'dlugie-haslo' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create an account' }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=3'));
    expect(calls.find((c) => c.key === 'POST /api/v1/worlds/3/join')!.body).toEqual({ quadrant: 'random' });
    expect(onLogin).not.toHaveBeenCalled();
  });

  it('with reduced motion only the poster shows', async () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {} }));
    const { container } = render(<App />);
    await screen.findByRole('heading', { level: 1 });
    const poster = container.querySelector('.clip img')!;
    expect(poster.getAttribute('width')).toBe('960');
    expect(poster.getAttribute('height')).toBe('540');
    expect(container.querySelector('.clip video')).toBeNull();
  });

  it('otherwise a muted looping inline video with webm and mp4', async () => {
    const { container } = render(<App />);
    await waitFor(() => expect(container.querySelector('.clip video')).not.toBeNull());
    const video = container.querySelector('.clip video') as HTMLVideoElement;
    expect(video.muted).toBe(true);
    for (const a of ['autoplay', 'loop', 'playsinline', 'poster']) expect(video.hasAttribute(a), a).toBe(true);
    expect([...video.querySelectorAll('source')].map((s) => s.getAttribute('type'))).toEqual(['video/webm', 'video/mp4']);
    // The poster stays under the video: it is the largest paint and should not be swapped out.
    expect(container.querySelector('.clip img')?.getAttribute('fetchpriority')).toBe('high');
  });

  it('the login form is not on the front page', async () => {
    render(<App />);
    await screen.findByRole('heading', { level: 1 });
    expect(screen.queryByLabelText('Password')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Log in' })).toBeNull();
  });
});
