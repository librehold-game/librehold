// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };

let routes: Record<string, () => Route>;
let requested: string[];
const now = Date.now();

const long = { id: 3, name: 'Kraina', instant: false, status: 'open', paused: false, startedAt: now - 20 * 86_400_000, day: 21,
  humans: 4, bots: 30, speed: 8 };
const running = { id: 41, name: 'Game #41', gameNumber: 41, instant: true, status: 'open', paused: false, startedAt: now - 600_000,
  endsAt: now + 50 * 60_000, humans: 3, bots: 18, speed: 100, openSeats: 9 };
const world = {
  ...long, ended: false,
  settings: { worldSpeed: 8, unitSpeed: 1, mapSize: 200, beginnerProtectionDays: 7, lastVillage: 'protected', fieldBattles: true,
    barbarianDensity: 'normal', battleLosses: 'normal', botCount: 30, botArrival: 'allAtOnce', difficulty: 'normal', botStance: 'normal',
    humanModifiers: { resources: 1, recruitment: 1, building: 1 }, botModifiers: { resources: 1, recruitment: 1, building: 1 }, sightRadius: 1 },
  events: [{ at: now - 60_000 * 5, kind: 'battle', worldId: 3, worldName: 'Kraina', instant: false, attacker: 'Ala', defender: 'Wolf',
    attackerBot: false, defenderBot: true, troops: 300, x: 1, y: 2 }],
};
const map = { size: 200, players: [{ name: 'Ala', color: '#ff0000', isBot: false }], villages: [[1, 2, 0], [5, 5, -1]] };

beforeEach(() => {
  setLocale('en');
  requested = [];
  routes = {
    'GET /api/v1/me': () => ({ status: 401, body: { error: { code: 'unauthenticated' } } }),
    'GET /api/v1/public/worlds': () => ({ status: 200, body: { running: [running], long: [long],
      finished: [{ worldId: 40, name: 'Game #40', gameNumber: 40, endedAt: now - 86_400_000 }] } }),
    'GET /api/v1/public/worlds/3': () => ({ status: 200, body: world }),
    'GET /api/v1/public/worlds/3/villages': () => ({ status: 200, body: map }),
    'GET /api/v1/public/worlds/3/ranking?tab=points': () => ({ status: 200, body: [
      { id: 'h1', name: 'Ala', color: '#f00', points: 900, villages: 3, att: 0, def: 0, sup: 0, quit: false, isBot: false },
      { id: 'b1', name: 'Wolf', color: '#00f', points: 800, villages: 2, att: 0, def: 0, sup: 0, quit: false, isBot: true }] }),
    'GET /api/v1/public/worlds/3/ranking?tab=daily': () => ({ status: 200, body: [] }),
    'GET /api/v1/public/worlds/3/ranking?tab=conquests': () => ({ status: 200, body: [] }),
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    requested.push(key);
    const r = routes[key]?.() ?? { status: 404, body: { error: { code: 'notFound' } } };
    return new Response(JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const at = (path: string) => history.replaceState(null, '', path);

describe('worlds pages', () => {
  it('the list shows running games, long worlds and finished games with links', async () => {
    at('/worlds');
    render(<App />);
    const game = await screen.findByRole('link', { name: 'Game #41' });
    expect(game.getAttribute('href')).toBe('/worlds/41');
    expect(screen.getByRole('link', { name: 'Kraina' }).getAttribute('href')).toBe('/worlds/3');
    expect(screen.getByRole('link', { name: 'Game #40' }).getAttribute('href')).toBe('/games/40');
    const runningSection = screen.getByRole('region', { name: 'Instant games now' });
    expect(runningSection.textContent).toContain('3 players · 18 bots');
    expect(runningSection.textContent).toMatch(/49 min|50 min/);
    expect(screen.getByRole('region', { name: 'Long worlds' }).textContent).toContain('day 21');
  });

  it('empty sections say so', async () => {
    routes['GET /api/v1/public/worlds'] = () => ({ status: 200, body: { running: [], long: [], finished: [] } });
    at('/worlds');
    render(<App />);
    expect(await screen.findByText('No instant game is running right now.')).toBeTruthy();
    expect(screen.getByText('No long world yet.')).toBeTruthy();
    expect(screen.getByText('No finished game yet.')).toBeTruthy();
  });

  it('a world page shows settings, the ranking with bot tags, the map box and events', async () => {
    at('/worlds/3');
    const { container } = render(<App />);
    expect(await screen.findByRole('heading', { level: 1, name: 'Kraina' })).toBeTruthy();
    const settings = screen.getByRole('region', { name: 'Settings' });
    expect(settings.textContent).toContain('×8');
    expect(settings.textContent).toContain('200×200');
    const ranking = await screen.findByRole('table');
    expect(within(ranking).getAllByRole('row')).toHaveLength(3);
    expect(ranking.querySelectorAll('.bot-tag')).toHaveLength(1);
    expect(await screen.findByRole('img', { name: 'Map of Kraina' })).toBeTruthy();
    expect(container.querySelector('.event-list li')?.textContent).toContain('Ala attacked Wolf (bot)');
    expect(document.title).toContain('Kraina');
  });

  // Review focus 5: a page for a world that is not there, or for an address that is not a world.
  it('an unknown world shows the not-found page', async () => {
    at('/worlds/999');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeTruthy();
    cleanup();
    requested = [];
    at('/worlds/abc');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeTruthy();
    expect(requested.some((r) => r.includes('abc'))).toBe(false);
  });

  it('Polish links use Polish addresses and Gra #41', async () => {
    at('/pl/swiaty');
    render(<App />);
    const game = await screen.findByRole('link', { name: 'Gra #41' });
    expect(game.getAttribute('href')).toBe('/pl/swiaty/41');
    expect(screen.getByRole('link', { name: 'Kraina' }).getAttribute('href')).toBe('/pl/swiaty/3');
  });
});
