// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { unitName } from '../src/i18n/names';
import { MapView } from '../src/ui/views/MapView';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import type { Command, GameState, Village } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer, type StubRoute } from './serverStub';

const HOUR = 3_600_000;

let g: GameState;
let home: Village;
let bot: Village;
let bodies: Record<string, unknown[]>;

function addCmd(c: Partial<Command> & Pick<Command, 'id' | 'type' | 'originId' | 'targetId'>): Command {
  const full: Command = { units: {}, departedAt: g.now - 60_000, arrivesAt: g.now + HOUR, ...c };
  g.commands.push(full);
  return full;
}

const record = (name: string): StubRoute => (body) => {
  (bodies[name] ??= []).push(body);
  return { status: 200, body: stateFixture(g) };
};

async function boot(build: () => void) {
  build();
  stubServer(g, { routes: {
    'POST /api/v1/worlds/1/orders/intercept': record('intercept'),
    'POST /api/v1/worlds/1/orders/cancel-command': record('cancel'),
  } });
  await useGameStore.getState().boot();
}

/** Puts commands into the client state as the server sends them to a bystander (the test fixture leaves them out). */
function inject(...cs: Command[]) {
  act(() => {
    const game = useGameStore.getState().game!;
    useGameStore.setState({ game: { ...game, commands: [...game.commands, ...cs] } });
  });
}

const marker = (c: HTMLElement, id: string) => c.querySelector<SVGGElement>(`[data-army="${id}"] [data-marker]`)!;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  home.units.lightCav = 40;
  home.units.scout = 10;
  bodies = {};
  useUiStore.setState({ view: 'map', mapCenter: null });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('field battles in the client', () => {
  it('incoming attack card offers scouting and a field attack and posts intercept', async () => {
    await boot(() => addCmd({ id: 'inc', type: 'attack', originId: bot.id, targetId: home.id, departedAt: g.now, arrivesAt: g.now + HOUR }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'inc'));
    const card = screen.getByRole('dialog', { name: 'Incoming attack' });
    expect(within(card).getByRole('button', { name: 'Scout' })).toBeTruthy();
    fireEvent.click(within(card).getByRole('button', { name: 'Attack in the field' }));
    fireEvent.change(within(card).getByLabelText(unitName('lightCav')), { target: { value: '20' } });
    expect(within(card).getByText(/Meets it/)).toBeTruthy();
    await act(async () => { fireEvent.click(within(card).getByRole('button', { name: 'Send' })); });
    expect(bodies.intercept).toEqual([{ originId: home.id, commandId: 'inc', units: { lightCav: 20 } }]);
  });

  it('the form shows the meeting time and disables sending when too late', async () => {
    await boot(() => undefined);
    const far = makeBotVillage(g, home.x + 12, home.y + 9);
    // A neighbour's village is half a second away for the army, nine minutes for scouts from home.
    inject({ id: 'pa', type: 'attack', originId: far.id, targetId: bot.id, units: {}, departedAt: g.now - HOUR, arrivesAt: g.now + 500 });
    act(() => {
      const game = useGameStore.getState().game!;
      useGameStore.setState({ game: { ...game, config: { ...game.config, sightRadius: 3 } } });
    });
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'pa'));
    const card = screen.getByRole('dialog', { name: 'Passing attack' });
    fireEvent.click(within(card).getByRole('button', { name: 'Scout' }));
    fireEvent.change(within(card).getByLabelText(unitName('scout')), { target: { value: '2' } });
    expect(within(card).getByText(/Too late/)).toBeTruthy();
    expect((within(card).getByRole('button', { name: 'Send' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('passing army shows with its generic icon and the same actions', async () => {
    await boot(() => undefined);
    const bot2 = makeBotVillage(g);
    inject({ id: 'pa', type: 'attack', originId: bot.id, targetId: bot2.id, units: {}, departedAt: g.now, arrivesAt: g.now + HOUR });
    act(() => {
      const game = useGameStore.getState().game!;
      useGameStore.setState({ game: { ...game, config: { ...game.config, sightRadius: 3 } } });
    });
    const { container } = render(<MapView />);
    expect(container.querySelector('[data-army="pa"]')!.getAttribute('data-kind')).toBe('passingAttack');
    expect(container.querySelector('[data-army="pa"]')!.innerHTML).not.toContain('unit-');
    fireEvent.click(marker(container, 'pa'));
    const card = screen.getByRole('dialog', { name: 'Passing attack' });
    expect(within(card).getByRole('button', { name: 'Scout' })).toBeTruthy();
    expect(within(card).getByRole('button', { name: 'Attack in the field' })).toBeTruthy();
  });

  it('own army hit in the field shows Turn back', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 5 }, fieldHit: true, departedAt: g.now - HOUR, arrivesAt: g.now + HOUR }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'own'));
    const card = screen.getByRole('dialog', { name: 'Your attack' });
    await act(async () => { fireEvent.click(within(card).getByRole('button', { name: 'Turn back' })); });
    expect(bodies.cancel).toEqual([{ commandId: 'own' }]);
  });

  it('threat to my army shows no units', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 5 } }));
    inject({ id: 'th', type: 'intercept', originId: bot.id, targetId: '', point: { x: home.x + 1, y: home.y }, units: { axe: 4321 }, departedAt: g.now, arrivesAt: g.now + HOUR });
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'th'));
    const card = screen.getByRole('dialog', { name: 'Coming at your army' });
    expect(card.textContent).not.toMatch(/4[ ,. ]?321/);
    expect(within(card).queryByRole('button', { name: 'Scout' })).toBeNull();
  });
});
