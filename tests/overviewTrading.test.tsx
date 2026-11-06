// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale, t } from '../src/i18n/i18n';
import type { GameState, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { OverviewView } from '../src/ui/views/OverviewView';
import { activeVillage, addCommand, addOwnVillage, makeBotVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

let g: GameState;
let home: Village;
let other: Village;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  other = addOwnVillage(g, home.x + 3, home.y + 2, 'Second');
  g.activeVillageId = home.id;
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
});

/** The Overview with its Trading tab open. */
function showTrading() {
  stubServer(g);
  useGameStore.setState({ game: g, phase: 'ready' });
  const view = render(<OverviewView />);
  fireEvent.click(screen.getByRole('button', { name: t('overview.tab.trading') }));
  return view;
}

/** The Overview with its Commands tab open. */
function showCommands() {
  stubServer(g);
  useGameStore.setState({ game: g, phase: 'ready' });
  const view = render(<OverviewView />);
  fireEvent.click(screen.getByRole('button', { name: t('overview.tab.commands') }));
  return view;
}

describe('the Commands tab of the overview', () => {
  it('leaves merchant shipments to the Trading tab', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 20 } });
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    const { container } = showCommands();
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(1);
    // The troop count, where a shipment would show its haul and merchants.
    expect(rows[0]!.textContent).toContain('20');
    expect(rows[0]!.textContent).not.toContain('merchant');
  });

  it('still says so when only shipments are under way', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    showCommands();
    expect(screen.getByText(t('overview.noMoves'))).toBeTruthy();
  });
});

describe('the Trading tab of the overview', () => {
  it('is one of the overview tabs', () => {
    stubServer(g);
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<OverviewView />);
    expect(screen.getByRole('button', { name: 'Trading' })).toBeTruthy();
  });

  it('lists the merchant shipments of every village in view, soonest first', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 3, haul: { wood: 900, clay: 0, iron: 0 }, arrivesAt: g.now + 7_200_000 });
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: 300, clay: 0, iron: 0 }, arrivesAt: g.now + 3_600_000 });
    const { container } = showTrading();
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(2);
    expect(rows[0]!.textContent).toContain('1 merchant');
    expect(rows[1]!.textContent).toContain('3 merchants');
  });

  it('leaves marches out: this tab is about merchants', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 20 } });
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    const { container } = showTrading();
    expect(container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(container.textContent).not.toContain(t('rally.cmd.attack'));
  });

  it('is the Commands table: the same four columns', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    const { container } = showTrading();
    const headers = [...container.querySelectorAll('thead th')].map((th) => th.textContent);
    expect(headers).toEqual([t('overview.type'), t('overview.route'), t('overview.unitsLoot'), t('overview.arrival')]);
  });

  it('shows the resources each shipment carries, not a single total', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 2, haul: { wood: 1200, clay: 800, iron: 400 } });
    const { container } = showTrading();
    const row = container.querySelector('tbody tr')!;
    // A total of 2,400 would hide what actually arrives, which is what the row is read for.
    const titles = [...row.querySelectorAll('[title]')].map((el) => el.getAttribute('title'));
    expect(titles).toEqual(expect.arrayContaining(['Wood', 'Clay', 'Iron']));
    for (const amount of ['1,200', '800', '400']) expect(row.textContent).toContain(amount);
    expect(row.textContent).not.toContain('2,400');
    expect(row.textContent).toContain('2 merchants');
  });

  it('says so when no merchant is on the road', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 20 } });
    showTrading();
    expect(screen.getByText(t('overview.noTrade'))).toBeTruthy();
  });

  it('counts a shipment another player is bringing as something to show', () => {
    // The cart is homed at the sender, so an empty check that only looks at the origin claims "nothing on the road"
    // while the panel would list the incoming haul.
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: bot.id, targetId: home.id, merchants: 1, haul: { wood: 0, clay: 400, iron: 0 } });
    const { container } = showTrading();
    expect(screen.queryByText(t('overview.noTrade'))).toBeNull();
    expect(container.querySelectorAll('tbody tr')).toHaveLength(1);
    // The route runs from the bot's village to mine, which is what tells the two apart in this table.
    expect(container.querySelector('tbody tr')!.textContent).toContain(home.name);
  });

  it('keeps every shipment on one page: unlike the sidebar it is a full page, not a card', () => {
    const bot = makeBotVillage(g);
    for (let i = 1; i <= 9; i++) {
      addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 1, haul: { wood: i * 10, clay: 0, iron: 0 }, arrivesAt: g.now + i * 3_600_000 });
    }
    const { container } = showTrading();
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(9);
    expect(rows[0]!.textContent).toContain('10');
    expect(container.textContent).not.toContain('1 / 2');
  });
});
