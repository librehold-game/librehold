// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from '../src/account/App';
import '../src/account/pagesEager';
import { navigate } from '../src/account/router';
import { setLocale } from '../src/i18n/i18n';

type Route = { status: number; body?: unknown };
type Handler = (init: RequestInit | undefined) => Route;

let routes: Record<string, Handler>;
let calls: { key: string; body: unknown }[];

function json(status: number, body?: unknown): Route {
  return { status, body };
}

beforeEach(() => {
  calls = [];
  routes = { 'GET /api/v1/me': () => json(401, { error: { code: 'unauthenticated', message: 'Log in' } }) };
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`;
    calls.push({ key, body: init?.body ? JSON.parse(String(init.body)) : undefined });
    const handler = routes[key];
    if (!handler) return new Response(JSON.stringify({ error: { code: 'notFound', message: 'missing' } }), { status: 404 });
    const r = handler(init);
    return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
  document.cookie = 'lang=; Max-Age=0; Path=/';
});

const at = (path: string) => history.replaceState(null, '', path);

const alice = { id: 2, username: 'ala', isAdmin: false };
const settings = { worldSpeed: 8, unitSpeed: 1, mapSize: 200, barbarianDensity: 'normal', battleLosses: 'normal',
  humanModifiers: { resources: 1, recruitment: 1, building: 1 }, botModifiers: { resources: 1, recruitment: 1, building: 1 },
  botCount: 10, botArrival: 'allAtOnce', rampUpDays: 14, beginnerProtectionDays: 7, lastVillage: 'protected',
  difficulty: 'normal', botStance: 'normal' };

function fill(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

describe('account language', () => {
  it('renders the landing page in English by default', async () => {
    setLocale('en');
    render(<App />);
    await screen.findByRole('heading', { name: 'Village wars in your browser' });
    expect(screen.getByRole('link', { name: 'Log in' }).getAttribute('href')).toBe('/login');
    expect(screen.getAllByRole('link', { name: 'Create an account' }).length).toBeGreaterThanOrEqual(1);
    setLocale('pl');
  });

  it('shows server errors in the reader\'s language', async () => {
    setLocale('en');
    routes['POST /api/v1/auth/login'] = () => json(401, { error: { code: 'badCredentials' } });
    at('/login');
    render(<App />);
    fireEvent.change(await screen.findByLabelText('Username'), { target: { value: 'ala' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong-password' } });
    fireEvent.click(screen.getByRole('button', { name: 'Log in' }));
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Wrong username or password');
    setLocale('pl');
  });

  it('takes the language stored on the account', async () => {
    routes['GET /api/v1/me'] = () => json(200, { ...alice, locale: 'en' });
    routes['GET /api/v1/worlds'] = () => json(200, { mine: [], open: [] });
    render(<App />);
    await waitFor(() => expect(document.documentElement.lang).toBe('en'));
    setLocale('pl');
  });

  it('switching language patches the account', async () => {
    routes['GET /api/v1/me'] = () => json(200, { ...alice, locale: 'pl' });
    routes['GET /api/v1/worlds'] = () => json(200, { mine: [], open: [] });
    routes['PATCH /api/v1/me'] = () => json(200, { ...alice, locale: 'en' });
    at('/pl/lobby');
    render(<App />);
    await screen.findByRole('heading', { name: 'Moje światy' });
    fireEvent.click(screen.getByRole('link', { name: 'EN' }));
    await waitFor(() => expect(calls.some((c) => c.key === 'PATCH /api/v1/me' && (c.body as { locale: string }).locale === 'en')).toBe(true));
    expect(location.pathname).toBe('/lobby');
    expect(document.documentElement.lang).toBe('en');
    expect(screen.getByRole('link', { name: 'EN' }).getAttribute('aria-current')).toBe('page');
    setLocale('pl');
  });

  it('a guest switching language does not call the server', async () => {
    at('/pl/');
    render(<App />);
    await screen.findByRole('heading', { name: 'Wojna o wioski w przeglądarce' });
    fireEvent.click(screen.getByRole('link', { name: 'EN' }));
    expect(location.pathname).toBe('/');
    expect(calls.some((c) => c.key === 'PATCH /api/v1/me')).toBe(false);
    setLocale('pl');
  });
});

describe('account site', () => {
  it('a visitor lands on a page that invites to play and links to the login', async () => {
    at('/pl/');
    const { container } = render(<App />);
    await screen.findByRole('heading', { name: 'Wojna o wioski w przeglądarce' });
    await waitFor(() => expect(screen.getByRole('link', { name: 'Zaloguj się' }).getAttribute('href')).toBe('/pl/logowanie'));
    const joins = screen.getAllByRole('link', { name: 'Załóż konto' });
    expect(joins.length).toBeGreaterThanOrEqual(1);
    for (const a of joins) expect(a.getAttribute('href')).toBe('/pl/rejestracja');
    expect(container.querySelectorAll('.feature')).toHaveLength(4);
    for (const img of container.querySelectorAll('img')) expect(img.getAttribute('alt'), img.getAttribute('src') ?? '').not.toBeNull();
  });

  it('login and registration open with the pixel landscape banner', async () => {
    at('/pl/logowanie');
    const { container } = render(<App />);
    await screen.findByRole('heading', { name: /Logowanie/ });
    expect(container.querySelector<HTMLImageElement>('img.hero')?.src).toContain('hero.png');
    cleanup();
    at('/pl/rejestracja');
    const register = render(<App />).container;
    await screen.findByRole('heading', { name: /Rejestracja/ });
    expect(register.querySelector('img.hero')).not.toBeNull();
  });

  it('registration is open: no access code, and the server error shows by its field', async () => {
    at('/pl/rejestracja');
    routes['POST /api/v1/auth/register'] = () =>
      json(400, { error: { code: 'usernameTaken', field: 'username', message: 'taken' } });
    render(<App />);
    await screen.findByRole('heading', { name: /Rejestracja/ });
    expect(screen.queryByLabelText('Kod dostępu')).toBeNull();
    fill('Nazwa użytkownika', 'ala');
    fill('Hasło', 'long-password');
    fill('Powtórz hasło', 'long-password');
    fireEvent.click(screen.getByRole('button', { name: 'Załóż konto' }));
    await screen.findByText('Ta nazwa jest zajęta');
    expect(calls.find((c) => c.key === 'POST /api/v1/auth/register')?.body).toEqual({
      username: 'ala', password: 'long-password', passwordConfirm: 'long-password', locale: 'pl' });
  });

  it('too many new accounts from one address get their own message', async () => {
    at('/pl/rejestracja');
    routes['POST /api/v1/auth/register'] = () =>
      json(429, { error: { code: 'tooManyRegistrations', message: 'x' } });
    render(<App />);
    await screen.findByRole('heading', { name: /Rejestracja/ });
    fill('Nazwa użytkownika', 'ala');
    fill('Hasło', 'long-password');
    fill('Powtórz hasło', 'long-password');
    fireEvent.click(screen.getByRole('button', { name: 'Załóż konto' }));
    await screen.findByText('Z tego adresu założono już kilka kont. Spróbuj ponownie za godzinę.');
  });

  it('a successful login goes to the lobby', async () => {
    at('/pl/logowanie');
    routes['POST /api/v1/auth/login'] = () => json(200, { account: alice, token: 't' });
    routes['GET /api/v1/worlds'] = () => json(200, { mine: [], open: [] });
    render(<App />);
    await screen.findByRole('heading', { name: /Logowanie/ });
    fill('Nazwa użytkownika', 'ala');
    fill('Hasło', 'wrong-password');
    routes['POST /api/v1/auth/login'] = () => json(401, { error: { code: 'badCredentials', message: 'x' } });
    fireEvent.click(screen.getByRole('button', { name: 'Zaloguj' }));
    await screen.findByText('Błędna nazwa użytkownika lub hasło');
    routes['POST /api/v1/auth/login'] = () => json(200, { account: alice, token: 't' });
    fill('Hasło', 'long-password');
    fireEvent.click(screen.getByRole('button', { name: 'Zaloguj' }));
    await screen.findByRole('heading', { name: 'Moje światy' });
    expect(location.pathname).toBe('/pl/lobby');
  });

  it('joining from the lobby sends the chosen direction', async () => {
    routes['GET /api/v1/me'] = () => json(200, alice);
    routes['GET /api/v1/worlds'] = () => json(200, { mine: [], open: [{ id: 3, name: 'World 3', humans: 1, bots: 10, paused: false, settings }] });
    routes['POST /api/v1/worlds/3/join'] = () => json(200, { playerId: 'h2' });
    at('/pl/lobby');
    render(<App />);
    await screen.findByText('World 3');
    fireEvent.change(screen.getByLabelText('Kierunek'), { target: { value: 'nw' } });
    fireEvent.click(screen.getByRole('button', { name: 'Dołącz' }));
    await waitFor(() => expect(calls.find((c) => c.key === 'POST /api/v1/worlds/3/join')?.body).toEqual({ quadrant: 'nw' }));
  });

  it('the admin panel is for admins only', async () => {
    routes['GET /api/v1/me'] = () => json(200, alice);
    routes['GET /api/v1/worlds'] = () => json(200, { mine: [], open: [] });
    at('/pl/admin');
    render(<App />);
    await screen.findByRole('heading', { name: 'Moje światy' });
    expect(location.pathname).toBe('/pl/lobby');
    expect(screen.queryByRole('link', { name: 'Panel admina' })).toBeNull();
    cleanup();
    routes['GET /api/v1/me'] = () => json(200, { ...alice, isAdmin: true });
    routes['GET /api/v1/admin/accounts'] = () => json(200, []);
    render(<App />);
    expect(await screen.findByRole('link', { name: 'Panel admina' })).toBeTruthy();
    act(() => navigate('/pl/admin'));
    // The admin page fetches its first tab; under a busy full run that takes longer than the default second.
    await screen.findByRole('tab', { name: 'Konta' }, { timeout: 5000 });
    expect(screen.queryByRole('tab', { name: 'Kody' })).toBeNull();
  });
});
