// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale, t } from '../src/i18n/i18n';
import { RankingTable, type RankingData, type RankingHost } from '../src/shared/ranking/RankingTable';
import type { PlayerRow } from '../src/shared/ranking/types';

const row = (i: number, extra: Partial<PlayerRow> = {}): PlayerRow => ({
  id: `p${i}`, name: `Player ${i}`, color: '#abc', points: 1000 - i, villages: i % 7, att: i, def: 500 - i, sup: 0, quit: false, isBot: false, ...extra,
});
const empty: RankingData = { players: [], daily: [], conquests: [] };
const host = (over: Partial<RankingHost> = {}): RankingHost => ({
  renderPlayer: (p) => <a href={`/p/${p.id}`}>{p.name}</a>,
  isMe: () => false,
  formatTime: (ms) => `at ${ms}`,
  ...over,
});
const names = (c: HTMLElement) => [...c.querySelectorAll('tbody tr td:nth-child(2)')].map((td) => td.textContent);

beforeEach(() => { loadAccountCatalogs(); setLocale('en'); });
afterEach(cleanup);

describe('RankingTable', () => {
  it('shows the points tab by default, sorted, 50 a page', () => {
    const players = Array.from({ length: 120 }, (_, i) => row(i + 1)).reverse();
    const { container } = render(<RankingTable data={{ ...empty, players }} host={host()} />);
    expect(container.querySelectorAll('tbody tr')).toHaveLength(50);
    expect(container.querySelector('tbody tr td')!.textContent).toBe('1');
    expect(names(container)[0]).toBe('Player 1');
    expect(screen.getByText(t('ranking.page', { n: 1, pages: 3 }))).toBeTruthy();
    fireEvent.click(screen.getByText('→'));
    expect(container.querySelector('tbody tr td')!.textContent).toBe('51');
    expect(names(container)[0]).toBe('Player 51');
    expect(screen.getByText(t('ranking.page', { n: 2, pages: 3 }))).toBeTruthy();
  });

  it('switches category', () => {
    const players = [row(1, { villages: 1, def: 10 }), row(2, { villages: 9, def: 90 }), row(3, { villages: 5, def: 50 })];
    const { container } = render(<RankingTable data={{ ...empty, players }} host={host()} />);
    fireEvent.click(screen.getByText(t('ranking.tab.villages')));
    expect(names(container)).toEqual(['Player 2', 'Player 3', 'Player 1']);
    fireEvent.click(screen.getByText(t('ranking.tab.od')));
    fireEvent.click(screen.getByText(t('ranking.odKind.def')));
    expect(names(container)).toEqual(['Player 2', 'Player 3', 'Player 1']);
    fireEvent.click(screen.getByText(t('ranking.tab.daily')));
    expect(screen.getByText(t('ranking.nobodyToday'))).toBeTruthy();
    fireEvent.click(screen.getByText(t('ranking.tab.conquests')));
    expect(screen.getByText(t('ranking.noConquests'))).toBeTruthy();
  });

  it('lets the host render names and mark itself', () => {
    const players = [row(1), row(2, { isBot: true }), row(3, { quit: true })];
    const { container } = render(<RankingTable data={{ ...empty, players }} host={host({ isMe: (id) => id === 'p1' })} />);
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows[0]!.className).toContain('ranking-me');
    expect(rows[0]!.querySelector('a')!.getAttribute('href')).toBe('/p/p1');
    expect(rows[1]!.textContent).toContain(t('ranking.bot'));
    expect(rows[2]!.className).toContain('ranking-quit');
    expect(rows[2]!.textContent).toContain(t('ranking.inactive').trim());
  });

  it('formats conquest times through the host and marks the host\'s own', () => {
    const conquests = [{ t: 42, x: 3, y: 4, toId: 'p1', to: 'Player 1', fromId: null, from: null, toIsBot: false, fromIsBot: false }];
    const { container } = render(<RankingTable data={{ ...empty, conquests }} host={host({ isMe: (id) => id === 'p1' })} />);
    fireEvent.click(screen.getByText(t('ranking.tab.conquests')));
    expect(container.textContent).toContain('at 42');
    expect(container.textContent).toContain(t('ranking.barbarian'));
    expect(container.querySelector('tbody tr')!.className).toContain('ranking-me');
  });

  it('shows the refresh note only when the host gives one', () => {
    const { rerender } = render(<RankingTable data={empty} host={host()} />);
    expect(screen.queryByText('Refreshes hourly')).toBeNull();
    rerender(<RankingTable data={empty} host={host({ refreshNote: 'Refreshes hourly' })} />);
    expect(screen.getByText('Refreshes hourly')).toBeTruthy();
  });
});
