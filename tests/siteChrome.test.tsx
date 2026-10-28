// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import { SiteFooter } from '../src/account/SiteFooter';
import { SiteHeader } from '../src/account/SiteHeader';
import { pathFor } from '../src/account/routes';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

const alice = { id: 2, username: 'ala', isAdmin: false, isGuest: false, locale: 'en' as const, tutorialDone: true };
const noop = () => undefined;

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
});

afterEach(() => {
  cleanup();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const href = (name: string) => screen.getByRole('link', { name }).getAttribute('href');

describe('site header', () => {
  it('signed out the header offers how to play, worlds, faq and log in', () => {
    render(<SiteHeader me={null} onLogout={noop} />);
    expect(href('How to play')).toBe(pathFor('howToPlay', 'en'));
    expect(href('Worlds')).toBe(pathFor('worlds', 'en'));
    expect(href('FAQ')).toBe(pathFor('faq', 'en'));
    expect(href('Log in')).toBe(pathFor('login', 'en'));
    expect(href('LibreHold')).toBe('/');
    expect(screen.queryByRole('button', { name: 'Log out' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Lobby' })).toBeNull();
  });

  it('signed in it shows lobby, the name and log out, and admin only for admins', () => {
    const { rerender } = render(<SiteHeader me={alice} onLogout={noop} />);
    expect(href('Lobby')).toBe('/lobby');
    expect(href('ala')).toBe('/players/ala');
    expect(screen.getByRole('button', { name: 'Log out' })).toBeTruthy();
    expect(screen.queryByRole('link', { name: 'Log in' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Admin panel' })).toBeNull();
    rerender(<SiteHeader me={{ ...alice, isAdmin: true }} onLogout={noop} />);
    expect(href('Admin panel')).toBe('/admin');
  });

  it('in Polish every link points at the Polish address', () => {
    history.replaceState(null, '', '/pl/');
    setLocale('pl');
    render(<SiteHeader me={null} onLogout={noop} />);
    const nav = screen.getByRole('navigation', { name: 'Strona' });
    const hrefs = within(nav).getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/pl/jak-grac', '/pl/swiaty', '/pl/faq', '/pl/logowanie']);
    expect(href('LibreHold')).toBe('/pl/');
  });
});

describe('site footer', () => {
  it('the footer links every content page and the report form', () => {
    render(<SiteFooter />);
    for (const [name, id] of [['How to play', 'howToPlay'], ['Worlds', 'worlds'], ['FAQ', 'faq'], ['Changelog', 'changelog'],
      ['Privacy', 'privacy'], ['Terms', 'terms'], ['Report a problem', 'report']] as const) {
      expect(href(name), name).toBe(pathFor(id, 'en'));
    }
  });

  it('the footer names the author and the operator and links the source code', () => {
    for (const [locale, notice, operator, source] of [
      ['en', '© 2026 Szymon Kocur and contributors', 'Hosted by TomcatFlow', 'Source code (AGPL-3.0)'],
      ['pl', '© 2026 Szymon Kocur i współautorzy', 'Hostowane przez TomcatFlow', 'Kod źródłowy (AGPL-3.0)'],
    ] as const) {
      setLocale(locale);
      render(<SiteFooter />);
      const line = screen.getByRole('link', { name: source }).closest('p')!;
      expect(line.textContent).toBe(`${notice} · ${operator} · ${source}`);
      const href = (name: string) => within(line).getByRole('link', { name }).getAttribute('href');
      expect(href('Szymon Kocur')).toBe('https://szymonkocur.com');
      expect(href('TomcatFlow')).toBe('https://tomcatflow.com');
      expect(href(source)).toBe('https://github.com/librehold-game/librehold');
      for (const a of line.querySelectorAll('a')) expect(a.getAttribute('rel')).toBeNull();
      cleanup();
    }
  });
});
