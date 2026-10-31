// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { setLocale, t } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };

let routes: Record<string, () => Route>;

const card = { id: 3, name: 'Kraina', instant: false, status: 'open', paused: false, startedAt: 0, day: 21, humans: 4, bots: 40, speed: 8 };
const longSettings = {
  worldSpeed: 8, unitSpeed: 1, mapSize: 200, beginnerProtectionDays: 7, lastVillage: 'protected', fieldBattles: true,
  barbarianDensity: 'normal', battleLosses: 'normal', botCount: 40, botArrival: 'rampUp', rampUpDays: 14, difficulty: 'hard',
  botStance: 'normal', humanModifiers: { resources: 1.5, recruitment: 1.2, building: 0.8 }, botModifiers: { resources: 1, recruitment: 1, building: 1 },
  sightRadius: 2,
};
const world = { ...card, ended: false, settings: longSettings, events: [] };
const instantCard = { ...card, id: 41, name: 'Game #41', gameNumber: 41, instant: true, day: undefined, bots: 18, humans: 3, speed: 100 };
const instantWorld = {
  ...instantCard, ended: false, events: [],
  settings: { ...longSettings, durationMin: 60, humanSeats: 12, dominationShare: 0.6, noCoins: true, nobleCost: { wood: 100, clay: 200, iron: 300 },
    ownedNobleDropMin: 10, ownedNobleDropMax: 20, academyExtraNobles: 2, botArrival: 'allAtOnce', rampUpDays: undefined },
};

const player = (i: number, over: Record<string, unknown> = {}) => ({
  id: `h${i}`, name: `Player ${i}`, color: '#abc', points: 1000 - i, villages: 3, att: 0, def: 0, sup: 0, quit: false, isBot: false, ...over,
});
const profile = (over: Record<string, unknown> = {}) => ({
  player: { id: 'h2', name: 'Ala', color: '#d23b2f', status: 'active', points: 1234, villageIds: ['v1'], isBot: false },
  villages: [{ id: 'v1', name: 'Ala 1', x: 12, y: 34, points: 321 }], history: { hourly: [], daily: [1, 5, 9] },
  username: 'Ala ma/kota', rank: 2,
  conquests: [{ t: Date.UTC(2026, 9, 30, 20, 15), x: 5, y: 6, toId: 'h2', to: 'Ala', fromId: null, from: null, toIsBot: false, fromIsBot: false }],
  ...over,
});

beforeEach(() => {
  setLocale('en');
  routes = {
    'GET /api/v1/me': () => ({ status: 401, body: { error: { code: 'unauthenticated' } } }),
    'GET /api/v1/public/worlds/3': () => ({ status: 200, body: world }),
    'GET /api/v1/public/worlds/3/villages': () => ({ status: 200, body: { size: 200, players: [], villages: [] } }),
    'GET /api/v1/public/worlds/3/ranking?tab=points': () => ({ status: 200, body: Array.from({ length: 120 }, (_, i) => player(i + 1, i === 1 ? { isBot: true } : {})) }),
    'GET /api/v1/public/worlds/3/ranking?tab=daily': () => ({ status: 200, body: [] }),
    'GET /api/v1/public/worlds/3/ranking?tab=conquests': () => ({ status: 200, body: [] }),
    'GET /api/v1/public/worlds/3/players/h2': () => ({ status: 200, body: profile() }),
    'GET /api/v1/public/worlds/41': () => ({ status: 200, body: instantWorld }),
    'GET /api/v1/public/worlds/41/villages': () => ({ status: 200, body: { size: 100, players: [], villages: [] } }),
    'GET /api/v1/public/worlds/41/ranking?tab=points': () => ({ status: 200, body: [] }),
    'GET /api/v1/public/worlds/41/ranking?tab=daily': () => ({ status: 200, body: [] }),
    'GET /api/v1/public/worlds/41/ranking?tab=conquests': () => ({ status: 200, body: [] }),
  };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const r = routes[`${init?.method ?? 'GET'} ${url}`]?.() ?? { status: 404, body: { error: { code: 'notFound' } } };
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
const settingsRegion = () => screen.getByRole('region', { name: t('site.worlds.settings') });

describe('the world page', () => {
  it('lists every setting group', async () => {
    at('/worlds/3');
    render(<App />);
    await screen.findByRole('heading', { level: 1, name: 'Kraina' });
    const region = settingsRegion();
    expect(within(region).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(
      ['pace', 'map', 'rules', 'modifiers'].map((g) => t(`site.worlds.group.${g}`)));
    expect(region.textContent).toContain('×8');
    expect(region.textContent).toContain('40');
    expect(region.textContent).toContain('×1.5');
    expect(region.textContent).toContain('×0.8');
    expect(region.textContent).toContain(t('admin.choice.difficulty.hard'));
    expect(region.textContent).not.toContain(t('site.worlds.setting.coins'));
    expect(region.textContent).not.toContain(t('site.worlds.setting.nobleCost'));
  });

  it('shows the instant-only settings of a running game', async () => {
    at('/worlds/41');
    render(<App />);
    await screen.findByRole('heading', { level: 1, name: 'Game #41' });
    const text = settingsRegion().textContent!;
    for (const key of ['duration', 'seats', 'dominationShare', 'coins', 'nobleCost', 'nobleLoyalty', 'academyNobles']) {
      expect(text).toContain(t(`site.worlds.setting.${key}`));
    }
    expect(text).toContain('60%');
    expect(text).toContain('100 · 200 · 300');
    expect(text).toContain('10–20');
  });

  it('ranks everyone, 50 a page, and links names to the player page', async () => {
    at('/worlds/3');
    const { container } = render(<App />);
    await screen.findByRole('link', { name: 'Player 1' });
    expect(container.querySelectorAll('.ranking-table tbody tr')).toHaveLength(50);
    expect(screen.getByRole('link', { name: 'Player 1' }).getAttribute('href')).toBe('/worlds/3/players/h1');
    expect(container.querySelectorAll('.ranking-table .bot-tag')).toHaveLength(1);
    expect(screen.getByText(t('ranking.page', { n: 1, pages: 3 }))).toBeTruthy();
  });

  it('shows conquest times as dates', async () => {
    const when = Date.UTC(2026, 9, 30, 20, 15);
    routes['GET /api/v1/public/worlds/3/ranking?tab=conquests'] = () => ({ status: 200, body: [
      { t: when, x: 5, y: 6, toId: 'h2', to: 'Ala', fromId: null, from: null, toIsBot: false, fromIsBot: false }] });
    at('/worlds/3');
    const { container } = render(<App />);
    await screen.findByRole('link', { name: 'Player 1' });
    fireEvent.click(screen.getByText(t('ranking.tab.conquests')));
    const expected = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' }).format(when);
    expect(container.textContent).toContain(expected);
    expect(container.textContent).not.toContain('1970');
  });

  it("sends a link to a game that has ended on to the game's page", async () => {
    routes['GET /api/v1/public/worlds/5'] = () => ({ status: 404, body: { error: { code: 'notFound' } } });
    routes['GET /api/v1/public/games/5'] = () => ({ status: 200, body: { worldId: 5, name: 'Game #5', rows: [], summary: {} } });
    at('/worlds/5');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/games/5'));
  });

  it('shows the not-found page for a world and a game that do not exist', async () => {
    at('/worlds/999');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeTruthy();
  });
});

describe('the player page', () => {
  it("sends a player link of a game that has ended on to the game's page", async () => {
    routes['GET /api/v1/public/games/5'] = () => ({ status: 200, body: { worldId: 5, name: 'Game #5', rows: [], summary: {} } });
    at('/worlds/5/players/h1');
    render(<App />);
    await waitFor(() => expect(location.pathname).toBe('/games/5'));
  });

  it('shows the villages and conquests and links to the global profile', async () => {
    at('/worlds/3/players/h2');
    const { container } = render(<App />);
    await screen.findByRole('heading', { level: 1 });
    expect(container.textContent).toContain('Ala 1');
    expect(container.textContent).toContain('12|34');
    expect(container.textContent).toContain(new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' }).format(Date.UTC(2026, 9, 30, 20, 15)));
    expect(screen.getByRole('link', { name: new RegExp('Kraina') }).getAttribute('href')).toBe('/worlds/3');
    expect(screen.getByRole('link', { name: t('site.worlds.globalProfile') }).getAttribute('href')).toBe('/players/Ala%20ma%2Fkota');
    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toContain('noindex');
  });

  it('has no global profile link for a bot or for a human without an account', async () => {
    routes['GET /api/v1/public/worlds/3/players/h2'] = () => ({ status: 200, body: profile({ username: undefined, player: { ...profile().player, isBot: true } }) });
    at('/worlds/3/players/h2');
    const { container } = render(<App />);
    await screen.findByRole('heading', { level: 1 });
    expect(screen.queryByRole('link', { name: t('site.worlds.globalProfile') })).toBeNull();
    expect(container.querySelector('.bot-tag')).toBeTruthy();
    cleanup();
    routes['GET /api/v1/public/worlds/3/players/h2'] = () => ({ status: 200, body: profile({ username: undefined }) });
    render(<App />);
    await screen.findByRole('heading', { level: 1 });
    expect(screen.queryByRole('link', { name: t('site.worlds.globalProfile') })).toBeNull();
  });

  it('shows the not-found page for a player who is not there', async () => {
    at('/worlds/3/players/nobody');
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeTruthy();
  });
});
