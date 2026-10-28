// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen } from '@testing-library/react';
import { startAccountSite } from '../src/account/boot';
import { render } from '../src/account/entry-server';
import { lazyRoute, provideRoute } from '../src/account/pages';
import PrivacyRoute from '../src/account/PrivacyPage';
import { setLocale } from '../src/i18n/i18n';

// Every React root the site mounts, so each test can unmount it: a root left running keeps React's scheduler busy
// after the test environment is gone and throws "window is not defined".
const roots = vi.hoisted(() => [] as { unmount(): void }[]);
vi.mock('react-dom/client', async (original) => {
  const m = await original<typeof import('react-dom/client')>();
  return {
    ...m,
    createRoot: (...args: Parameters<typeof m.createRoot>) => {
      const r = m.createRoot(...args);
      roots.push(r);
      return r;
    },
    hydrateRoot: (...args: Parameters<typeof m.hydrateRoot>) => {
      const r = m.hydrateRoot(...args);
      roots.push(r);
      return r;
    },
  };
});

const guest = { id: 9, username: 'Player_fox', isAdmin: false, isGuest: true, locale: 'en' };
let me: () => Response;
let root: HTMLElement;

beforeEach(() => {
  me = () => new Response(JSON.stringify({ error: { code: 'unauthenticated' } }), { status: 401 });
  vi.stubGlobal('fetch', vi.fn(async (url: string) => (url === '/api/v1/me'
    ? me()
    : new Response(JSON.stringify({ error: { code: 'notFound' } }), { status: 404 }))));
  root = document.createElement('div');
  root.id = 'root';
  document.body.appendChild(root);
});

afterEach(() => {
  act(() => roots.splice(0).forEach((r) => r.unmount()));
  root.remove();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

/** Prerenders `page` with the browser at `address` (happy-dom has a window, so the render reads its location). */
async function start(address: string, page: string | null) {
  history.replaceState(null, '', address);
  root.innerHTML = page === null ? '' : render(page).html;
  let mode = '';
  await act(async () => {
    mode = await startAccountSite(root);
  });
  return mode;
}

describe('hydration', () => {
  it('prerendered privacy page hydrates without warnings', async () => {
    const errors = vi.spyOn(console, 'error');
    expect(await start('/privacy', '/privacy')).toBe('hydrate');
    expect(await screen.findByRole('heading', { name: 'Privacy' })).toBeTruthy();
    expect(errors).not.toHaveBeenCalled();
  });

  it('a lazily loaded page hydrates without warnings', async () => {
    history.replaceState(null, '', '/privacy');
    const html = render('/privacy').html;
    let loaded = false;
    lazyRoute('privacy', () => import('../src/account/PrivacyPage').then((m) => {
      loaded = true;
      return m;
    }));
    try {
      root.innerHTML = html;
      const errors = vi.spyOn(console, 'error');
      let mode = '';
      let loadedBeforeHydration = false;
      await act(async () => {
        mode = await startAccountSite(root);
        loadedBeforeHydration = loaded;
      });
      expect(mode).toBe('hydrate');
      expect(loadedBeforeHydration).toBe(true);
      expect(screen.getByRole('heading', { name: 'Privacy' })).toBeTruthy();
      expect(errors).not.toHaveBeenCalled();
    } finally {
      provideRoute('privacy', PrivacyRoute);
    }
  });

  it('front page hydrates and a guest gets Continue your game', async () => {
    me = () => new Response(JSON.stringify(guest), { status: 200 });
    const errors = vi.spyOn(console, 'error');
    expect(await start('/', '/')).toBe('hydrate');
    expect(await screen.findAllByRole('button', { name: 'Continue your game' }).then((b) => b[0]!)).toBeTruthy();
    expect(errors).not.toHaveBeenCalled();
  });

  it('legacy hash on a prerendered page renders the new address', async () => {
    expect(await start('/#/login', '/')).toBe('render');
    expect(location.pathname).toBe('/login');
    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeTruthy();
  });

  it('the empty shell renders the app', async () => {
    expect(await start('/lobby', null)).toBe('render');
  });
});
