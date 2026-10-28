// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@testing-library/react';
import { HUMAN_ID, type GameState } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { PlayerProfile } from '../src/ui/components/PlayerProfile';
import { RankingView } from '../src/ui/views/RankingView';
import { botVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

/** The human, the test bot with its village and three more bots without villages. */
function rankedWorld(): GameState {
  const g = newTestGame('test-bot');
  const v = botVillage(g);
  v.units.axe = 9876; v.units.lightCav = 4321;
  for (const i of [1, 2, 3]) g.players[`bot_${i}`] = { ...structuredClone(g.players.bot_test!), id: `bot_${i}`, name: `Bot ${i}`, villageIds: [] };
  const pts = [400, 1200, 50, 800, 3000];
  Object.values(g.players).forEach((p, i) => { p.points = pts[i]!; });
  return g;
}

let world: GameState;
let calls: string[];

beforeEach(async () => {
  world = rankedWorld();
  calls = stubServer(world, { me: 'h3' });
  await useGameStore.getState().boot();
  useUiStore.setState({ view: 'ranking' });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe('rankings and profiles from the server', () => {
  it('points ranking: order and the highlighted player', async () => {
    const { container } = render(<RankingView />);
    await screen.findByText(world.players[HUMAN_ID]!.name);
    const rows = [...container.querySelectorAll('tbody tr')];
    const points = rows.map((r) => Number(r.querySelector('[data-label="Punkty"]')!.textContent!.replace(/\D/g, '')));
    expect(points).toEqual([...points].sort((a, b) => b - a));
    expect(rows.length).toBe(5);
    expect(container.querySelector('tr.ranking-me')!.textContent).toContain(world.players[HUMAN_ID]!.name);
    expect(calls).toContain('GET /api/v1/worlds/1/ranking?tab=points');
  });

  it('the ranking refreshes after the ranking changes on the server', async () => {
    render(<RankingView />);
    await screen.findByText(world.players[HUMAN_ID]!.name);
    const before = calls.length;
    useGameStore.setState({ rankingRev: useGameStore.getState().rankingRev + 1 });
    await vi.waitFor(() => expect(calls.length).toBeGreaterThan(before));
  });

  it('the profile shows only public data fetched from the server', async () => {
    const bot = Object.values(world.players).find((p) => !p.isHuman)!;
    const v = world.villages[bot.villageIds[0]!]!;
    render(<PlayerProfile playerId={bot.id} onClose={() => {}} />);
    await screen.findByText(`${v.x}|${v.y}`, { exact: false });
    const text = document.body.textContent ?? '';
    expect(text).toContain(bot.name);
    expect(calls).toContain(`GET /api/v1/worlds/1/players/${bot.id}`);
    // Bot nicknames contain unit names, so we check troop counts, not names.
    for (const n of ['9876', '9 876', '4321', '4 321']) expect(text).not.toContain(n);
  });

  it('your own profile asks the server for your own id', async () => {
    render(<PlayerProfile playerId={HUMAN_ID} onClose={() => {}} />);
    await screen.findByText(/\(ty\)/);
    expect(calls).toContain('GET /api/v1/worlds/1/players/h3');
  });

  it('every ranking table cell has a data-label', async () => {
    const { container } = render(<RankingView />);
    await screen.findByText(world.players[HUMAN_ID]!.name);
    for (const tab of ['Punkty', 'Wioski', 'Pokonani', 'Dzienne', 'Przejęcia']) {
      fireEvent.click(screen.getByRole('button', { name: tab }));
      for (const td of container.querySelectorAll('td')) expect(td.hasAttribute('data-label') || td.classList.contains('stack-inline')).toBe(true);
    }
  });
});
