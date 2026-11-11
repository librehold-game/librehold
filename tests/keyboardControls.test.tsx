// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { reportVillageOf } from '../src/engine/reports';
import type { GameState, Report } from '../src/engine/types';
import { setLocale, t } from '../src/i18n/i18n';
import { buildingName } from '../src/i18n/names';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { ErrorToast } from '../src/ui/components/ErrorToast';
import { BuildPlanPanel } from '../src/ui/views/BuildPlanPanel';
import { ReportsView } from '../src/ui/views/ReportsView';
import { VillageView } from '../src/ui/views/VillageView';
import { activeVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer } from './serverStub';

/** Controls that used to answer only a mouse click are real buttons: Tab, Enter and Space come with the element. */
let g: GameState;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  useGameStore.setState({ game: g, phase: 'ready', notice: null, lastError: null });
  useUiStore.setState({ focusBuilding: null });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

function attackReport(): Report {
  const home = activeVillage(g);
  const bot = makeBotVillage(g);
  return {
    id: 'r1', time: g.now, type: 'attack', originId: home.id, targetId: bot.id,
    origin: reportVillageOf(g, home.id), target: reportVillageOf(g, bot.id),
    attacker: { units: { axe: 10 }, losses: { axe: 3 } }, defender: { units: { spear: 5 }, losses: { spear: 5 } },
    luck: 0.05, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
  };
}

describe('reports list', () => {
  it('the report button opens the report and marks it read once', async () => {
    g.reports.push(attackReport());
    const calls = stubServer(g, { routes: {
      'POST /api/v1/worlds/1/orders/mark-reports-read': () => ({ status: 200, body: stateFixture(g) }),
    } });
    await useGameStore.getState().boot();
    render(<ReportsView />);
    const marks = () => calls.filter((c) => c.endsWith('/orders/mark-reports-read'));

    await act(async () => { fireEvent.click(await screen.findByRole('button', { name: /→/ })); });

    expect(screen.getByText('Attacker')).toBeTruthy();
    await waitFor(() => expect(marks()).toHaveLength(1));
    await act(async () => { await Promise.resolve(); });
    expect(marks()).toHaveLength(1);
  });
});

describe('build plan editor', () => {
  it('a step is a button named after what it removes', () => {
    g.buildTemplates = [];
    render(<BuildPlanPanel />);
    fireEvent.click(screen.getByRole('button', { name: t('plan.edit') }));
    fireEvent.click(screen.getByRole('button', { name: `+${buildingName('main')}` }));
    fireEvent.click(screen.getByRole('button', { name: `+${buildingName('barracks')}` }));

    fireEvent.click(screen.getByRole('button', { name: /Remove step 1/ }));

    expect(screen.getByRole('button', { name: `Remove step 1: ${buildingName('barracks')}` })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Remove step 2/ })).toBeNull();
  });
});

describe('production panel', () => {
  it('a resource is a button that focuses its building and says what it is', () => {
    render(<VillageView />);
    const wood = screen.getByRole('button', { name: 'Wood' });
    expect(wood.getAttribute('title')).toContain(buildingName('wood'));

    fireEvent.click(wood);

    expect(useUiStore.getState().focusBuilding).toBe('wood');
  });
});

describe('messages', () => {
  it('an error is announced and its close button clears it', () => {
    useGameStore.setState({ lastError: 'Not enough wood' });
    render(<ErrorToast />);
    expect(screen.getByRole('alert').textContent).toBe('Not enough wood');

    fireEvent.click(screen.getByRole('button', { name: t('info.close') }));

    expect(useGameStore.getState().lastError).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('a notice is announced and its close button clears it', () => {
    useGameStore.setState({ notice: 'The horde declared you an enemy.' });
    render(<ErrorToast />);
    expect(screen.getByRole('status').textContent).toBe('The horde declared you an enemy.');

    fireEvent.click(screen.getByRole('button', { name: t('info.close') }));

    expect(useGameStore.getState().notice).toBeNull();
    expect(screen.queryByRole('status')).toBeNull();
  });
});
