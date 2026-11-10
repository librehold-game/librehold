// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { setLocale, t } from '../src/i18n/i18n';
import { reportVillageOf, unreadReports } from '../src/engine/reports';
import type { GameState, Report, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { ReportsView } from '../src/ui/views/ReportsView';
import { activeVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer, type StubRoute } from './serverStub';

let g: GameState;
let home: Village;
let bot: Village;
let bodies: Record<string, unknown[]>;

function attackReport(over: Partial<Report>): Report {
  return {
    id: `r${g.reports.length + 1}`, time: g.now, type: 'attack', originId: home.id, targetId: bot.id,
    origin: reportVillageOf(g, home.id), target: reportVillageOf(g, bot.id),
    attacker: { units: { lightCav: 5 }, losses: {} }, defender: { units: {}, losses: {} },
    luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    ...over,
  };
}

const record = (name: string): StubRoute => (body) => {
  (bodies[name] ??= []).push(body);
  return { status: 200, body: stateFixture(g) };
};

const rows = () => screen.getAllByRole('row').slice(1);
const hideBox = () => screen.getByLabelText('Hide auto-farm reports') as HTMLInputElement;

beforeEach(async () => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  g.reports.push(attackReport({}));
  g.reports.push(attackReport({ autoFarm: true }));
  bodies = {};
  stubServer(g, { routes: {
    'POST /api/v1/worlds/1/orders/mark-all-reports-read': record('markAll'),
    'POST /api/v1/worlds/1/orders/delete-read-reports': record('deleteRead'),
  } });
  useUiStore.setState({ view: 'reports', reportsHideAutoFarm: false, reportsVillageFilter: null });
  await useGameStore.getState().boot();
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('auto-farm reports', () => {
  it('the list and the report itself carry the auto-farm tag in words, other reports do not', () => {
    render(<ReportsView />);
    expect(rows()).toHaveLength(2);
    const tagged = rows().filter((r) => r.textContent!.includes(t('reports.autoFarmTag')));
    expect(tagged).toHaveLength(1);
    fireEvent.click(tagged[0]!);
    expect(screen.getByRole('heading', { level: 3 }).textContent).toContain(t('reports.autoFarmTag'));
  });

  it('the checkbox hides the auto-farm reports and shows them again', () => {
    render(<ReportsView />);
    fireEvent.click(hideBox());
    expect(rows()).toHaveLength(1);
    expect(screen.queryByText(t('reports.autoFarmTag'))).toBeNull();
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

  it('they arrive unread but stay out of the unread count', () => {
    const game = useGameStore.getState().game!;
    expect(game.reports.every((r) => !r.read)).toBe(true);
    expect(unreadReports(game)).toBe(1);
  });

  it('when the filters hide every report, the list says so', () => {
    act(() => {
      const game = useGameStore.getState().game!;
      useGameStore.setState({ game: { ...game, reports: game.reports.filter((r) => r.autoFarm) } });
    });
    render(<ReportsView />);
    fireEvent.click(hideBox());
    screen.getByText('No reports match the filters.');
  });

  it('marking all read and deleting read leave the hidden auto-farm reports alone', async () => {
    render(<ReportsView />);
    fireEvent.click(screen.getByText('Mark all as read'));
    await waitFor(() => expect(bodies.markAll).toEqual([{ skipAutoFarm: false }]));
    fireEvent.click(hideBox());
    fireEvent.click(screen.getByText('Mark all as read'));
    fireEvent.click(screen.getByText('Delete read'));
    await waitFor(() => expect(bodies.markAll).toEqual([{ skipAutoFarm: false }, { skipAutoFarm: true }]));
    await waitFor(() => expect(bodies.deleteRead).toEqual([{ skipAutoFarm: true }]));
  });
});
