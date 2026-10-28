// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { catalogEntries, detectLocale, getLocale, registerCatalog, rememberLocale, setLocale, t, translate } from '../src/i18n/i18n';
import { formatNumber } from '../src/i18n/format';

beforeEach(() => {
  registerCatalog('en', { test: { hello: 'Hello, {name}', onlyEn: 'English only', villages: { one: '# village', other: '# villages' } } });
  registerCatalog('pl', { test: { hello: 'Cześć, {name}', villages: { one: '# wioska', few: '# wioski', many: '# wiosek', other: '# wioski' } } });
});

afterEach(() => {
  document.cookie = 'lang=; Max-Age=0; Path=/';
  setLocale('en');
  vi.restoreAllMocks();
});

describe('translation', () => {
  it('falls back from pl to en and then to the key', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    setLocale('pl');
    expect(t('test.onlyEn')).toBe('English only');
    expect(t('test.nowhere')).toBe('test.nowhere');
  });

  it('interpolates named params', () => {
    setLocale('en');
    expect(t('test.hello', { name: 'Ala' })).toBe('Hello, Ala');
  });

  it('picks Polish plural forms', () => {
    setLocale('pl');
    expect(t('test.villages', { count: 1 })).toBe('1 wioska');
    expect(t('test.villages', { count: 2 })).toBe('2 wioski');
    expect(t('test.villages', { count: 5 })).toBe('5 wiosek');
    expect(t('test.villages', { count: 22 })).toBe('22 wioski');
    expect(t('test.villages', { count: 1000 })).toBe('1000 wiosek');
    expect(t('test.villages', { count: 12345 }).replace(/\s/g, ' ')).toBe('12 345 wiosek');
  });

  it('picks English plural forms', () => {
    setLocale('en');
    expect(t('test.villages', { count: 1 })).toBe('1 village');
    expect(t('test.villages', { count: 3 })).toBe('3 villages');
  });

  it('detects pl from the cookie before the browser', () => {
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('en-US');
    document.cookie = 'lang=pl; Path=/';
    expect(detectLocale()).toBe('pl');
    document.cookie = 'lang=; Max-Age=0; Path=/';
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('pl-PL');
    expect(detectLocale()).toBe('pl');
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('de-DE');
    expect(detectLocale()).toBe('en');
  });

  it('setLocale updates html lang but not the cookie', () => {
    setLocale('pl');
    expect(getLocale()).toBe('pl');
    expect(document.documentElement.lang).toBe('pl');
    expect(document.cookie).not.toContain('lang=pl');
  });

  it('rememberLocale stores the cookie', () => {
    rememberLocale('pl');
    expect(getLocale()).toBe('pl');
    expect(document.cookie).toContain('lang=pl');
  });

  it('translate reads another language without switching', () => {
    setLocale('en');
    expect(translate('pl', 'test.hello', { name: 'Ala' })).toBe('Cześć, Ala');
    expect(translate('pl', 'test.villages', { count: 5 })).toBe('5 wiosek');
    expect(translate('pl', 'test.onlyEn')).toBe('English only');
    expect(getLocale()).toBe('en');
  });

  it('catalogEntries returns flat messages under a prefix, plurals as objects', () => {
    const pl = catalogEntries('pl', 'test.');
    expect(pl['test.hello']).toBe('Cześć, {name}');
    expect(pl['test.onlyEn']).toBe('English only');
    expect(pl['test.villages']).toEqual({ one: '# wioska', few: '# wioski', many: '# wiosek', other: '# wioski' });
    expect(Object.keys(catalogEntries('en', 'nothing.'))).toEqual([]);
  });

  it('formats numbers per locale', () => {
    setLocale('en');
    expect(formatNumber(12345)).toBe('12,345');
    setLocale('pl');
    expect(formatNumber(12345).replace(/\s/g, ' ')).toBe('12 345');
  });
});
