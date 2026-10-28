// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { Link, navigate, useRoute } from '../src/account/router';
import { getLocale, registerCatalog, setLocale } from '../src/i18n/i18n';
import { LanguageSwitch } from '../src/i18n/LanguageSwitch';

function Probe() {
  const r = useRoute();
  return <span data-testid="probe">{r.id}</span>;
}

beforeEach(() => {
  registerCatalog('en', { language: { label: 'Language', en: 'EN', pl: 'PL' } });
  history.replaceState(null, '', '/');
  setLocale('en');
});

afterEach(() => {
  cleanup();
  document.cookie = 'lang=; Max-Age=0; Path=/';
});

describe('router', () => {
  it('Link navigates without a reload and useRoute follows', () => {
    render(<><Probe /><Link to="/pl/prywatnosc">go</Link></>);
    expect(screen.getByTestId('probe').textContent).toBe('home');
    fireEvent.click(screen.getByText('go'));
    expect(screen.getByTestId('probe').textContent).toBe('privacy');
    expect(getLocale()).toBe('pl');
    expect(location.pathname).toBe('/pl/prywatnosc');
  });

  it("modified clicks keep the browser's behaviour", () => {
    const prevented: boolean[] = [];
    // happy-dom follows an unprevented link itself; record what our handler did, then stop it.
    const record = (e: Event) => { prevented.push(e.defaultPrevented); e.preventDefault(); };
    window.addEventListener('click', record);
    const push = vi.spyOn(history, 'pushState');
    render(<><Link to="/privacy">plain</Link><Link to="/privacy" target="_blank">blank</Link></>);
    const a = screen.getByText('plain');
    fireEvent.click(a, { ctrlKey: true });
    fireEvent.click(a, { metaKey: true });
    fireEvent.click(a, { button: 1 });
    fireEvent.click(screen.getByText('blank'));
    window.removeEventListener('click', record);
    expect(prevented).toEqual([false, false, false, false]);
    expect(push).not.toHaveBeenCalled();
    expect(location.pathname).toBe('/');
    push.mockRestore();
  });

  it('back button restores the page and its language', () => {
    render(<Probe />);
    act(() => navigate('/pl/prywatnosc'));
    expect(screen.getByTestId('probe').textContent).toBe('privacy');
    act(() => {
      history.replaceState(null, '', '/');
      dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(screen.getByTestId('probe').textContent).toBe('home');
    expect(getLocale()).toBe('en');
  });

  it('replace adds no history entry', () => {
    const before = history.length;
    act(() => navigate('/privacy', true));
    expect(history.length).toBe(before);
    expect(location.pathname).toBe('/privacy');
  });

  it('unknown addresses are notFound in the address language', () => {
    render(<Probe />);
    act(() => navigate('/pl/nic'));
    expect(screen.getByTestId('probe').textContent).toBe('notFound');
    expect(getLocale()).toBe('pl');
  });

  it('language switch links to the twin and remembers the choice', () => {
    const onChange = vi.fn();
    render(<LanguageSwitch hrefFor={(l) => (l === 'pl' ? '/pl/' : '/')} onChange={onChange} />);
    const pl = screen.getByText('PL');
    expect(pl.getAttribute('href')).toBe('/pl/');
    expect(pl.getAttribute('hreflang')).toBe('pl');
    fireEvent.click(pl);
    expect(document.cookie).toContain('lang=pl');
    expect(onChange).toHaveBeenCalledWith('pl');
  });
});
