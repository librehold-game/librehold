// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
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

const record = (name: string, then?: () => void): StubRoute => (body) => {
  (bodies[name] ??= []).push(body);
  then?.();
  return { status: 200, body: stateFixture(g) };
};

async function boot(build: () => void) {
  build();
  stubServer(g, { routes: {
    'POST /api/v1/worlds/1/orders/cancel-command': record('cancel', () => {
      const c = g.commands.find((x) => x.id === 'own')!;
      Object.assign(c, { type: 'return', originId: c.targetId, targetId: c.originId, departedAt: g.now, arrivesAt: g.now + 60_000 });
    }),
    'POST /api/v1/worlds/1/orders/dodge': record('dodge'),
  } });
  await useGameStore.getState().boot();
}

const marker = (c: HTMLElement, id: string) => c.querySelector<SVGGElement>(`[data-army="${id}"] [data-marker]`)!;

/** happy-dom lays nothing out, so the map's room has to be faked to get the phone layout. */
function setClientWidth(px: number): () => void {
  const proto = globalThis.HTMLElement.prototype as unknown as Record<string, unknown>;
  const original = Object.getOwnPropertyDescriptor(proto, 'clientWidth');
  Object.defineProperty(proto, 'clientWidth', { configurable: true, get: () => px });
  return () => {
    if (original) Object.defineProperty(proto, 'clientWidth', original);
    else delete proto.clientWidth;
  };
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  bodies = {};
  useUiStore.setState({ view: 'map', mapCenter: null });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('army card', () => {
  it('tapping an own army opens its card with exact units and Cancel inside the window', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 777, noble: 1 } }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'own'));
    const card = screen.getByRole('dialog', { name: 'Your attack' });
    expect(card.textContent).toContain('777');
    expect(within(card).getByRole('button', { name: 'Cancel' })).toBeTruthy();
    expect(within(card).getByRole('button', { name: 'Open in rally point' })).toBeTruthy();
  });

  it('Cancel posts cancel-command and the card closes when the army turns back', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'own'));
    await act(async () => { fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' })); });
    expect(bodies.cancel).toEqual([{ commandId: 'own' }]);
    await vi.waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('no Cancel once the cancel window has passed', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 }, departedAt: g.now - HOUR, arrivesAt: g.now + HOUR }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'own'));
    expect(within(screen.getByRole('dialog')).queryByRole('button', { name: 'Cancel' })).toBeNull();
  });

  it('incoming attack card shows the threat label and Dodge, never units', async () => {
    await boot(() => addCmd({ id: 'inc', type: 'attack', originId: bot.id, targetId: home.id }));
    act(() => {
      const game = useGameStore.getState().game!;
      useGameStore.setState({ game: { ...game, commands: game.commands.map((c) => (c.id === 'inc' ? { ...c, units: { axe: 4321 } } : c)) } });
    });
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'inc'));
    const card = screen.getByRole('dialog', { name: 'Incoming attack' });
    expect(card.textContent).not.toMatch(/4[ ,.\u00a0]?321/);
    expect(card.innerHTML).not.toContain('unit-axe');
    expect(card.textContent).toContain(`${bot.x}|${bot.y}`);
    expect(card.textContent).toMatch(/scouts|light cavalry|heavy cavalry|infantry|swordsmen|rams|hetman/);
    await act(async () => { fireEvent.click(within(card).getByRole('button', { name: 'Dodge' })); });
    expect(bodies.dodge).toEqual([{ commandId: 'inc' }]);
    expect(within(card).getByRole('button', { name: 'Open the attacked village' })).toBeTruthy();
  });

  it('incoming support card offers only meeting it in the field', async () => {
    await boot(() => addCmd({ id: 'sup', type: 'support', originId: bot.id, targetId: home.id }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'sup'));
    const card = screen.getByRole('dialog', { name: 'Incoming support' });
    expect(within(card).queryAllByRole('button').map((b) => b.textContent)).toEqual(['✕', 'Scout', 'Attack in the field']);
    expect(card.textContent).toContain(`${bot.x}|${bot.y}`);
  });

  it('dragging from a marker pans instead of opening the card', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } }));
    const { container } = render(<MapView />);
    const tile = () => container.querySelector<HTMLElement>(`[data-tile="${home.x}|${home.y}"]`)!.style.left;
    const before = tile();
    const m = marker(container, 'own');
    fireEvent.pointerDown(m, { pointerId: 1, clientX: 100, clientY: 100 });
    fireEvent.pointerMove(m, { pointerId: 1, clientX: 130, clientY: 100 });
    fireEvent.pointerUp(m, { pointerId: 1, clientX: 130, clientY: 100 });
    fireEvent.click(m);
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(tile()).not.toBe(before);
  });

  it('Escape closes the card and choosing a village closes it too', async () => {
    await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } }));
    const { container } = render(<MapView />);
    fireEvent.click(marker(container, 'own'));
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
    fireEvent.click(marker(container, 'own'));
    fireEvent.click(container.querySelector(`[data-tile="${bot.x}|${bot.y}"]`)!);
    expect(screen.queryByRole('dialog', { name: 'Your attack' })).toBeNull();
  });

  it('the card is a sheet on a narrow screen', async () => {
    const restore = setClientWidth(360);
    try {
      await boot(() => addCmd({ id: 'own', type: 'attack', originId: home.id, targetId: bot.id, units: { axe: 10 } }));
      const { container } = render(<MapView />);
      fireEvent.click(marker(container, 'own'));
      const card = screen.getByRole('dialog', { name: 'Your attack' });
      expect(card.closest('[data-map-surface]')).toBeNull();
    } finally {
      restore();
    }
  });
});
