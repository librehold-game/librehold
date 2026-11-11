// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { canCancelCommand, isOwnCommand, playerCommands, unitsAway } from '../src/engine/commands';
import { reportVillageOf } from '../src/engine/reports';
import { HUMAN_ID, MS_PER_HOUR, type Command, type GameState, type Report, type Village } from '../src/engine/types';
import type { CommandView } from '../src/net/types';
import { toGameState } from '../src/net/toGameState';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { IncomingList } from '../src/ui/components/IncomingList';
import { armiesOnMap } from '../src/ui/map/armies';
import { MapView } from '../src/ui/views/MapView';
import { RallyPointView } from '../src/ui/views/RallyPointView';
import { ReportsView } from '../src/ui/views/ReportsView';
import { armyOwnerId } from '../src/ui/watch/WatchCards';
import { sceneModel } from '../src/ui/scene/sceneModel';
import { activeVillage, addCommand, addOwnVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer } from './serverStub';

const NO_HOME = 'Nothing was taken: the village the attackers set out from was conquered, so they have no home to carry loot to.';

let g: GameState;
let home: Village;
let bot: Village;
let bot2: Village;

type NewCommand = Partial<Command> & Pick<Command, 'type' | 'originId' | 'targetId'>;
/** The player's army, cut loose from a village they have lost since. */
const own = (c: NewCommand) => addCommand(g, { cutLoose: true, ownerId: HUMAN_ID, ...c });
/** Someone else's army cut loose from its village, as the server sends it: no owner, no units. */
const foreign = (c: NewCommand) => addCommand(g, { cutLoose: true, ...c });
/** The village the player has just taken from someone whose armies are still out. */
const taken = () => addOwnVillage(g, home.x + 3, home.y + 3, 'Taken');

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  bot2 = makeBotVillage(g);
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('armies cut loose by a conquest', () => {
  it('the own one arrives with the player as its owner, a foreign one with neither owner nor units', () => {
    const mine = own({ type: 'attack', originId: bot.id, targetId: bot2.id, units: { axe: 10 } });
    const theirs = addCommand(g, { type: 'attack', originId: bot.id, targetId: home.id, units: { axe: 5 }, cutLoose: true, ownerId: 'bot_test' });
    const st = stateFixture(g, 'h7');
    expect(st.commands.find((c) => c.id === mine.id)!.ownerId).toBe('h7');
    const cs = toGameState(st, [], []).commands;
    expect(cs.find((c) => c.id === mine.id)).toMatchObject({ cutLoose: true, ownerId: HUMAN_ID, units: { axe: 10 } });
    const f = cs.find((c) => c.id === theirs.id)!;
    expect(f.cutLoose).toBe(true);
    expect(f.ownerId).toBeUndefined();
    expect(f.units).toEqual({});
  });

  it('belong to their owner, not to the village they left', () => {
    const x = taken();
    const mine = own({ type: 'attack', originId: bot.id, targetId: bot2.id });
    const leaving = foreign({ type: 'attack', originId: x.id, targetId: bot2.id });
    const coming = foreign({ type: 'attack', originId: x.id, targetId: home.id });
    const ordinary = addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id });
    const back = addCommand(g, { type: 'return', originId: bot.id, targetId: home.id });
    const ids = playerCommands(g).map((c) => c.id);
    expect(ids).toContain(mine.id);
    expect(ids).not.toContain(leaving.id);
    expect(ids).toContain(coming.id);
    expect([mine, leaving, coming, ordinary, back].map((c) => isOwnCommand(g, c))).toEqual([true, false, false, true, true]);
  });

  it('cannot be recalled and take no room in any village', () => {
    const fresh = own({ type: 'attack', originId: bot.id, targetId: bot2.id });
    const hit = own({ type: 'attack', originId: bot.id, targetId: bot2.id, fieldHit: true });
    expect(canCancelCommand(g, fresh)).toBe(false);
    expect(canCancelCommand(g, hit)).toBe(false);
    expect(canCancelCommand(g, { ...fresh, cutLoose: undefined, ownerId: undefined })).toBe(true);
    own({ type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } });
    expect(unitsAway(g, home.id).axe).toBe(0);
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 4 } });
    expect(unitsAway(g, home.id).axe).toBe(4);
  });

  it('the map draws the own ones as the player\'s and a foreign one from an own village as incoming', () => {
    const x = taken();
    own({ id: 'la', type: 'attack', originId: bot.id, targetId: bot2.id, units: { axe: 10 } });
    own({ id: 'li', type: 'intercept', originId: bot.id, targetId: '', point: { x: home.x + 0.5, y: home.y + 1.5 }, units: { lightCav: 5 },
      intercept: { commandId: 'x', leg: { originId: bot2.id, targetId: home.id, departedAt: g.now, arrivesAt: g.now + 2 * MS_PER_HOUR } } });
    foreign({ id: 'fa', type: 'attack', originId: x.id, targetId: home.id });
    const byId = Object.fromEntries(armiesOnMap(g).map((a) => [a.id, a]));
    expect(byId.la).toMatchObject({ kind: 'ownAttack', units: { axe: 10 } });
    expect(byId.li!.kind).toBe('ownIntercept');
    expect(byId.fa).toMatchObject({ kind: 'incomingAttack', units: null });
  });

  it('the rally point lists the own one under any active village, without Recall', () => {
    own({ type: 'attack', originId: bot.id, targetId: bot2.id, units: { axe: 10 } });
    addCommand(g, { type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 3 } });
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RallyPointView />);
    fireEvent.click(screen.getByRole('button', { name: /^Commands/ }));
    expect((screen.getByLabelText('active village only') as HTMLInputElement).checked).toBe(true);
    expect(document.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(screen.queryAllByRole('button', { name: 'Recall' })).toHaveLength(1);
  });

  it('the rally point of the new owner of its village lists a foreign one under its target, as incoming', () => {
    const x = taken();
    foreign({ type: 'attack', originId: x.id, targetId: home.id });
    g.activeVillageId = x.id;
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<RallyPointView />);
    fireEvent.click(screen.getByRole('button', { name: /^Commands/ }));
    expect(document.querySelectorAll('tbody tr')).toHaveLength(0);
    cleanup();
    g.activeVillageId = home.id;
    useGameStore.setState({ game: { ...g }, phase: 'ready' });
    render(<RallyPointView />);
    fireEvent.click(screen.getByRole('button', { name: /^Commands/ }));
    const rows = [...document.querySelectorAll('tbody tr')].filter((r) => r.textContent!.includes('INCOMING'));
    expect(rows).toHaveLength(1);
    expect(rows[0]!.className).toContain('text-bad');
    expect(screen.queryAllByRole('button', { name: 'Recall' })).toHaveLength(0);
  });

  it('the village scene shows a winning hit by one from a village the player holds now as a defeat', () => {
    const x = taken();
    const hit = (originId: string, ownerId: string, targetId: string): Report => ({
      id: `r-${originId}-${targetId}`, time: g.now, type: 'attack', originId, targetId,
      origin: { ...reportVillageOf(g, originId), ownerId }, target: { ...reportVillageOf(g, targetId), ownerId: 'h7' },
      attacker: { units: { axe: 10 }, losses: {} }, defender: { units: {}, losses: {} },
      luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
    });
    const defeated = (r: Report) => sceneModel(toGameState(stateFixture(g, 'h7'), [r], []), r.targetId)!.defeated;
    expect(defeated(hit(x.id, 'bot_test', home.id))).toBe(true);
    expect(defeated(hit(bot.id, 'bot_test', home.id))).toBe(true);
    expect(defeated(hit(home.id, 'h7', x.id))).toBe(false);
  });

  it('the incoming list shows a foreign one from an own village, not an own one landing at home', () => {
    const x = taken();
    foreign({ type: 'attack', originId: x.id, targetId: home.id });
    own({ type: 'attack', originId: bot.id, targetId: home.id });
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<IncomingList villageId={home.id} />);
    expect(screen.getByText('Incoming attacks')).toBeTruthy();
    expect(screen.getByText(/Taken/)).toBeTruthy();
    expect(screen.getAllByRole('button', { name: 'Dodge' })).toHaveLength(1);
  });

  it('the army card of the own one shows its troops and the rally point, but no Cancel or Turn back', async () => {
    own({ id: 'own', type: 'attack', originId: bot.id, targetId: bot2.id, units: { axe: 777 } });
    useUiStore.setState({ view: 'map', mapCenter: null });
    stubServer(g, { me: 'h7' });
    await useGameStore.getState().boot();
    const { container } = render(<MapView />);
    fireEvent.click(container.querySelector<SVGGElement>('[data-army="own"] [data-marker]')!);
    const card = screen.getByRole('dialog', { name: 'Your attack' });
    expect(card.textContent).toContain('777');
    expect(within(card).queryByRole('button', { name: 'Cancel' })).toBeNull();
    expect(within(card).queryByRole('button', { name: 'Turn back' })).toBeNull();
    expect(within(card).getByRole('button', { name: 'Open in rally point' })).toBeTruthy();
  });

  it('the god view colours one by its own owner', () => {
    const owners: Record<string, string> = { [bot.id]: 'bot_test', [home.id]: 'h7' };
    const owner = (id: string) => owners[id] ?? '';
    const c: CommandView = { id: 'c', type: 'attack', originId: bot.id, targetId: home.id, departedAt: 0, arrivesAt: 1 };
    expect(armyOwnerId(c, owner)).toBe('bot_test');
    expect(armyOwnerId({ ...c, type: 'return' }, owner)).toBe('h7');
    expect(armyOwnerId({ ...c, cutLoose: true, ownerId: 'h9' }, owner)).toBe('h9');
  });
});

describe('the report of an army with no home', () => {
  function report(over: Partial<Report>): Report {
    const x = home.x + 3, y = home.y + 3;
    return {
      id: `r${g.reports.length + 1}`, time: g.now, type: 'attack', originId: `v:${x}:${y}`, targetId: bot.id,
      origin: { id: `v:${x}:${y}`, name: '', x, y, ownerId: 'h7', ownerName: g.players[HUMAN_ID]!.name },
      target: reportVillageOf(g, bot.id),
      attacker: { units: { axe: 10 }, losses: { axe: 1 } }, defender: { units: {}, losses: {} },
      luck: 0, morale: 1, wallBefore: 0, wallAfter: 0, outcome: 'attackerWon', read: false,
      ...over,
    };
  }

  async function open(r: Report) {
    g.reports.push(r);
    useUiStore.setState({ view: 'reports', reportsHideAutoFarm: false, reportsVillageFilter: null });
    stubServer(g, { me: 'h7' });
    await useGameStore.getState().boot();
    render(<ReportsView />);
    fireEvent.click(await screen.findByText(/→/));
  }

  it('says why nothing was taken instead of the loot', async () => {
    await open(report({ noHome: true }));
    expect(screen.getByText(NO_HOME)).toBeTruthy();
    expect(screen.queryByText('Loot:')).toBeNull();
  });

  it('says nothing of it on a report without the flag', async () => {
    await open(report({}));
    expect(screen.queryByText(NO_HOME)).toBeNull();
    expect(screen.queryByText('Loot:')).toBeNull();
  });
});
