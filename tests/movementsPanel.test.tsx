// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale, t } from '../src/i18n/i18n';
import { militaryCommands } from '../src/engine/commands';
import type { GameState, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { MovementsPanel, MovementsSection } from '../src/ui/views/MovementsPanel';
import { merchantCount, merchantText } from '../src/ui/components/CommandsTable';
import { villageLabelOf } from '../src/ui/villageName';
import { activeVillage, addCommand, addOwnVillage, makeBotVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

let g: GameState;
let home: Village;
let other: Village;

beforeEach(() => {
  setLocale('en');
  useUiStore.setState((s) => ({ prefs: { ...s.prefs, movementsCollapsed: false, movementsHideAutoFarm: false } }));
  g = newTestGame();
  home = activeVillage(g);
  other = addOwnVillage(g, home.x + 3, home.y + 2, 'Second');
  g.activeVillageId = other.id;
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
});

/** The store has to hold the game before these components can read the active village. */
const show = (ui: React.ReactElement) => {
  stubServer(g);
  useGameStore.setState({ game: g, phase: 'ready' });
  return render(ui);
};

const bodyText = () => document.body.textContent ?? '';

describe('movements panel', () => {
  it('is hidden when the village has nothing under way', () => {
    const { container } = show(<MovementsSection />);
    expect(container.textContent).toBe('');
  });

  it('is shown under its heading while something is on the road', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 } });
    show(<MovementsSection />);
    expect(screen.getByText(t('village.movements'))).toBeTruthy();
  });

  it('lists an own attack with its route and troops, home to target', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle(t('rally.cmd.attack'))).toBeTruthy();
    expect(bodyText()).toContain(villageLabelOf(g, other));
    expect(bodyText()).toContain(villageLabelOf(g, home));
    // The troop column shows exactly what is marching, not a merchant haul.
    const troop = [...screen.getAllByTitle('Spearman')].map((e) => e.textContent).join('');
    expect(troop).toContain('40');
  });

  it('lists an own support as moving to the target village', () => {
    addCommand(g, { type: 'support', originId: other.id, targetId: home.id, units: { sword: 12 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle(t('rally.cmd.support'))).toBeTruthy();
    expect(bodyText()).toContain(villageLabelOf(g, home));
  });

  it('lists a return under the village it goes home to', () => {
    addCommand(g, { type: 'return', originId: home.id, targetId: other.id, units: { spear: 7 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle(t('rally.cmd.return'))).toBeTruthy();
    expect(bodyText()).toContain(villageLabelOf(g, other));
  });

  it('lists a merchant shipment this village sent, with its resources and merchant count', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 2, haul: { wood: 1500, clay: 300, iron: 0 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle(t('rally.cmd.trade'))).toBeTruthy();
    expect(screen.getByText('· 2 merchants')).toBeTruthy();
    // Each resource of the haul is spelled out with its own icon and amount.
    expect(screen.getByTitle('Wood').textContent).toBe('1,500');
    expect(screen.getByTitle('Clay').textContent).toBe('300');
    expect(screen.queryByTitle('Iron')).toBeNull();
  });

  it('spells out the resources of an incoming haul too', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: bot.id, targetId: other.id, merchants: 1, haul: { wood: 0, clay: 400, iron: 250 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle('Clay').textContent).toBe('400');
    expect(screen.getByTitle('Iron').textContent).toBe('250');
    expect(screen.queryByTitle('Wood')).toBeNull();
  });

  it('lists a haul another player sent to this village, marked incoming', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: bot.id, targetId: other.id, merchants: 1, haul: { wood: 0, clay: 400, iron: 0 } });
    show(<MovementsPanel />);
    expect(screen.getByTitle(t('rally.cmd.trade'))).toBeTruthy();
    expect(screen.getByText('· 1 merchant')).toBeTruthy();
    expect(screen.getByText(t('movements.incoming'))).toBeTruthy();
  });

  it('tints only the incoming haul and leaves my own shipment plain', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: bot.id, targetId: other.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    const { container } = show(<MovementsPanel />);
    // The countdown inside the arrival cell is tinted, the same way the Overview marks an incoming haul.
    const countdowns = container.querySelectorAll('tbody td:last-child > span:first-child');
    expect(countdowns).toHaveLength(2);
    expect([...countdowns].filter((s) => s.className.includes('text-ok'))).toHaveLength(1);
  });

  it('does not list another player moving between their own villages, or an unlisted trade', () => {
    const a = makeBotVillage(g);
    const b = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: a.id, targetId: b.id, merchants: 1, haul: { wood: 10, clay: 0, iron: 0 } });
    addCommand(g, { type: 'attack', originId: a.id, targetId: b.id, units: { spear: 5 } });
    const { container } = show(<MovementsPanel />);
    expect(container.textContent).toBe('');
  });

  it('narrows to merchant shipments when the screen asks for trades only', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 9 } });
    addCommand(g, { type: 'support', originId: other.id, targetId: home.id, units: { sword: 4 } });
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    const { container } = show(<MovementsSection scope="trade" heading="Merchant shipments" />);
    expect(container.textContent).toContain('Merchant shipments');
    expect(screen.getByTitle(t('rally.cmd.trade'))).toBeTruthy();
    expect(screen.queryByTitle(t('rally.cmd.attack'))).toBeNull();
    expect(screen.queryByTitle(t('rally.cmd.support'))).toBeNull();
    // A march alone leaves the filtered panel empty, so the Market does not sprout a heading with nothing under it.
    cleanup();
    g.commands = g.commands.filter((c) => c.type !== 'trade');
    const empty = show(<MovementsSection scope="trade" heading="Merchant shipments" />);
    expect(empty.container.textContent).toBe('');
  });

  it('orders the rows by arrival', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 1 }, arrivesAt: g.now + 3 * 3_600_000 });
    addCommand(g, { type: 'support', originId: other.id, targetId: home.id, units: { spear: 1 }, arrivesAt: g.now + 3_600_000 });
    const { container } = show(<MovementsPanel />);
    const kinds = [...container.querySelectorAll('tbody tr td:first-child img')].map((i) => i.getAttribute('title'));
    expect(kinds).toEqual(['Support', 'Attack']);
  });

  it("leaves another of my villages' marches off this village's panel", () => {
    // `other` is the active village here, so a march `home` sent belongs on home's panel, not on this one.
    addCommand(g, { type: 'attack', originId: home.id, targetId: other.id, units: { spear: 5 } });
    addCommand(g, { type: 'support', originId: home.id, targetId: other.id, units: { sword: 3 } });
    const { container } = show(<MovementsPanel />);
    expect(container.querySelectorAll('tbody tr')).toHaveLength(0);
  });

  it('shows a shipment coming to this village under this village alone', () => {
    const bot = makeBotVillage(g);
    // The cart is homed at the bot (a trade is stored with its origin), so a strict home test would lose it.
    addCommand(g, { type: 'trade', originId: bot.id, targetId: other.id, merchants: 1, haul: { wood: 0, clay: 250, iron: 0 } });
    const mine = show(<MovementsPanel />);
    expect(mine.container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(mine.container.textContent).toContain(t('movements.incoming'));
    cleanup();
    // The sending village does not also list it.
    g.activeVillageId = home.id;
    const sender = show(<MovementsPanel />);
    expect(sender.container.querySelectorAll('tbody tr')).toHaveLength(0);
    expect(sender.container.textContent).not.toContain(t('movements.incoming'));
  });

  it('shows a shipment between two of my own villages on the receiving village too', () => {
    // Both ends are mine, so this is not a haul "another player" sent — but the resources still arrive here.
    addCommand(g, { type: 'trade', originId: home.id, targetId: other.id, merchants: 2, haul: { wood: 300, clay: 0, iron: 0 } });
    const receiving = show(<MovementsPanel />);
    expect(receiving.container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(receiving.container.textContent).toContain('· 2 merchants');
    cleanup();
    // And the village that sent them.
    g.activeVillageId = home.id;
    const { container } = show(<MovementsPanel />);
    expect(container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(container.textContent).toContain('· 2 merchants');
  });

  it('lists the delivery and the empty return leg each on the village they belong to', () => {
    // The server pairs a send with an empty leg bringing the merchants home: origin/target swapped, `returning` set.
    addCommand(g, { type: 'trade', originId: home.id, targetId: other.id, merchants: 2, haul: { wood: 300, clay: 0, iron: 0 } });
    addCommand(g, { type: 'trade', originId: other.id, targetId: home.id, merchants: 2, returning: true, arrivesAt: g.now + 3_600_000 });
    // The receiver sees the delivery only; the merchants' way home is the sender's business.
    const receiving = show(<MovementsPanel />);
    expect(receiving.container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(receiving.container.textContent).toContain('· 2 merchants');
    expect(receiving.container.textContent).not.toContain(t('movements.incoming'));
    cleanup();
    // The sender sees both: the cart it sent out and its merchants coming back.
    g.activeVillageId = home.id;
    const { container } = show(<MovementsPanel />);
    expect(container.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(container.textContent).not.toContain(t('movements.incoming'));
  });

  it('keeps my own returning merchants out of the incoming list', () => {
    // A return leg is stored with swapped ends; its merchants are mine and are simply on their way home.
    addCommand(g, { type: 'trade', originId: 'v:99:99', targetId: other.id, merchants: 2, returning: true });
    const { container } = show(<MovementsPanel />);
    expect(container.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(container.textContent).not.toContain(t('movements.incoming'));
  });

  it('pages at most eight movements a page, soonest home first', () => {
    // Ten marches, each an hour later than the one before, added out of order.
    const order = [7, 2, 9, 0, 5, 1, 8, 3, 6, 4];
    for (const i of order) {
      addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: i + 1 }, arrivesAt: g.now + (i + 1) * 3_600_000 });
    }
    const { container } = show(<MovementsPanel />);
    const rows = () => [...container.querySelectorAll('tbody tr')];
    // The Units cell carries the one unit count this march has.
    const spears = () => rows().map((tr) => tr.children[2]?.textContent?.replace(/\D/g, ''));
    expect(rows()).toHaveLength(8);
    // Page 1 is the eight soonest: one spear to eight spears, in that order.
    expect(spears()).toEqual(['1', '2', '3', '4', '5', '6', '7', '8']);
    expect(container.textContent).toContain('1 / 2');
    fireEvent.click(screen.getByText(t('market.pageNext')));
    expect(rows()).toHaveLength(2);
    expect(spears()).toEqual(['9', '10']);
    expect(container.textContent).toContain('2 / 2');
    expect(screen.getByText(t('market.pageNext')).closest('button')).toHaveProperty('disabled', true);
  });

  it("covers every village it is given, and nothing outside them", () => {
    // The Overview shows a group at once; a march from a village outside the set must not appear.
    const outside = addOwnVillage(g, home.x + 9, home.y + 9, 'Outside');
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 1, haul: { wood: 100, clay: 0, iron: 0 } });
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 2, haul: { wood: 200, clay: 0, iron: 0 } });
    addCommand(g, { type: 'trade', originId: outside.id, targetId: bot.id, merchants: 4, haul: { wood: 400, clay: 0, iron: 0 } });
    const { container } = show(<MovementsPanel scope="trade" villageIds={[home.id, other.id]} />);
    const rows = [...container.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(2);
    expect(container.textContent).not.toContain('· 4 merchants');
    // Still soonest first across the whole set.
    expect(rows.map((tr) => tr.textContent?.includes('· 1 merchant') ? 1 : 2)).toEqual([1, 2]);
  });

  it('reads singular for one merchant and plural for several', () => {
    expect(merchantText(1)).toBe('1 merchant returning');
    expect(merchantText(2)).toBe('2 merchants returning');
    expect(merchantCount(1)).toBe('1 merchant');
    expect(merchantCount(4)).toBe('4 merchants');
  });
});

describe('militaryCommands', () => {
  it('keeps only the village\'s own marches, never a trade', () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 1 } });
    addCommand(g, { type: 'return', originId: home.id, targetId: other.id, units: { spear: 1 } });
    addCommand(g, { type: 'trade', originId: other.id, targetId: bot.id, merchants: 1 });
    addCommand(g, { type: 'attack', originId: bot.id, targetId: home.id, units: { spear: 9 } });
    const types = militaryCommands(g, other.id).map((c) => c.type).sort();
    expect(types).toEqual(['attack', 'return']);
  });
});

describe('movements panel and the Overview share one table', () => {
  it('draws the same four columns the Commands tab does', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 3 } });
    const { container } = show(<MovementsPanel />);
    expect(container.querySelector('table.table-game.table-stack')).toBeTruthy();
    expect([...container.querySelectorAll('thead th')].map((th) => th.textContent))
      .toEqual([t('overview.type'), t('overview.route'), t('overview.unitsLoot'), t('overview.arrival')]);
  });

  it('shows the absolute arrival time beside the countdown, as the Overview does', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 3 } });
    const { container } = show(<MovementsPanel />);
    // Countdown ("1:00:00") plus the local clock time ("tomorrow at 00:13:20"), as two elements.
    const cell = container.querySelector('tbody td:last-child')!;
    expect(cell.children).toHaveLength(2);
    expect(cell.textContent).toMatch(/\d+:\d\d:\d\d/);
    expect(cell.textContent).toMatch(/at \d/);
  });
});

describe('movements panel auto-farm marks', () => {
  const rows = (container: HTMLElement) => [...container.querySelectorAll('tbody tr')];

  it('tags an auto-farm march with a short badge, whose tooltip is the wording the reports use', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 }, autoFarm: true });
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { sword: 5 } });
    const { container } = show(<MovementsPanel />);
    expect(screen.getAllByText(t('movements.autoFarmBadge'))).toHaveLength(1);
    // The badge sits on the route, next to the villages it goes between, and names itself on hover.
    const routeCells = [...container.querySelectorAll('tbody td[data-label]')].filter((td) => td.getAttribute('data-label') === t('overview.route'));
    expect(routeCells.filter((td) => td.textContent?.includes(t('movements.autoFarmBadge')))).toHaveLength(1);
    expect(routeCells[0]!.querySelector(`[title="${t('reports.autoFarmTag')}"]`)).toBeTruthy();
  });

  it('tags the way home of an auto-farm raid too, which the server carries over', () => {
    addCommand(g, { type: 'return', originId: home.id, targetId: other.id, units: { spear: 9 }, autoFarm: true, farm: true });
    show(<MovementsPanel />);
    expect(screen.getByText(t('movements.autoFarmBadge'))).toBeTruthy();
  });

  it('leaves a raid the player sent by hand untagged, even though the farm assistant sent it', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 }, farm: true });
    const { container } = show(<MovementsPanel />);
    expect(rows(container)).toHaveLength(1);
    expect(screen.queryByText(t('movements.autoFarmBadge'))).toBeNull();
  });

  it('hides the auto-farm marches and brings them back when unticked', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 }, autoFarm: true });
    addCommand(g, { type: 'support', originId: other.id, targetId: home.id, units: { sword: 5 } });
    const { container } = show(<MovementsSection />);
    expect(rows(container)).toHaveLength(2);
    const box = screen.getByRole('checkbox', { name: t('movements.hideAutoFarm') });
    expect((box as HTMLInputElement).checked).toBe(false);
    fireEvent.click(box);
    expect(rows(container)).toHaveLength(1);
    // The only "auto-farm" left on screen is the toggle's own label.
    expect([...container.querySelectorAll('tbody td')].some((td) => td.textContent === t('movements.autoFarmBadge'))).toBe(false);
    // The choice is a browser preference, so it outlives the card unmounting with its last movement.
    expect(useUiStore.getState().prefs.movementsHideAutoFarm).toBe(true);
    fireEvent.click(box);
    expect(rows(container)).toHaveLength(2);
  });

  it('says the auto-farm is hidden rather than showing an empty table', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 }, autoFarm: true });
    useUiStore.setState((s) => ({ prefs: { ...s.prefs, movementsHideAutoFarm: true } }));
    const { container } = show(<MovementsSection />);
    expect(container.querySelector('table')).toBeNull();
    expect(container.textContent).toContain(t('movements.autoFarmHidden'));
    // The toggle stays reachable, so the player can bring the marches back.
    expect(screen.getByRole('checkbox', { name: t('movements.hideAutoFarm') })).toBeTruthy();
  });

  it("offers no filter when none of the village's marches came from the auto-farm", () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 40 } });
    show(<MovementsSection />);
    expect(screen.queryByRole('checkbox', { name: t('movements.hideAutoFarm') })).toBeNull();
  });

  it('does not let another village\'s auto-farm march bring the filter to this one', () => {
    // `other` is the active village; this march is homed at `home` and so belongs on home's panel.
    addCommand(g, { type: 'attack', originId: home.id, targetId: other.id, units: { spear: 40 }, autoFarm: true });
    addCommand(g, { type: 'support', originId: other.id, targetId: home.id, units: { sword: 5 } });
    show(<MovementsSection />);
    expect(screen.queryByRole('checkbox', { name: t('movements.hideAutoFarm') })).toBeNull();
  });
});

describe('movements card folding', () => {
  const collapse = () => screen.getByTitle(t('village.collapse'));

  it('folds the rows away and the heading stays, with a + to bring them back', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 4 } });
    const { container } = show(<MovementsSection />);
    expect(container.querySelector('table')).toBeTruthy();
    fireEvent.click(collapse());
    // The heading stays so the player can unfold it again; only the card goes.
    expect(screen.getByText(t('village.movements'))).toBeTruthy();
    expect(container.querySelector('table')).toBeNull();
    expect(screen.getByTitle(t('village.expand'))).toBeTruthy();
    fireEvent.click(screen.getByTitle(t('village.expand')));
    expect(container.querySelector('table')).toBeTruthy();
  });

  it('remembers the fold, so it is still folded the next time a movement starts', () => {
    addCommand(g, { type: 'attack', originId: other.id, targetId: home.id, units: { spear: 4 } });
    const first = show(<MovementsSection />);
    fireEvent.click(collapse());
    expect(useUiStore.getState().prefs.movementsCollapsed).toBe(true);
    // The card unmounts entirely once the last movement lands, and comes back on the next march.
    cleanup();
    expect(first.container.textContent).toBe('');
    const again = show(<MovementsSection />);
    expect(again.container.querySelector('table')).toBeNull();
    expect(screen.getByText(t('village.movements'))).toBeTruthy();
  });

  it('stays folded while the village is switched, and folds the Market shipments panel too', () => {
    addCommand(g, { type: 'trade', originId: other.id, targetId: makeBotVillage(g).id, merchants: 1, haul: { wood: 10, clay: 0, iron: 0 } });
    show(<MovementsSection scope="trade" heading="Merchant shipments" />);
    fireEvent.click(collapse());
    // The Market mounts the same section with its own heading; the toggle belongs to the card, not one screen.
    expect(useUiStore.getState().prefs.movementsCollapsed).toBe(true);
    expect(document.body.textContent).toContain('Merchant shipments');
    expect(document.querySelector('table')).toBeNull();
  });
});
