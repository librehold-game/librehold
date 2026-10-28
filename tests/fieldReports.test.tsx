// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { reportVillageOf } from '../src/engine/reports';
import type { GameState, Report, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { ReportsView } from '../src/ui/views/ReportsView';
import { activeVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer } from './serverStub';

const HOUR = 3_600_000;
let g: GameState;
let home: Village;
let bot: Village;
let cancels: unknown[];

function fieldReport(over: Partial<Report>): Report {
  return {
    id: `r${g.reports.length + 1}`, time: g.now, type: 'field', originId: home.id, targetId: bot.id,
    origin: reportVillageOf(g, home.id), target: reportVillageOf(g, bot.id),
    attacker: { units: { axe: 10 }, losses: { axe: 3 } }, defender: { units: { lightCav: 50 }, losses: { lightCav: 50 } },
    luck: 0.05, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    field: { x: 12.4, y: 7.6, targetType: 'attack', targetCommandId: 'own' },
    ...over,
  };
}

async function open(r: Report) {
  g.reports.push(r);
  stubServer(g, { routes: {
    'POST /api/v1/worlds/1/orders/cancel-command': (b) => { cancels.push(b); return { status: 200, body: stateFixture(g) }; },
  } });
  await useGameStore.getState().boot();
  render(<ReportsView />);
  fireEvent.click(await screen.findByText(/→/));
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  cancels = [];
  useUiStore.setState({ view: 'reports' });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('field battle reports', () => {
  it('a field battle shows both sides, the meeting place and Turn back for a hit army still marching', async () => {
    g.commands.push({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 7 }, fieldHit: true, departedAt: g.now - HOUR, arrivesAt: g.now + HOUR });
    await open(fieldReport({}));
    expect(screen.getByText(/Field battle at 12.4\|7.6/)).toBeTruthy();
    expect(screen.getByText('Attacker')).toBeTruthy();
    expect(screen.getByText('Defender')).toBeTruthy();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Turn back' })); });
    expect(cancels).toEqual([{ commandId: 'own' }]);
  });

  it('no Turn back once the army is no longer marching', async () => {
    await open(fieldReport({}));
    expect(screen.queryByRole('button', { name: 'Turn back' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Repeat the attack (same troops)' })).toBeNull();
  });

  it('a field scouting report lists the scouted units', async () => {
    await open(fieldReport({ type: 'fieldScout', attacker: { units: { scout: 3 }, losses: {} }, defender: { units: { axe: 120 }, losses: {} },
      scout: { units: { axe: 120 } } as Report['scout'] }));
    expect(screen.getByText(/Scouted army/)).toBeTruthy();
    expect(document.body.textContent).toContain('120');
  });

  it('a missed interception says the army was gone', async () => {
    await open(fieldReport({ outcome: 'defenderWon', target: { id: '', name: '', x: 0, y: 0 }, targetId: '', defender: { units: {}, losses: {} },
      field: { x: 12.4, y: 7.6, targetType: 'attack', targetCommandId: 'gone', missed: true } }));
    expect(screen.getByText(/army was gone/)).toBeTruthy();
    expect(document.body.textContent).not.toContain('(0|0)');
  });
});
