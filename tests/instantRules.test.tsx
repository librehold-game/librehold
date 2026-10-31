// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { UNITS } from '../src/data/units.data';
import { checkRecruit, nobleLimit, unitCost } from '../src/engine/units';
import { noblesNeededFor } from '../src/engine/conquest';
import { HUMAN_ID, type GameState } from '../src/engine/types';
import { setLocale } from '../src/i18n/i18n';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { Header } from '../src/ui/layout/Header';
import { AcademyScreen } from '../src/ui/views/AcademyScreen';
import { activeVillage, newTestGame } from './helpers';

const NOBLE_COST = { wood: 1000, clay: 2000, iron: 3000 };

function game(noCoins: boolean): GameState {
  const g = newTestGame();
  const v = activeVillage(g);
  v.buildings.academy = 3;
  v.resources = { wood: 50_000, clay: 50_000, iron: 50_000 };
  g.players[HUMAN_ID]!.coins = 3;
  if (noCoins) {
    g.config.noCoins = true;
    g.config.nobleCost = NOBLE_COST;
    g.config.ownedNobleDropMin = 50;
    g.config.ownedNobleDropMax = 60;
    g.config.academyExtraNobles = 1;
  }
  return g;
}

function mount(g: GameState): void {
  useGameStore.setState({ game: g, phase: 'ready', devMode: true, notice: null, lastError: null });
  useUiStore.setState({ view: 'village' });
}

beforeEach(() => setLocale('en'));
afterEach(() => {
  cleanup();
  setLocale('pl');
});

describe('instant worlds without coins', () => {
  it('a nobleman asks for no coins and costs the world price', () => {
    const g = game(true);
    g.players[HUMAN_ID]!.coins = 0;
    const check = checkRecruit(g, activeVillage(g), 'noble', 1);
    expect(check.blockers.some((b) => b.kind === 'coins')).toBe(false);
    expect(check.cost).toEqual(NOBLE_COST);
    expect(check.maxAffordable).toBeGreaterThan(0);
  });

  it('a long world still asks for coins', () => {
    const g = game(false);
    g.players[HUMAN_ID]!.coins = 0;
    const check = checkRecruit(g, activeVillage(g), 'noble', 1);
    expect(check.blockers.some((b) => b.kind === 'coins')).toBe(true);
  });

  it('a long world prices a nobleman from the unit table even when the server sends a zero nobleCost', () => {
    const g = game(false);
    g.config.nobleCost = { wood: 0, clay: 0, iron: 0 };
    expect(unitCost('noble', 1, g.config)).toEqual(UNITS.noble.cost);
  });

  it('owned villages need two nobles at a drop of 50-60', () => {
    const g = game(true);
    expect(noblesNeededFor(g.config, true, 100)).toBe(2);
    expect(noblesNeededFor(g.config, false, 15)).toBe(1);
  });

  it('an academy holds one more noble in an instant world only', () => {
    const g = game(true);
    const v = activeVillage(g);
    v.buildings.academy = 1;
    expect(nobleLimit(g.config, v)).toBe(2);
    v.buildings.academy = 0;
    expect(nobleLimit(g.config, v)).toBe(0);
    const long = game(false);
    activeVillage(long).buildings.academy = 1;
    expect(nobleLimit(long.config, activeVillage(long))).toBe(1);
  });

  it('the academy shows no coin panel and no mint button', () => {
    mount(game(true));
    render(<AcademyScreen />);
    expect(screen.queryByRole('button', { name: /mint/i })).toBeNull();
    expect(screen.queryByRole('heading', { name: /coins/i })).toBeNull();
  });

  it('the academy of a long world keeps its mint button', () => {
    mount(game(false));
    render(<AcademyScreen />);
    expect(screen.getByRole('button', { name: /mint/i })).toBeTruthy();
  });

  it('the header shows no coin badge', () => {
    mount(game(true));
    const { container } = render(<Header />);
    expect(container.querySelector('img[src*="coin"]')).toBeNull();
  });
});
