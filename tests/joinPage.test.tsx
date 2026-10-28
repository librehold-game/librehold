// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import type { Account } from '../src/account/api';
import JoinRoute from '../src/account/JoinPage';
import { playNavigation } from '../src/account/play';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };
let routes: Record<string, () => Route>;
let calls: string[];
let assign: ReturnType<typeof vi.spyOn>;

const CODE = 'abcdefghijklmnopqrst';
const info = { name: 'Friday night', host: 'ala', status: 'waiting', humans: 2, seats: 10, open: true, registeredOnly: false };
const player: Account = { id: 5, username: 'ola', isAdmin: false, isGuest: false, locale: 'en', tutorialDone: true };
const guest: Account = { ...player, id: 6, username: 'Guest_6', isGuest: true };

function page(me: Account | null) {
  return render(<JoinRoute params={{ code: CODE }} me={me} onLogin={() => {}} onClaimed={() => {}} />);
}

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  calls = [];
  routes = {};
  assign = vi.spyOn(playNavigation, 'assign').mockImplementation(() => {});
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    calls.push(key);
    const handler = routes[key];
    if (!handler) return new Response(JSON.stringify({ error: { code: 'notFound' } }), { status: 404 });
    const r = handler();
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  assign.mockRestore();
  vi.unstubAllGlobals();
  setLocale('pl');
});

describe('the invite page', () => {
  it('a signed-out visitor becomes a guest and lands in the game', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: info });
    routes['POST /api/v1/auth/guest'] = () => ({ status: 201, body: { account: guest } });
    routes[`POST /api/v1/invites/${CODE}/join`] = () => ({ status: 200, body: { worldId: 7 } });
    page(null);
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=7'));
    expect(calls.indexOf('POST /api/v1/auth/guest')).toBeLessThan(calls.indexOf(`POST /api/v1/invites/${CODE}/join`));
    expect(screen.getByText(/Friday night/)).toBeTruthy();
  });

  it('a full world offers Play now', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: { ...info, open: false, reason: 'worldFull', humans: 10 } });
    page(player);
    expect(await screen.findByText('This game is full.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Play now' })).toBeTruthy();
    expect(calls).not.toContain(`POST /api/v1/invites/${CODE}/join`);
  });

  it('registered players only asks a guest to sign in', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: { ...info, registeredOnly: true } });
    page(guest);
    expect(await screen.findByText('Only registered players may join this game.')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Log in' }).getAttribute('href')).toBe('/login');
    expect(screen.getByRole('link', { name: 'Create an account' }).getAttribute('href')).toBe('/register');
    expect(calls).not.toContain(`POST /api/v1/invites/${CODE}/join`);
  });

  it('an unknown code says the link leads nowhere', async () => {
    page(player);
    expect(await screen.findByText('This link leads nowhere.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Play now' })).toBeTruthy();
  });

  it('the host is sent to the god view', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: info });
    routes[`POST /api/v1/invites/${CODE}/join`] = () => ({ status: 200, body: { worldId: 7, watch: true } });
    page(player);
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/game/?world=7&watch=1'));
    expect(calls).not.toContain('POST /api/v1/auth/guest');
  });

  it('a refused join says why and offers Play now', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: info });
    routes[`POST /api/v1/invites/${CODE}/join`] = () => ({ status: 400, body: { error: { code: 'banned', field: 'world' } } });
    page(player);
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'The host has removed you from this game.');
    expect(screen.getByRole('button', { name: 'Play now' })).toBeTruthy();
    expect(assign).not.toHaveBeenCalled();
  });

  it('the page is noindex', async () => {
    routes[`GET /api/v1/public/invites/${CODE}`] = () => ({ status: 200, body: { ...info, open: false, reason: 'joinClosed' } });
    page(player);
    await screen.findByText('Joining this game has closed.');
    await waitFor(() => expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex'));
    expect(document.title).toContain('Friday night');
  });
});
