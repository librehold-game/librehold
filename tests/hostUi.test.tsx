// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { Account, HostedCard } from '../src/account/api';
import { AdminPage } from '../src/account/admin/AdminPage';
import { LobbyPage } from '../src/account/LobbyPage';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

vi.mock('qrcode-generator', () => ({
  default: () => ({ addData() {}, make() {}, getModuleCount: () => 3, isDark: (r: number, c: number) => r === c }),
}));

type Route = { status: number; body?: unknown };
let routes: Record<string, (body: unknown) => Route>;
let calls: { key: string; body: unknown }[];

const ala: Account = { id: 2, username: 'ala', isAdmin: false, isGuest: false, locale: 'en', tutorialDone: true };
const limits = { canHost: true, maxSeats: 10, maxBots: 5, full: false };
const settings = { worldSpeed: 100 };

function card(over: Partial<HostedCard> = {}): HostedCard {
  return {
    worldId: 7, name: "ala's game", status: 'waiting', humans: 1, seats: 10, bots: 3, inviteCode: 'abcdefghijklmnopqrst',
    registeredOnly: false, pauseLeftMs: 30 * 60_000, players: [{ id: 'h101', name: 'gracz1', isGuest: false }], settings,
    ...over,
  } as HostedCard;
}

function lobby(over: Record<string, unknown> = {}) {
  return { mine: [], open: [], playing: [], hosted: null, host: limits, pastHosted: [], ...over };
}

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  calls = [];
  routes = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ key, body });
    const handler = routes[key];
    if (!handler) return new Response(JSON.stringify({ error: { code: 'notFound' } }), { status: 404 });
    const r = handler(body);
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  setLocale('pl');
});

const posted = (key: string) => calls.find((c) => c.key === key);

describe('hosting from the lobby', () => {
  it('a registered player hosts a game with the defaults', async () => {
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: lobby() });
    routes['POST /api/v1/hosted'] = () => ({ status: 201, body: card({ humans: 0, players: [] }) });
    render(<LobbyPage me={ala} onClaimed={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Host a game' }));
    expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe("ala's game");
    fireEvent.click(screen.getByRole('button', { name: 'Create the game' }));
    await waitFor(() => expect(posted('POST /api/v1/hosted')).toBeTruthy());
    expect(posted('POST /api/v1/hosted')!.body).toMatchObject({
      name: "ala's game", durationMin: 60, seats: 10, bots: 3, sightRadius: 5, dominationPct: 60, lateJoinMin: 10,
      fieldBattles: true, startVillage: 'grown', barbarians: 'normal', registeredOnly: false,
    });
    const link = await screen.findByLabelText('Invite link');
    expect((link as HTMLInputElement).value).toBe(`${location.origin}/join/abcdefghijklmnopqrst`);
  });

  it('guests see no host form', async () => {
    render(<LobbyPage me={{ ...ala, isGuest: true }} onClaimed={() => {}} />);
    await screen.findByRole('heading', { name: 'Instant game' });
    expect(screen.queryByRole('button', { name: 'Host a game' })).toBeNull();
  });

  it('server full shows the try-again message', async () => {
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: lobby() });
    routes['POST /api/v1/hosted'] = () => ({ status: 503, body: { error: { code: 'serverFull' } } });
    render(<LobbyPage me={ala} onClaimed={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Host a game' }));
    fireEvent.click(screen.getByRole('button', { name: 'Create the game' }));
    expect((await screen.findByRole('alert')).textContent).toBe('The server is full. Try again in a few minutes.');
  });

  it('the panel copies, shows a QR code and replaces the link after confirming', async () => {
    const writeText = vi.fn(async () => undefined);
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    const confirm = vi.fn(() => false);
    vi.stubGlobal('confirm', confirm);
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: lobby({ hosted: card() }) });
    routes['POST /api/v1/worlds/7/host/invite'] = () => ({ status: 200, body: { inviteCode: 'zyxwvutsrqponmlkjihg' } });
    render(<LobbyPage me={ala} onClaimed={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Copy' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(`${location.origin}/join/abcdefghijklmnopqrst`));
    expect(await screen.findByText('Copied')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Show QR code' }));
    const qr = await screen.findByRole('img', { name: 'QR code of the invite link' });
    expect(qr.querySelectorAll('rect')).toHaveLength(3);
    fireEvent.click(screen.getByRole('button', { name: 'Replace the link' }));
    expect(confirm).toHaveBeenCalled();
    expect(posted('POST /api/v1/worlds/7/host/invite')).toBeUndefined();
    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: 'Replace the link' }));
    await waitFor(() => expect((screen.getByLabelText('Invite link') as HTMLInputElement).value).toBe(
      `${location.origin}/join/zyxwvutsrqponmlkjihg`));
  });

  it('Start, Pause, End now and Kick post to the host routes', async () => {
    vi.stubGlobal('confirm', () => true);
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: lobby({ hosted: card() }) });
    routes['POST /api/v1/worlds/7/host/start'] = () => ({ status: 200, body: card({ status: 'running' }) });
    routes['POST /api/v1/worlds/7/host/pause'] = () => ({ status: 200, body: card({ status: 'paused', pauseEndsAt: Date.now() + 600_000 }) });
    routes['POST /api/v1/worlds/7/host/kick'] = () => ({ status: 200, body: card({ status: 'paused', humans: 0, players: [] }) });
    routes['POST /api/v1/worlds/7/host/end'] = () => ({ status: 200, body: card({ status: 'ended', humans: 0, players: [], resultsCode: 'resultsresultsresult' }) });
    render(<LobbyPage me={ala} onClaimed={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Start' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Pause (30 min left)' }));
    expect(await screen.findByRole('button', { name: 'Resume' })).toBeTruthy();
    const players = screen.getByRole('list', { name: 'Players' });
    fireEvent.click(within(players).getByRole('button', { name: 'Kick gracz1' }));
    await waitFor(() => expect(posted('POST /api/v1/worlds/7/host/kick')?.body).toEqual({ playerId: 'h101' }));
    fireEvent.click(screen.getByRole('button', { name: 'End now' }));
    const results = await screen.findByRole('link', { name: 'Results' });
    expect(results.getAttribute('href')).toBe('/games/resultsresultsresult');
    expect(calls.map((c) => c.key).filter((k) => k.includes('/host/'))).toEqual([
      'POST /api/v1/worlds/7/host/start', 'POST /api/v1/worlds/7/host/pause', 'POST /api/v1/worlds/7/host/kick',
      'POST /api/v1/worlds/7/host/end',
    ]);
    expect(screen.getByRole('link', { name: 'Open the god view' }).getAttribute('href')).toBe('/game/?world=7&watch=1');
  });

  it('lists past hosted games and the player worlds the account plays in', async () => {
    routes['GET /api/v1/worlds'] = () => ({ status: 200, body: lobby({
      pastHosted: [{ worldId: 5, name: 'Old Friday', code: 'pastpastpastpastpast', endedAt: 1, rank: 0, players: 4, points: 0, conquered: 0, killed: 0 }],
      playing: [{ id: 9, name: 'Ola\'s game', humans: 3, bots: 2, paused: false, settings }],
    }) });
    render(<LobbyPage me={ala} onClaimed={() => {}} />);
    expect((await screen.findByRole('link', { name: 'Old Friday' })).getAttribute('href')).toBe('/games/pastpastpastpastpast');
    expect(screen.getByText("Ola's game")).toBeTruthy();
  });
});

describe('admin limits for player worlds', () => {
  it('admin saves seats and bots', async () => {
    routes['GET /api/v1/admin/accounts'] = () => ({ status: 200, body: [] });
    routes['GET /api/v1/admin/host-settings'] = () => ({ status: 200, body: { maxSeats: 10, maxBots: 5 } });
    routes['PUT /api/v1/admin/host-settings'] = (body) => ({ status: 200, body });
    render(<AdminPage />);
    fireEvent.click(screen.getByRole('tab', { name: 'Player worlds' }));
    const seats = await screen.findByLabelText('Seats per world');
    await waitFor(() => expect((seats as HTMLInputElement).value).toBe('10'));
    fireEvent.change(seats, { target: { value: '20' } });
    fireEvent.change(screen.getByLabelText('Bots per world'), { target: { value: '8' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(posted('PUT /api/v1/admin/host-settings')?.body).toEqual({ maxSeats: 20, maxBots: 8 }));
    expect(await screen.findByText('Saved')).toBeTruthy();
  });
});

describe('admin worlds list', () => {
  it('an ended world shows why it ended and has no pause or open/close', async () => {
    const world = { status: 'open', humans: 0, bots: 3, paused: false, startedAt: 0, settings };
    routes['GET /api/v1/admin/accounts'] = () => ({ status: 200, body: [] });
    routes['GET /api/v1/admin/worlds'] = () => ({ status: 200, body: [
      { ...world, id: 1, name: 'W1' },
      { ...world, id: 2, name: 'Game #3', ended: { at: Date.UTC(2026, 9, 29, 8, 37), reason: 'abandoned' } },
    ] });
    render(<AdminPage />);
    fireEvent.click(screen.getByRole('tab', { name: 'Worlds' }));
    const running = within((await screen.findByText('W1')).closest('tr')!);
    expect(running.getByText('running')).toBeTruthy();
    expect(running.getByRole('button', { name: 'Pause' })).toBeTruthy();
    const ended = within(screen.getByText('Game #3').closest('tr')!);
    expect(ended.getByText('ended')).toBeTruthy();
    expect(ended.getByText(/everyone left/)).toBeTruthy();
    expect(ended.queryByText('running')).toBeNull();
    for (const name of ['Pause', 'Resume', 'Close', 'Open']) expect(ended.queryByRole('button', { name })).toBeNull();
  });
});
