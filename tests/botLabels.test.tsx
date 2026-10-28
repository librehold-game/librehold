// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import type { GameState } from '../src/engine/types';
import { setLocale } from '../src/i18n/i18n';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { PlayerProfile } from '../src/ui/components/PlayerProfile';
import { RankingView } from '../src/ui/views/RankingView';
import { botVillage, newTestGame } from './helpers';
import { stubServer } from './serverStub';

let world: GameState;

beforeEach(async () => {
  setLocale('en');
  world = newTestGame('test-bot');
  botVillage(world);
  for (const i of [1, 2]) world.players[`bot_${i}`] = { ...structuredClone(world.players.bot_test!), id: `bot_${i}`, name: `Bot ${i}`, villageIds: [] };
  stubServer(world);
  await useGameStore.getState().boot();
  useUiStore.setState({ view: 'ranking' });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  setLocale('pl');
});

describe('bots are labelled as bots', () => {
  it('the ranking tags every bot and no human', async () => {
    const { container } = render(<RankingView />);
    await screen.findAllByText('bot');
    const bots = Object.values(world.players).filter((p) => !p.isHuman);
    expect(container.querySelectorAll('.bot-tag')).toHaveLength(bots.length);
  });

  it('a bot profile carries the tag; the own one does not', async () => {
    const bot = Object.values(world.players).find((p) => !p.isHuman)!;
    const { unmount } = render(<PlayerProfile playerId={bot.id} onClose={() => {}} />);
    expect(await screen.findByRole('dialog')).toBeTruthy();
    expect(document.querySelector('.bot-tag')).not.toBeNull();
    unmount();
    render(<PlayerProfile playerId="human" onClose={() => {}} />);
    await screen.findByRole('dialog');
    expect(document.querySelector('.bot-tag')).toBeNull();
  });
});
