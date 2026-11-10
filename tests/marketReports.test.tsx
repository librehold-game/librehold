// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale, t } from '../src/i18n/i18n';
import { reportVillageOf } from '../src/engine/reports';
import type { GameState, MarketReport, Report, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { ReportsView } from '../src/ui/views/ReportsView';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

let g: GameState;
let home: Village;
let other: Village;

const market = (over: Partial<MarketReport> = {}): MarketReport => ({
  counterpartyId: 'bot_3',
  counterpartyName: 'Brotherhood',
  counterpartyIsBot: true,
  fromVillage: reportVillageOf(g, other.id),
  travelMs: 26 * 60_000,
  give: { wood: 0, clay: 0, iron: 800 },
  want: { wood: 1200, clay: 400, iron: 0 },
  offers: 1,
  ...over,
});

function marketReport(over: Partial<Report> = {}, m: Partial<MarketReport> = {}): Report {
  const rep: Report = {
    id: `r${g.reports.length + 1}`, time: g.now, type: 'market',
    originId: other.id, targetId: home.id,
    origin: reportVillageOf(g, other.id), target: reportVillageOf(g, home.id),
    attacker: { units: {}, losses: {} }, defender: { units: {}, losses: {} },
    luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    market: market(m),
    ...over,
  };
  return rep;
}

function show(reports: Report[]) {
  stubServer(g);
  g.reports = [...reports];
  useGameStore.setState({ game: g, phase: 'ready' });
  useUiStore.setState({ view: 'reports', reportsVillageFilter: null });
  return render(<ReportsView />);
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  other = makeBotVillage(g);
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('settled trade reports', () => {
  it('offers a market filter beside the others', () => {
    show([marketReport()]);
    const options = [...document.querySelectorAll('option')].map((o) => o.textContent);
    expect(options).toContain(t('reports.filter.market'));
  });

  it('the market filter lists only settled deals', () => {
    const attack: Report = {
      ...marketReport({ id: 'a1' }),
      type: 'attack',
      market: undefined,
      outcome: 'attackerWon',
      attacker: { units: { spear: 5 }, losses: {} },
    };
    const { container } = show([marketReport({ id: 'm1' }), attack]);
    fireEvent.change(container.querySelector('select')!, { target: { value: 'market' } });
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(1);
  });

  it('names the player who accepted, the village the goods ship from, and the travel time', () => {
    show([marketReport()]);
    fireEvent.click(screen.getByText(/→/));
    expect(screen.getByText('Brotherhood')).toBeTruthy();
    // The from-village label and the travel time are both shown.
    expect(document.body.textContent).toContain(t('reports.market.comingFrom'));
    expect(document.body.textContent).toContain('26:00');
  });

  it('shows what the player gives and what they receive', () => {
    show([marketReport()]);
    fireEvent.click(screen.getByText(/→/));
    expect(screen.getByText(t('reports.market.youGive'))).toBeTruthy();
    expect(screen.getByText(t('reports.market.youGet'))).toBeTruthy();
    // 800 iron given, 1,200 wood and 400 clay received.
    expect(document.body.textContent).toContain('800');
    expect(document.body.textContent).toContain('1,200');
    expect(document.body.textContent).toContain('400');
  });

  it('one report covers a deal that settled several offers, and says how many', () => {
    show([marketReport({}, { offers: 10, want: { wood: 12000, clay: 4000, iron: 0 } })]);
    fireEvent.click(screen.getByText(/→/));
    expect(document.body.textContent).toContain(t('reports.market.offers', { n: 10 }));
    // The totals are the whole deal, not one post.
    expect(document.body.textContent).toContain('12,000');
  });

  it('marks a bot counterparty, and leaves a human one unmarked', () => {
    show([marketReport({ id: 'bot' }, { counterpartyIsBot: true, counterpartyName: 'Brotherhood' })]);
    fireEvent.click(screen.getByText(/→/));
    // Two marks: the bot who took the deal, and the bot whose village ships the goods (the fixture's village is a bot's).
    expect(document.querySelectorAll('.bot-tag').length).toBeGreaterThan(0);
    cleanup();

    // A human counterparty whose goods come from a player village: nothing should be marked.
    show([marketReport({ id: 'human' }, { counterpartyIsBot: false, counterpartyName: 'Alice', fromVillage: reportVillageOf(g, home.id) })]);
    fireEvent.click(screen.getByText(/→/));
    expect(screen.getByText('Alice')).toBeTruthy();
    expect(document.querySelectorAll('.bot-tag')).toHaveLength(0);
  });
});
