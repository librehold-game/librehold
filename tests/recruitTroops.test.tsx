// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import { unitsAway, unitsAwayParts } from '../src/engine/commands';
import type { GameState, Village } from '../src/engine/types';
import { setLocale, t } from '../src/i18n/i18n';
import { unitName } from '../src/i18n/names';
import { useGameStore } from '../src/state/gameStore';
import { RecruitScreen } from '../src/ui/views/RecruitScreen';
import { activeVillage, addCommand, addOwnVillage, makeBotVillage, newTestGame } from './helpers';

let g: GameState;
let home: Village;
let second: Village;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  home.buildings.barracks = 5;
  home.units.spear = 12;
  second = addOwnVillage(g, home.x + 2, home.y + 2, 'Second');
});
afterEach(() => {
  cleanup();
  setLocale('pl');
});

/** Troops on the move from home: an attack on a bot, support on its way to the second village, and an army coming back. */
function sendTroopsOut() {
  const bot = makeBotVillage(g);
  addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 15 } });
  addCommand(g, { type: 'support', originId: home.id, targetId: second.id, units: { spear: 2 } });
  addCommand(g, { type: 'return', originId: bot.id, targetId: home.id, units: { spear: 3 } });
  second.supportFrom[home.id] = { spear: 5 };
}

const spearRow = () => screen.getByText(unitName('spear')).closest('tr')!;
const cell = () => within(spearRow()).getByText(/\/ /).closest('td')!;

describe('units away from a village', () => {
  it('are split into those on the move and those supporting another own village', () => {
    sendTroopsOut();
    const { moving, supporting } = unitsAwayParts(g, home.id);
    expect(moving.spear).toBe(20);
    expect(supporting.spear).toBe(5);
    expect(unitsAway(g, home.id).spear).toBe(25);
  });
});

describe('the recruitment table', () => {
  it('shows the units in the village out of all of them, and where the rest are', () => {
    sendTroopsOut();
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RecruitScreen building="barracks" />);
    expect(cell().textContent).toBe('12 / 3720 on the move · 5 supporting');
    expect(screen.getByRole('columnheader', { name: 'In the village / total' }).getAttribute('title'))
      .toBe(t('recruit.colInVillageHint'));
  });

  it('shows only the totals when every unit is at home', () => {
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RecruitScreen building="barracks" />);
    expect(cell().textContent).toBe('12 / 12');
  });

  it('names just the part that is away', () => {
    second.supportFrom[home.id] = { spear: 4 };
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RecruitScreen building="barracks" />);
    expect(cell().textContent).toBe('12 / 164 supporting');
  });

  it('has the Polish wording', () => {
    setLocale('pl');
    sendTroopsOut();
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RecruitScreen building="barracks" />);
    expect(screen.getByRole('columnheader', { name: 'W wiosce / łącznie' })).toBeTruthy();
    expect(cell().textContent).toBe('12 / 3720 w drodze · 5 jako wsparcie');
  });
});
