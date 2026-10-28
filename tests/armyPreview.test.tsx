// @vitest-environment happy-dom
import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { BIOME, tileBiome } from '../src/ui/map/biomes';
import { armiesOnMap, groundAt } from '../src/ui/map/armies';
import { ArmyPreview } from '../src/ui/map/ArmyPreview';
import type { GameState } from '../src/engine/types';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';

const HOUR = 3_600_000;

function fakeContext(): CanvasRenderingContext2D {
  const props: Record<string | symbol, unknown> = {};
  return new Proxy(props, {
    get: (t, k) => (k in t ? t[k] : () => undefined),
    set: (t, k, v) => { t[k] = v; return true; },
  }) as unknown as CanvasRenderingContext2D;
}

beforeAll(() => {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation((() => fakeContext()) as never);
});
beforeEach(() => useUiStore.setState({ prefs: { ...useUiStore.getState().prefs, animations: 'full' } }));
afterEach(() => cleanup());

/** A meadow tile with water right next to it, east (armies stand on tile centres). */
function shore(g: GameState): [number, number] {
  for (let y = 5; y < g.config.mapSize - 5; y++) for (let x = 5; x < g.config.mapSize - 5; x++) {
    if (tileBiome(g.worldSeed, x, y) === BIOME.grass && groundAt(g.worldSeed, x + 0.5, y + 0.5) === 'meadow' && tileBiome(g.worldSeed, x + 1, y) === BIOME.water && groundAt(g.worldSeed, x + 1.5, y + 0.5) === 'water') return [x, y];
  }
  throw new Error('no shore');
}

function advance(ms: number) {
  act(() => {
    const g = useGameStore.getState().game!;
    useGameStore.setState({ game: { ...g, now: g.now + ms } });
  });
}

describe('army preview', () => {
  it('backdrop follows the tile under the army', () => {
    const g = newTestGame();
    const [x, y] = shore(g);
    const from = makeBotVillage(g, x, y);
    const to = makeBotVillage(g, x + 1, y);
    from.ownerId = activeVillage(g).ownerId;
    g.players[from.ownerId!]!.villageIds.push(from.id);
    g.commands.push({ id: 'own', type: 'support', originId: from.id, targetId: to.id, units: { spear: 30 }, departedAt: g.now, arrivesAt: g.now + HOUR });
    useGameStore.setState({ game: g });
    const army = armiesOnMap(g).find((a) => a.id === 'own')!;
    const { container } = render(<ArmyPreview army={army} seed={g.worldSeed} isVillage={() => false} />);
    const canvas = () => container.querySelector('canvas')!;
    expect(canvas().getAttribute('data-ground')).toBe('meadow');
    expect(canvas().getAttribute('data-boats')).toBe('0');
    advance(HOUR);
    expect(canvas().getAttribute('data-ground')).toBe('water');
    expect(Number(canvas().getAttribute('data-boats'))).toBeGreaterThan(0);
  });

  it('incoming attack preview shows the torch group', () => {
    const g = newTestGame();
    const home = activeVillage(g);
    const bot = makeBotVillage(g);
    g.commands.push({ id: 'inc', type: 'attack', originId: bot.id, targetId: home.id, units: { axe: 5000 }, departedAt: g.now - HOUR, arrivesAt: g.now + HOUR });
    useGameStore.setState({ game: g });
    const army = armiesOnMap(g).find((a) => a.id === 'inc')!;
    const { container } = render(<ArmyPreview army={army} seed={g.worldSeed} isVillage={() => false} />);
    const canvas = container.querySelector('canvas')!;
    expect(canvas.getAttribute('data-figures')).toBe('8');
    expect(canvas.width).toBe(240);
    expect(canvas.height).toBe(120);
  });
});
