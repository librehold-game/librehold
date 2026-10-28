// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App as SiteApp } from '../src/account/App';
import '../src/account/pagesEager';
import { BUILD_VERSION } from '../src/net/report';
import type { GameState } from '../src/engine/types';
import { setLocale } from '../src/i18n/i18n';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { App as GameApp } from '../src/ui/App';
import { newTestGame } from './helpers';
import { stubServer } from './serverStub';

type Route = { status: number; body?: unknown };
let routes: Record<string, (body: unknown) => Route>;
let calls: { key: string; body: unknown }[];
const alice = { id: 2, username: 'ala', isAdmin: false, isGuest: false, locale: 'en', tutorialDone: true };
const admin = { ...alice, id: 1, username: 'admin', isAdmin: true };
const TEXT = 'The map stays black after I log in.';

function stubSite() {
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ key, body });
    const r = routes[key]?.(body) ?? { status: 404, body: { error: { code: 'notFound' } } };
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
}

beforeEach(() => {
  setLocale('en');
  calls = [];
  routes = {
    'GET /api/v1/me': () => ({ status: 401, body: { error: { code: 'unauthenticated' } } }),
    'POST /api/v1/reports': () => ({ status: 201, body: { id: 7 } }),
  };
});

afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

async function openSiteForm() {
  stubSite();
  history.replaceState(null, '', '/report');
  render(<SiteApp />);
  return screen.findByRole('button', { name: 'Send' });
}

const fill = (text: string, email = '') => {
  fireEvent.change(screen.getByLabelText('What is it about?'), { target: { value: 'bug' } });
  fireEvent.change(screen.getByLabelText('What happened?'), { target: { value: text } });
  if (email) fireEvent.change(screen.getByLabelText('Email (optional)'), { target: { value: email } });
};

describe('report form on the site', () => {
  it('the site form posts kind, text and email', async () => {
    const send = await openSiteForm();
    expect(screen.getByText(/without an account/)).toBeTruthy();
    fill(TEXT, 'a@b.cd');
    fireEvent.click(send);
    expect(await screen.findByText(/Thank you/)).toBeTruthy();
    expect(calls.find((c) => c.key === 'POST /api/v1/reports')!.body).toEqual({ kind: 'bug', text: TEXT, email: 'a@b.cd' });
  });

  it('a short text cannot be sent', async () => {
    const send = await openSiteForm();
    fill('too short');
    expect(send.hasAttribute('disabled')).toBe(true);
    fill('ąęółżźćńśą');
    expect(send.hasAttribute('disabled')).toBe(false);
  });

  it('sending twice quickly posts once', async () => {
    let release!: () => void;
    routes['POST /api/v1/reports'] = () => ({ status: 201, body: { id: 7 } });
    const send = await openSiteForm();
    fill(TEXT);
    const slow = new Promise<void>((r) => { release = r; });
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    const inner = fetchMock.getMockImplementation() as (url: string, init?: RequestInit) => Promise<Response>;
    fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
      if (String(url) === '/api/v1/reports') await slow;
      return inner(url, init);
    });
    fireEvent.click(send);
    fireEvent.click(send);
    await act(async () => release());
    await screen.findByText(/Thank you/);
    expect(calls.filter((c) => c.key === 'POST /api/v1/reports')).toHaveLength(1);
  });

  it('too many reports reads in words', async () => {
    routes['POST /api/v1/reports'] = () => ({ status: 429, body: { error: { code: 'tooManyReports' } } });
    const send = await openSiteForm();
    fill(TEXT);
    fireEvent.click(send);
    expect((await screen.findByRole('alert')).textContent).toBe("You've sent five reports in the last hour. Try again later.");
  });

  it('a signed-in reader is told the report carries the account', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: alice });
    await openSiteForm();
    expect(await screen.findByText(/as ala/)).toBeTruthy();
  });
});

describe('report dialog in the game', () => {
  let g: GameState;
  let bodies: unknown[];

  async function boot() {
    g = newTestGame();
    bodies = [];
    stubServer(g, { routes: { 'POST /api/v1/reports': (b) => { bodies.push(b); return { status: 201, body: { id: 3 } }; } } });
    render(<GameApp />);
    await screen.findByRole('banner');
  }

  it('the in-game dialog opens from settings and from the phone menu', async () => {
    await boot();
    act(() => useUiStore.getState().navigate('settings'));
    fireEvent.click(await within(screen.getByRole('main')).findByRole('button', { name: 'Report a problem' }));
    expect(await screen.findByRole('dialog', { name: 'Report a problem' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Report a problem' })).toBeNull());
    const phone = screen.getByRole('navigation', { name: 'Navigation' });
    fireEvent.click(within(phone).getByRole('button', { name: /More/ }));
    fireEvent.click(within(phone).getByRole('button', { name: 'Report a problem' }));
    expect(await screen.findByRole('dialog', { name: 'Report a problem' })).toBeTruthy();
  });

  it('the dialog shows world, screen, build and browser before sending and posts them', async () => {
    await boot();
    act(() => useUiStore.getState().navigate('settings'));
    fireEvent.click(await within(screen.getByRole('main')).findByRole('button', { name: 'Report a problem' }));
    const dialog = await screen.findByRole('dialog', { name: 'Report a problem' });
    const attached = within(dialog).getByRole('list', { name: 'Sent along with it' }).textContent!;
    for (const part of ['Settings', BUILD_VERSION, navigator.userAgent.slice(0, 20)]) expect(attached).toContain(part);
    fireEvent.change(within(dialog).getByLabelText('What happened?'), { target: { value: TEXT } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Send' }));
    await within(dialog).findByText(/Thank you/);
    expect(bodies).toEqual([{ kind: 'bug', text: TEXT, worldId: 1, screen: 'settings', build: BUILD_VERSION, userAgent: navigator.userAgent }]);
  });

  it('the in-game dialog takes an email for a reply', async () => {
    await boot();
    act(() => useUiStore.getState().navigate('settings'));
    fireEvent.click(await within(screen.getByRole('main')).findByRole('button', { name: 'Report a problem' }));
    const dialog = await screen.findByRole('dialog', { name: 'Report a problem' });
    fireEvent.change(within(dialog).getByLabelText('What happened?'), { target: { value: TEXT } });
    fireEvent.change(within(dialog).getByLabelText(/email/i), { target: { value: ' a@b.cd ' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Send' }));
    await within(dialog).findByText(/Thank you/);
    expect(bodies[0]).toMatchObject({ kind: 'bug', text: TEXT, email: 'a@b.cd', worldId: 1 });
  });
});

describe('admin and privacy', () => {
  it('the admin tab lists reports newest first and closes one', async () => {
    routes['GET /api/v1/me'] = () => ({ status: 200, body: admin });
    routes['GET /api/v1/admin/accounts'] = () => ({ status: 200, body: [] });
    routes['GET /api/v1/admin/reports'] = () => ({ status: 200, body: [
      { id: 9, createdAt: '2026-10-21T10:00:00Z', kind: 'player', text: 'Rude name in world one', email: '', username: '', worldName: '',
        screen: '', build: '', userAgent: '', status: 'new' },
      { id: 8, createdAt: '2026-10-20T10:00:00Z', kind: 'bug', text: 'The map stays black', email: 'a@b.cd', accountId: 2, username: 'ala',
        worldId: 4, worldName: 'Game #4', screen: 'map', build: 'abc', userAgent: 'Firefox', status: 'read' },
    ] });
    routes['PATCH /api/v1/admin/reports/9'] = () => ({ status: 204 });
    stubSite();
    history.replaceState(null, '', '/admin');
    render(<SiteApp />);
    fireEvent.click(await screen.findByRole('tab', { name: 'Reports' }));
    const rows = await screen.findAllByRole('row');
    const body = rows.slice(1);
    expect(body[0]!.textContent).toContain('Rude name in world one');
    expect(body[0]!.textContent).toContain('visitor');
    expect(body[1]!.textContent).toContain('ala');
    expect(body[1]!.textContent).toContain('Game #4');
    fireEvent.change(within(body[0]!).getByRole('combobox'), { target: { value: 'done' } });
    await waitFor(() => expect(calls.find((c) => c.key === 'PATCH /api/v1/admin/reports/9')?.body).toEqual({ status: 'done' }));
  });

  it('the privacy page says what a report keeps', async () => {
    stubSite();
    history.replaceState(null, '', '/privacy');
    render(<SiteApp />);
    const section = (await screen.findByRole('heading', { name: 'Problem reports' })).parentElement!;
    for (const part of ['IP address', 'a year', 'browser']) expect(section.textContent).toContain(part);
    expect(within(section).getByRole('link', { name: 'Report a problem' }).getAttribute('href')).toBe('/report');
  });
});
