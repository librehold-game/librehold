// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale, t } from '../src/i18n/i18n';
import { useGameStore } from '../src/state/gameStore';
import { MarketScreen } from '../src/ui/views/MarketScreen';
import { HUMAN_ID, emptyUnits, type GameState, type TradeOffer, type Village } from '../src/engine/types';
import { activeVillage, addCommand, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer } from './serverStub';
import { villageLabelOf } from '../src/ui/villageName';
import { canCancelCommand } from '../src/engine/commands';
import { formatCoords } from '../src/ui/format';
import { TRADE_BALANCE } from '../src/data/world.config';

const HOUR = 3_600_000;

let g: GameState;
let home: Village;
let bot: Village;

/** One offer: `from` sends 100 wood and wants 60 iron back for it. */
function offer(patch: Partial<TradeOffer> & { id: string }): TradeOffer {
  return {
    fromId: 'bot_test', originId: bot.id, give: { wood: 100, clay: 0, iron: 0 }, want: { wood: 0, clay: 0, iron: 60 },
    maxDeliveryMs: 0, expiresAt: g.now + 24 * HOUR, createdAt: g.now, ...patch,
  };
}

/** A public post from the bot offering 100 wood for 60 iron, of which there are `copies` identical ones. */
function postOffers(copies: number): TradeOffer[] {
  return Array.from({ length: copies }, (_, i) => ({
    id: `t${i}`, fromId: 'bot_test', originId: bot.id, give: { wood: 100, clay: 0, iron: 0 }, want: { wood: 0, clay: 0, iron: 60 },
    maxDeliveryMs: 0, expiresAt: g.now + 24 * HOUR, createdAt: g.now,
  }));
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  // A market so the board's accept path is open, and stock to pay with.
  home.buildings.market = 3;
  home.resources = { wood: 5000, clay: 5000, iron: 5000 };
  home.units = emptyUnits();
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
});

/** A village of someone else, for command literals that need a target. */
function botVillageId(g: GameState): string {
  return Object.values(g.villages).find((v) => v.id !== home.id)!.id;
}

/** The global market panel: the one whose table has the "Available" column. */
function globalMarketPanel(): HTMLElement {
  const tables = [...document.querySelectorAll('table.table-game')] as HTMLTableElement[];
  const table = tables.find((t) => [...t.querySelectorAll('thead th')].some((th) => th.textContent === 'Available'))!;
  return table.closest('.panel') as HTMLElement;
}

async function show(copies: number) {
  g.tradeOffers = postOffers(copies);
  stubServer(g);
  await useGameStore.getState().boot();
  render(<MarketScreen />);
  const board = globalMarketPanel();
  // A row captured once goes stale on the next render, so hand callers a fresh lookup.
  const row = () => board.querySelector('tbody tr') as HTMLElement;
  return { board, row };
}

describe('global board take-N', () => {
  it('scales the Offer and For columns together with the copy count', async () => {
    const { row } = await show(4);
    const amounts = () => ({
      offer: [...row().querySelectorAll('td:nth-child(1) strong[title]')].map((s) => `${s.getAttribute('title')}=${s.textContent}`),
      want: [...row().querySelectorAll('td:nth-child(2) strong[title]')].map((s) => `${s.getAttribute('title')}=${s.textContent}`),
    });
    expect(amounts()).toEqual({ offer: ['Wood=100'], want: ['Iron=60'] });

    const input = row().querySelector('input[type="number"]') as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: '3' } });
    expect(amounts()).toEqual({ offer: ['Wood=300'], want: ['Iron=180'] });
  });

  it('clamps the input to the number of copies on offer', async () => {
    const { row } = await show(2);
    const input = row().querySelector('input[type="number"]') as HTMLInputElement;
    expect(input.max).toBe('2');
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: '9' } });
    // Four copies were never on offer, so the row keeps showing what two of them come to.
    expect(row().querySelector('td:nth-child(1) strong[title]')!.textContent).toBe('200');
  });

  it('tints the For column by what my village can actually pay', async () => {
    home.resources = { wood: 5000, clay: 5000, iron: 100 };
    const { row } = await show(4);
    const tone = () => row().querySelector('td:nth-child(2) strong[title]')!.className;
    expect(tone()).toContain('text-green-700');
    const input = row().querySelector('input[type="number"]') as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: '2' } }); // 120 iron owed, only 100 held
    expect(tone()).toContain('text-red-700');
  });

  it('marks a bot poster with the bot tag', async () => {
    const { row } = await show(1);
    const player = row().querySelector('td:nth-child(3)') as HTMLElement;
    // The tag carries the same word the rest of the UI uses for a computer player, after the name.
    const tag = player.querySelector('.bot-tag');
    expect(tag).toBeTruthy();
    expect(tag!.textContent).toBe(t('player.bot'));
    expect(player.textContent).toContain('Test bot');
  });

  it("keeps the bot tag on the name's line in card mode", async () => {
    const { row } = await show(1);
    const player = row().querySelector('td:nth-child(3)') as HTMLElement;
    // The cell is a wrapping flex row on a phone, so name and tag must be a single nowrap item: otherwise the tag drops
    // to a second line on a narrow card and the row grows for no reason.
    const unit = player.firstElementChild as HTMLElement;
    expect(unit.tagName).toBe('SPAN');
    expect(unit.className).toContain('inline-flex');
    expect(unit.className).toContain('whitespace-nowrap');
    expect(unit.contains(player.querySelector('.bot-tag'))).toBe(true);
    expect(unit.textContent).toContain('Test bot');
  });

  it('does not tag a human poster', async () => {
    g.tradeOffers = postOffers(1).map((o) => ({ ...o, fromId: 'h_other' }));
    g.players.h_other = { ...g.players[HUMAN_ID]!, id: 'h_other', name: 'A person', isHuman: true };
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const row = (document.querySelector('.table-game')!.closest('.panel') as HTMLElement).querySelector('tbody tr') as HTMLElement;
    const player = row.querySelector('td:nth-child(3)') as HTMLElement;
    expect(player.textContent).toContain('A person');
    expect(player.querySelector('.bot-tag')).toBeNull();
  });

  it('sends every copy in the batch when accepting', async () => {
    const { board, row } = await show(3);
    const input = row().querySelector('input[type="number"]') as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: '3' } });
    const sent: unknown[] = [];
    stubServer(g, { routes: { 'POST /api/v1/worlds/1/orders/accept-trade-offer': (body) => { sent.push(body); return { status: 200, body: null }; } } });
    fireEvent.click(within(board).getByRole('button', { name: 'Accept' }));
    expect(sent).toHaveLength(1);
    expect((sent[0] as { offerIds: string[] }).offerIds).toHaveLength(3);
  });

});

describe('trade offers panel', () => {
  /** The trade-offers table (the one above the global board), or null when it has no rows. */
  const offersTable = () => document.querySelectorAll('table.table-game')[0] as HTMLTableElement | undefined;

  it('renders my own offers as a table of their own', async () => {
    g.tradeOffers = [offer({ id: 'mine', fromId: HUMAN_ID, originId: home.id })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = offersTable()!;
    expect(table).toBeTruthy();
    const headers = [...table.querySelectorAll('thead th')].map((th) => th.textContent);
    expect(headers).toEqual(['Offer', 'For', 'From village', 'Max delivery', 'Ratio', 'Expires', 'Action']);
    expect(table.querySelectorAll('tbody tr')).toHaveLength(1);
  });

  it('names the posting village on my own offer rather than repeating my name', async () => {
    // Offers I posted carry no news: the player is always me, so the column says which village put it up.
    g.tradeOffers = [offer({ id: 'mine', fromId: HUMAN_ID, originId: home.id })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = [...document.querySelectorAll('table.table-game')]
      .find((x) => /From village/.test(x.textContent ?? '')) as HTMLTableElement;
    expect(table).toBeTruthy();
    expect([...table.querySelectorAll('thead th')].map((th) => th.textContent))
      .toEqual(['Offer', 'For', 'From village', 'Max delivery', 'Ratio', 'Expires', 'Action']);
    const second = table.querySelector('tbody tr td:nth-child(3)')!;
    // The village label plus its coordinates ("Ala's village 5|5"), not the bare player name the board's column shows.
    expect(second.textContent).toBe(`${villageLabelOf(g, home)} ${formatCoords(home.x, home.y)}`);
    expect(second.textContent).not.toBe(g.players[HUMAN_ID]!.name);
  });

  it('keeps identical deals from different villages apart', async () => {
    // Same amounts and delivery from two of my villages: separate offers, so they must not collapse into one row.
    const second = { ...home, id: 'v:9:9', x: 9, y: 9, name: 'second' };
    g.villages[second.id] = second;
    g.players[HUMAN_ID] = { ...g.players[HUMAN_ID]!, villageIds: [home.id, second.id] };
    g.tradeOffers = [
      offer({ id: 'a', fromId: HUMAN_ID, originId: home.id }),
      offer({ id: 'b', fromId: HUMAN_ID, originId: second.id }),
    ];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = [...document.querySelectorAll('table.table-game')]
      .find((x) => /From village/.test(x.textContent ?? '')) as HTMLTableElement;
    const rows = [...table.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(2);
    for (const r of rows) expect(r.textContent).not.toContain('×2');
  });

  it('keeps identical deals with different delivery limits apart', async () => {
    // The server's postKey counts these as two distinct deals, so the client must not fold them into one ×N row, which
    // would show a single max delivery that is wrong for half the offers behind it.
    g.tradeOffers = [
      offer({ id: 'a', fromId: HUMAN_ID, originId: home.id, maxDeliveryMs: 2 * HOUR }),
      offer({ id: 'b', fromId: HUMAN_ID, originId: home.id, maxDeliveryMs: 8 * HOUR }),
    ];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = [...document.querySelectorAll('table.table-game')]
      .find((x) => /From village/.test(x.textContent ?? '')) as HTMLTableElement;
    const rows = [...table.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(2);
    for (const r of rows) expect(r.textContent).not.toContain('\u00d72');
    expect(rows.map((r) => r.querySelectorAll('td')[3]?.textContent)).toEqual(['2 h 0 min', '8 h 0 min']);
  });

  it('raises the reason in the toast when an incoming offer cannot be taken', async () => {
    // The same rule as the board, on the offers panel's own Accept: the row stays clean and the refusal is announced.
    home.buildings.market = 1; // one merchant = 1000 resources, and this deal asks for more
    g.tradeOffers = [offer({ id: 'heavy', want: { wood: 0, clay: 0, iron: 1500 } })];
    stubServer(g);
    await useGameStore.getState().boot();
    useGameStore.getState().clearError();
    render(<MarketScreen />);
    const row = offersTable()!.querySelector('tbody tr') as HTMLElement;
    const button = row.querySelector('button') as HTMLButtonElement;
    expect(button.disabled).toBe(false);
    expect(button.getAttribute('title')).toContain('market');
    expect(row.textContent).not.toContain('Needs 2 merchants');

    fireEvent.click(button);
    expect(useGameStore.getState().lastError).toContain('Needs 2 merchants but the market only has 1');
  });

  it('leaves the trip out of my own offer and shows the max delivery limit I set', async () => {
    // A public post has no taker, so the merchant trip cannot be known — the column carries my posting limit instead.
    g.tradeOffers = [offer({ id: 'mine', fromId: HUMAN_ID, maxDeliveryMs: 6 * HOUR })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const row = offersTable()!.querySelector('tbody tr') as HTMLElement;
    // Same short form the board's delivery column uses for a merchant trip.
    expect(row.querySelectorAll('td')[3]?.textContent).toBe('6 h 0 min');
    expect(screen.getByRole('button', { name: 'Withdraw' })).toBeTruthy();
  });

  it('says "no limit" on my own offer posted without a delivery limit', async () => {
    g.tradeOffers = [offer({ id: 'mine', fromId: HUMAN_ID, maxDeliveryMs: 0 })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const row = offersTable()!.querySelector('tbody tr') as HTMLElement;
    expect(row.querySelectorAll('td')[3]?.textContent).toBe('no limit');
  });

});

describe('the global market shows only other players', () => {
  const globalMarket = () => globalMarketPanel().querySelector('table.table-game');

  it('leaves my own public posts out, even though they are on the board for others', async () => {
    g.tradeOffers = [
      offer({ id: 'theirs', fromId: 'bot_test' }),
      offer({ id: 'mine', fromId: HUMAN_ID }),
    ];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = globalMarket()!;
    expect(table).toBeTruthy();
    const names = [...table.querySelectorAll('tbody tr td:nth-child(3)')].map((td) => td.textContent);
    // Exactly the other player's post, never mine.
    expect(names).toHaveLength(1);
    expect(names[0]).toContain('Test bot');
    // And my own post is still reachable above, in the trade-offers panel.
    expect(screen.getByRole('button', { name: 'Withdraw' })).toBeTruthy();
  });

  it('leaves my own posts out too', async () => {
    g.tradeOffers = [
      offer({ id: 'theirs', fromId: 'bot_test' }),
      offer({ id: 'mine', fromId: HUMAN_ID }),
    ];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const names = [...globalMarket()!.querySelectorAll('tbody tr td:nth-child(3)')].map((td) => td.textContent);
    expect(names).toHaveLength(1);
    expect(names[0]).toContain('Test bot');
  });

});

describe('filtering the global market by who posted', () => {
  /** Both kinds of poster on the board: the fixture's bot and a second human, each with a village in range. */
  const bothPosters = () => {
    const other = makeBotVillage(g);
    other.ownerId = 'human_test';
    g.players.human_test = {
      id: 'human_test', name: 'Other player', isHuman: true, color: '#a3522b', villageIds: [other.id], coins: 0,
      noblesTrained: 0, buildQueueSlots: 2, protectedUntil: 0, joinAt: 0, status: 'active', points: 0,
      history: { hourly: [], daily: [] },
    };
    g.tradeOffers = [
      offer({ id: 'from_bot', fromId: 'bot_test' }),
      offer({ id: 'from_human', fromId: 'human_test', originId: other.id }),
    ];
  };
  /** The poster column of the board's visible rows. */
  const posters = () => [...globalMarketPanel().querySelectorAll('tbody tr td:nth-child(3)')].map((td) => td.textContent ?? '');
  const selectByLabel = (panel: HTMLElement, label: string) => {
    const field = [...panel.querySelectorAll('label')].find((l) => l.textContent?.startsWith(label))!;
    return field.querySelector('select') as HTMLSelectElement;
  };

  it('shows everyone by default, then only players or only bots', async () => {
    bothPosters();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const panel = globalMarketPanel();

    expect(posters().sort()).toEqual([expect.stringContaining('Other player'), expect.stringContaining('Test bot')].sort());

    const filter = selectByLabel(panel, 'Posted by:');
    fireEvent.change(filter, { target: { value: 'human' } });
    expect(posters().every((p) => p.includes('Other player'))).toBe(true);
    expect(posters().some((p) => p.includes('Test bot'))).toBe(false);

    fireEvent.change(filter, { target: { value: 'bot' } });
    expect(posters().every((p) => p.includes('Test bot'))).toBe(true);
    expect(posters().some((p) => p.includes('Other player'))).toBe(false);

    fireEvent.change(filter, { target: { value: 'any' } });
    expect(posters()).toHaveLength(2);
  });

  it('starts on its own page again when the filter changes', async () => {
    // Eleven bot posts fill two pages; filtering must not leave the reader stranded on an empty second page.
    g.tradeOffers = Array.from({ length: 11 }, (_, i) => offer({
      id: `b${i}`, fromId: 'bot_test', give: { wood: 10 + i, clay: 0, iron: 0 }, want: { wood: 0, clay: 0, iron: 5 },
    }));
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const panel = globalMarketPanel();
    fireEvent.click(screen.getByRole('button', { name: 'Next →' }));
    expect(panel.textContent).toContain('2 / 2');

    fireEvent.change(selectByLabel(panel, 'Posted by:'), { target: { value: 'bot' } });
    expect(panel.textContent).toContain('1 / 2');
  });
});

describe('posting a deal several times', () => {
  /** Open the form and return its number inputs. The submit button must be looked up *after* typing: each change
   * re-renders the panel, and a button captured earlier keeps the closure from the render that made it. */
  const openForm = () => {
    const summary = screen.getByText('Post an offer');
    fireEvent.click(summary);
    const panel = summary.closest('details')!.parentElement as HTMLElement;
    const nums = [...panel.querySelectorAll('input[type="number"]')] as HTMLInputElement[];
    const submit = () => within(panel).getAllByRole('button').find((b) => /Post/.test(b.textContent ?? ''))!;
    return { panel, nums, submit };
  };

  it('posts the deal once per copy, and labels the button with the count', async () => {
    const sent: unknown[] = [];
    // The order answers with the fresh state, like the real server, so the loop can post the next copy.
    stubServer(g, { routes: { 'POST /api/v1/worlds/1/orders/create-trade-offer': (body) => {
      sent.push(body);
      return { status: 200, body: stateFixture(g) };
    } } });
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    // nums: 0 = delivery hours, 1..3 = give wood/clay/iron, 4..6 = want, 7 = copies
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '20' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '16' } });
    fireEvent.focus(nums[7]!); fireEvent.change(nums[7]!, { target: { value: '4' } });
    expect(submit().textContent).toBe('Post 4 offers');

    fireEvent.click(submit());
    await vi.waitFor(() => expect(sent).toHaveLength(4));
    // Every copy is the same deal, which is the point: several takers can each take a piece.
    for (const body of sent) {
      expect((body as { give: { wood: number } }).give.wood).toBe(20);
      expect((body as { want: { iron: number } }).want.iron).toBe(16);
    }
  });

  /** Fill this village's public-post slots with distinct deals, so it has no room for another new deal. */
  const fillBoard = () => {
    g.tradeOffers = Array.from({ length: TRADE_BALANCE.publicPerVillage }, (_, i) => offer({
      id: `mine${i}`, fromId: HUMAN_ID, originId: home.id, maxDeliveryMs: 0,
      give: { wood: 10 + i, clay: 0, iron: 0 }, want: { wood: 0, clay: 0, iron: 5 + i },
    }));
  };

  it('does not call a post a free copy when only its delivery limit differs', async () => {
    // The server counts the delivery limit as part of a deal (dealKey), so the same amounts at another limit is a
    // new post and has to fill a slot. Saying "free copies" here would promise what the refusal denies.
    fillBoard();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '10' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '5' } });
    // The first open deal is exactly 10 wood for 5 iron at "no limit"; give this post a limit instead.
    fireEvent.focus(nums[0]!); fireEvent.change(nums[0]!, { target: { value: '2' } });
    expect(submit()).toHaveProperty('disabled', true);
    expect(submit().getAttribute('title')).toContain(t('market.boardFullHint', { max: TRADE_BALANCE.publicPerVillage }));
  });

  it('treats the same amounts at the same limit as a free copy', async () => {
    fillBoard();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    // Same amounts and the same "no limit" as the first open deal, so this is a copy of it and free of the cap.
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '10' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '5' } });
    expect(submit()).toHaveProperty('disabled', false);
    expect(submit().getAttribute('title')).toBe('');
  });

  it('counts the cap per village, not pooled across the villages I own', async () => {
    // A second village of mine: under a pooled cap its slots would top up this village's three, and a fourth deal
    // here would go up. The cap is per origin village, so this village's own three fill it.
    const second: Village = { ...home, id: 'second', x: home.x + 1, y: home.y + 1 };
    g.villages[second.id] = second;
    g.players[HUMAN_ID] = { ...g.players[HUMAN_ID]!, villageIds: [home.id, second.id] };
    fillBoard();
    stubServer(g);
    await useGameStore.getState().boot();
    // Boot may recall another village from a previous test; this deal is posted from `home`, so make it active.
    useGameStore.getState().setActiveVillage(home.id);
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '77' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '5' } });
    expect(submit()).toHaveProperty('disabled', true);
    expect(submit().getAttribute('title')).toContain(t('market.boardFullHint', { max: TRADE_BALANCE.publicPerVillage }));
  });

  it('no longer shows a per-day count on the form', async () => {
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { panel } = openForm();
    expect(panel.textContent).not.toContain('left today');
  });

  it('raises the refusal in the toast when the form is opened at the cap', async () => {
    fillBoard();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    useGameStore.getState().clearError();
    openForm();
    expect(useGameStore.getState().lastError).toBe(t('market.boardFullHint', { max: TRADE_BALANCE.publicPerVillage }));
  });

  it('still lets a copy of a deal already out be posted after that warning', async () => {
    // The warning is about new deals. A copy needs no slot, so the form stays usable at the cap.
    fillBoard();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '10' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '5' } });
    expect(submit()).toHaveProperty('disabled', false);
    expect(submit().getAttribute('title')).toBe('');
  });

  it('refuses to post more than the village holds', async () => {
    home.resources = { wood: 100, clay: 5000, iron: 5000 };
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { nums, submit } = openForm();
    fireEvent.focus(nums[1]!); fireEvent.change(nums[1]!, { target: { value: '500' } });
    fireEvent.focus(nums[6]!); fireEvent.change(nums[6]!, { target: { value: '50' } });
    expect(submit()).toHaveProperty('disabled', true);
    expect(submit().getAttribute('title')).toContain('You do not hold that much to give');
  });

});

describe('merchant shipments under the market', () => {
  it('lists merchant shipments but never a march', async () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'trade', originId: home.id, targetId: bot.id, merchants: 2, haul: { wood: 700, clay: 0, iron: 0 } });
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 12 } });
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    expect(screen.getByText('Merchant shipments')).toBeTruthy();
    expect(screen.getByTitle(t('rally.cmd.trade'))).toBeTruthy();
    expect(screen.getByText('· 2 merchants')).toBeTruthy();
    expect(screen.queryByText('Attack')).toBeNull();
  });

  it('is absent when the only thing on the road is a march', async () => {
    const bot = makeBotVillage(g);
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { spear: 12 } });
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    expect(screen.queryByText('Merchant shipments')).toBeNull();
  });
});

describe('the plain transfer form', () => {
  /** A second village of mine to send to, so the form has a target. */
  const withOther = () => {
    const other = { ...home, id: 'v:9:9', x: 9, y: 9, name: 'other' };
    g.villages[other.id] = other;
    g.players[HUMAN_ID] = { ...g.players[HUMAN_ID]!, villageIds: [home.id, other.id] };
  };

  /** The transfer panel under the merchant line, and its Send button. */
  const sendForm = () => {
    const form = [...document.querySelectorAll('.panel')].find((p) => p.querySelector('h3')?.textContent?.startsWith('Merchants'))! as HTMLElement;
    return { button: within(form).getByRole('button', { name: 'Send resources' }) as HTMLButtonElement, form };
  };

  it('keeps the refusal out of the panel and raises it on press', async () => {
    // Level 1 = one merchant = 1000 resources. Sending 1500 can never be shipped, so the deal is refused.
    withOther();
    home.buildings.market = 1;
    home.resources = { wood: 5000, clay: 5000, iron: 5000 };
    stubServer(g);
    await useGameStore.getState().boot();
    useGameStore.getState().clearError();
    render(<MarketScreen />);
    const inputs = [...document.querySelectorAll('input[type="number"]')] as HTMLInputElement[];
    // The first three number boxes in the panel are the resource amounts of the transfer form.
    fireEvent.focus(inputs[0]!);
    fireEvent.change(inputs[0]!, { target: { value: '1500' } });

    const { button, form } = sendForm();
    expect(button.disabled).toBe(false);
    expect(button.getAttribute('title')).toContain('Upgrade the Market to get more merchants');
    expect(form.textContent).not.toContain('Upgrade the Market');

    fireEvent.click(button);
    expect(useGameStore.getState().lastError).toContain('Needs 2 merchants but the market only has 1');
  });

  it('still disables Send when there is no target village at all', async () => {
    stubServer(g); // one village only: the form says so instead of drawing the transfer row
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    expect(screen.queryByRole('button', { name: 'Send resources' })).toBeNull();
    expect(screen.getByText(/conquer a second village/i)).toBeTruthy();
  });
});

describe('a haul the market cannot handle', () => {
  /** The accept button in the global market. The reasons are no longer printed under it: they come up in the app's
   * error toast when the player presses the button, which keeps a long sentence off a phone card. */
  const firstGlobalRow = () => {
    const tables = [...document.querySelectorAll('table.table-game')] as HTMLTableElement[];
    const gm = tables.find((t) => [...t.querySelectorAll('thead th')].some((th) => th.textContent === 'Available'))!;
    const tr = gm.querySelector('tbody tr') as HTMLElement;
    const cell = tr.querySelectorAll('td')[6] as HTMLElement;
    return { button: cell.querySelector('button') as HTMLButtonElement, cell };
  };

  it('leaves the accept button pressable and explains the refusal in the toast', async () => {
    // Market level 1 = one merchant = 1000 resources. A post costing 1500 cannot be shipped however long you wait.
    home.buildings.market = 1;
    home.resources = { wood: 9000, clay: 9000, iron: 9000 };
    g.tradeOffers = [offer({ id: 'big', want: { wood: 0, clay: 0, iron: 1500 } })];
    stubServer(g);
    await useGameStore.getState().boot();
    useGameStore.getState().clearError();
    render(<MarketScreen />);
    const { button, cell } = firstGlobalRow();
    // The reason is still known to the button (its tooltip) but no longer written into the card.
    expect(button.disabled).toBe(false);
    expect(button.getAttribute('title')).toContain('Needs 2 merchants but the market only has 1');
    expect(cell.textContent).not.toContain('Needs 2 merchants');
    expect(useGameStore.getState().lastError).toBeNull();

    fireEvent.click(button);
    expect(useGameStore.getState().lastError).toContain('Needs 2 merchants but the market only has 1');
  });

  it('says merchants are busy when they are enough in total but already on the road', async () => {
    // Level 3 = three merchants; one is away with a trade, so only two are free and the haul needs three.
    home.buildings.market = 3;
    home.resources = { wood: 9000, clay: 9000, iron: 9000 };
    g.commands = [{
      id: 'c_busy', type: 'trade', originId: home.id, targetId: botVillageId(g), units: {},
      haul: { wood: 500, clay: 0, iron: 0 }, merchants: 1, departedAt: g.now - 1000, arrivesAt: g.now + 60_000,
    }];
    g.tradeOffers = [offer({ id: 'heavy', want: { wood: 0, clay: 0, iron: 2500 } })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const { button } = firstGlobalRow();
    expect(button.disabled).toBe(false);

    fireEvent.click(button);
    expect(useGameStore.getState().lastError).toContain('Not enough merchants: 3 needed, 2 free');
  });

  it('sends nothing when the refusal is raised', async () => {
    home.buildings.market = 1;
    home.resources = { wood: 9000, clay: 9000, iron: 9000 };
    g.tradeOffers = [offer({ id: 'big', want: { wood: 0, clay: 0, iron: 1500 } })];
    stubServer(g);
    await useGameStore.getState().boot();
    const sent: unknown[] = [];
    stubServer(g, { routes: { 'POST /api/v1/worlds/1/orders/accept-trade-offer': (body) => { sent.push(body); return { status: 200, body: null }; } } });
    render(<MarketScreen />);
    fireEvent.click(firstGlobalRow().button);
    expect(sent).toHaveLength(0);
  });

});

describe('an offer I posted', () => {
  it('is listed once, under my own offers, and not on the global board', async () => {
    g.tradeOffers = [offer({ id: 'mine', fromId: HUMAN_ID, originId: home.id })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = document.querySelectorAll('table.table-game')[0] as HTMLTableElement;
    expect(table.querySelectorAll('tbody tr')).toHaveLength(1);
    // The board is where *other* players' posts are listed; mine leaves it empty, with its own message instead.
    expect(screen.queryByText(t('market.boardEmpty'))).toBeTruthy();
  });
});

describe('grouping and withdrawing my own offers', () => {
  /** The trade-offers table (the panel above the global market), or null when it has no rows. */
  const offersTable = () => {
    const tables = [...document.querySelectorAll('table.table-game')] as HTMLTableElement[];
    return tables.find((t) => ![...t.querySelectorAll('thead th')].some((th) => th.textContent === 'Available'));
  };

  /** Four of my own identical public posts. */
  const mineFour = () => {
    g.tradeOffers = [0, 1, 2, 3].map((i) => offer({ id: `mine${i}`, fromId: HUMAN_ID }));
  };

  it('collapses identical offers of mine into one row with a count', async () => {
    mineFour();
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = offersTable()!;
    expect(table.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(table.querySelector('tbody tr td:nth-child(3)')?.textContent).toContain('×4');
    expect(screen.getAllByRole('button', { name: /Withdraw/ })).toHaveLength(1);
  });

  it('withdraws as many copies as asked, not always the whole group', async () => {
    mineFour();
    const cancelled: unknown[] = [];
    stubServer(g, { routes: { 'POST /api/v1/worlds/1/orders/cancel-trade-offer': (body) => {
      cancelled.push(body);
      return { status: 200, body: stateFixture(g) };
    } } });
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const table = offersTable()!;
    const row = table.querySelector('tbody tr') as HTMLElement;
    const count = row.querySelector('input[type="number"]') as HTMLInputElement;
    expect(count.max).toBe('4');
    fireEvent.focus(count);
    fireEvent.change(count, { target: { value: '2' } });
    // The button says how many it will take back.
    expect(row.querySelector('button')?.textContent).toBe('Withdraw 2');
    fireEvent.click(row.querySelector('button')!);
    await vi.waitFor(() => expect(cancelled).toHaveLength(2));
    expect(cancelled.map((b) => (b as { id: string }).id)).toEqual(['mine0', 'mine1']);
  });

  it('shows a plain Withdraw when the deal has only one copy', async () => {
    g.tradeOffers = [offer({ id: 'solo', fromId: HUMAN_ID })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    const row = offersTable()!.querySelector('tbody tr') as HTMLElement;
    expect(row.querySelector('input[type="number"]')).toBeNull();
    expect(row.querySelector('button')?.textContent).toBe('Withdraw');
  });
});

describe('trading switched off in the world', () => {
  it('leaves the market as it was before players could trade, and says why', async () => {
    g.config.tradingEnabled = false;
    g.tradeOffers = [offer({ id: 'botpost' })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    // The merchant line, the transfer panel and the travelling trader are what the market was before player trading.
    // This village has no second own village here, so the panel states that instead of offering a form.
    expect(screen.getByText(/Merchants:/)).toBeTruthy();
    expect(screen.getByText(/Trading works between your own villages/)).toBeTruthy();
    expect(screen.getByText('Travelling trader')).toBeTruthy();
    // Only the things that came with player trading are gone, with one line saying so.
    expect(screen.getByText('Trading is switched off in this world')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
    expect(screen.queryByRole('button', { name: /Post offer/ })).toBeNull();
    expect(screen.queryByText('Global Market')).toBeNull();
    expect(screen.queryByText('Your trade offers')).toBeNull();
    // And the merchant shipments panel came with player trading, so it is gone too.
    expect(screen.queryByText('Merchant shipments')).toBeNull();
    // The offer the server sent is not shown at all: no amounts, no poster, no withdrawal.
    const body = document.body.textContent ?? '';
    expect(body).not.toContain('×1');
    expect(screen.queryByRole('button', { name: /Withdraw/ })).toBeNull();
  });

  it('offers the transfer form once a second own village exists, even with trading off', async () => {
    g.config.tradingEnabled = false;
    const second = { ...home, id: 'v:9:9', x: 9, y: 9 };
    g.villages[second.id] = second;
    g.players[HUMAN_ID] = { ...g.players[HUMAN_ID]!, villageIds: [home.id, second.id] };
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    // Moving resources between your own villages is not trading with anyone, so the switch does not remove it.
    expect(screen.getByRole('button', { name: 'Send resources' })).toBeTruthy();
    expect(screen.queryByText(/conquer a second village/i)).toBeNull();
  });

  it('shows the full market again when the world has it on', async () => {
    g.config.tradingEnabled = true;
    g.tradeOffers = [offer({ id: 'botpost' })];
    stubServer(g);
    await useGameStore.getState().boot();
    render(<MarketScreen />);
    expect(screen.queryByText('Trading is switched off in this world')).toBeNull();
    expect(screen.getByText('Global Market')).toBeTruthy();
  });
});

describe('settled trade shipments', () => {
  it('cannot be recalled from the rally point', async () => {
    // Both legs of an accepted offer are marked settled by the server; the recall must not be offered for them.
    const leg = {
      id: 'settled', type: 'trade' as const, originId: home.id, targetId: botVillageId(g),
      units: {}, haul: { wood: 100, clay: 0, iron: 0 }, merchants: 1,
      departedAt: g.now, arrivesAt: g.now + 10 * 60_000, settled: true,
    };
    expect(canCancelCommand(g, leg)).toBe(false);
    // An ordinary market trade keeps its cancel window.
    expect(canCancelCommand(g, { ...leg, settled: undefined })).toBe(true);
  });
});
