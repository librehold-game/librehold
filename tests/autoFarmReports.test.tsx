// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { reportVillageOf } from '../src/engine/reports';
import type { GameState, Report, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { ReportsView } from '../src/ui/views/ReportsView';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

let g: GameState;
let home: Village;
let bot: Village;

function attackReport(over: Partial<Report>): Report {
  return {
    id: `r${g.reports.length + 1}`, time: g.now, type: 'attack', originId: home.id, targetId: bot.id,
    origin: reportVillageOf(g, home.id), target: reportVillageOf(g, bot.id),
    attacker: { units: { lightCav: 5 }, losses: {} }, defender: { units: {}, losses: {} },
    luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    ...over,
  };
}

const rows = () => screen.getAllByRole('row').slice(1);
const hideBox = () => screen.getByLabelText('Hide auto-farm reports') as HTMLInputElement;

beforeEach(async () => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  g.reports.push(attackReport({}));
  g.reports.push(attackReport({ autoFarm: true, read: true }));
  stubServer(g);
  useUiStore.setState({ view: 'reports', reportsHideAutoFarm: false, reportsVillageFilter: null });
  await useGameStore.getState().boot();
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('auto-farm reports', () => {
  it('the list and the report itself carry the auto-farm tag, other reports do not', () => {
    render(<ReportsView />);
    expect(rows()).toHaveLength(2);
    const tagged = rows().filter((r) => r.textContent!.includes('auto-farm'));
    expect(tagged).toHaveLength(1);
    fireEvent.click(tagged[0]!);
    expect(screen.getByRole('heading', { level: 3 }).textContent).toContain('auto-farm');
  });

  it('the checkbox hides the auto-farm reports and shows them again', () => {
    render(<ReportsView />);
    fireEvent.click(hideBox());
    expect(rows()).toHaveLength(1);
    expect(screen.queryByText('auto-farm')).toBeNull();
    fireEvent.click(hideBox());
    expect(rows()).toHaveLength(2);
  });

  it('the choice holds when the list opens again, while the village filter is cleared', () => {
    useUiStore.setState({ reportsVillageFilter: home.id });
    const { unmount } = render(<ReportsView />);
    fireEvent.click(hideBox());
    unmount();
    expect(useUiStore.getState().reportsVillageFilter).toBeNull();
    render(<ReportsView />);
    expect(hideBox().checked).toBe(true);
    expect(rows()).toHaveLength(1);
  });
});
