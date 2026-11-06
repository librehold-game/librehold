// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { MapView } from '../src/ui/views/MapView';
import { armiesOnMap } from '../src/ui/map/armies';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import type { Command, GameState, Village } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame, stateFixture } from './helpers';
import { stubServer } from './serverStub';

const HOUR = 3_600_000;

let g: GameState;
let home: Village;
let bot: Village;
let bodies: unknown[];

function addCmd(c: Partial<Command> & Pick<Command, 'id' | 'type' | 'originId' | 'targetId'>): Command {
  const full: Command = { units: { lightCav: 5 }, departedAt: g.now - 60_000, arrivesAt: g.now + HOUR, ...c };
  g.commands.push(full);
  return full;
}

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  bot = makeBotVillage(g);
  bodies = [];
  useUiStore.setState({ view: 'map', mapCenter: null });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('farm raids on the map', () => {
  it('are shown unless the player hides them, and their ways home with them', () => {
    addCmd({ id: 'farm', type: 'attack', originId: home.id, targetId: bot.id, farm: true });
    addCmd({ id: 'home', type: 'return', originId: bot.id, targetId: home.id, farm: true });
    addCmd({ id: 'manual', type: 'attack', originId: home.id, targetId: bot.id });
    const ids = () => armiesOnMap(g).map((a) => a.id).sort();
    expect(ids()).toEqual(['farm', 'home', 'manual']);
    g.farmAssistant.hideOnMap = true;
    expect(ids()).toEqual(['manual']);
  });

  it('the map checkbox hides them and saves the choice on the server', async () => {
    addCmd({ id: 'farm', type: 'attack', originId: home.id, targetId: bot.id, farm: true });
    addCmd({ id: 'manual', type: 'attack', originId: home.id, targetId: bot.id });
    stubServer(g, { routes: {
      'POST /api/v1/worlds/1/orders/hide-farm-on-map': (body) => {
        bodies.push(body);
        g.farmAssistant.hideOnMap = (body as { hide: boolean }).hide;
        return { status: 200, body: stateFixture(g) };
      },
    } });
    await useGameStore.getState().boot();
    const { container } = render(<MapView />);
    const box = screen.getByRole('checkbox', { name: 'Show farming' }) as HTMLInputElement;
    expect(box.checked).toBe(true);
    expect(container.querySelector('[data-army="farm"]')).toBeTruthy();
    await act(async () => { fireEvent.click(box); });
    expect(bodies).toEqual([{ hide: true }]);
    expect((screen.getByRole('checkbox', { name: 'Show farming' }) as HTMLInputElement).checked).toBe(false);
    expect(container.querySelector('[data-army="farm"]')).toBeNull();
    expect(container.querySelector('[data-army="manual"]')).toBeTruthy();
  });
});
