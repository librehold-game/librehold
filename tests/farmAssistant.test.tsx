// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { FarmAssistantView } from '../src/ui/views/FarmAssistantView';
import { localDayIndex } from '../src/engine/clock';
import { useGameStore } from '../src/state/gameStore';
import { setLocale } from '../src/i18n/i18n';
import { newDefaultGame } from './helpers';
import { stubServer, type StubRoute } from './serverStub';

async function boot(farm: StubRoute, edit: (g: ReturnType<typeof newDefaultGame>) => void = () => {}) {
  useGameStore.getState().shutdown();
  const g = newDefaultGame();
  edit(g);
  const key = `GET /api/v1/worlds/1/farm-targets?village=${encodeURIComponent(g.activeVillageId)}`;
  stubServer(g, { routes: { [key]: farm } });
  await useGameStore.getState().boot();
}

beforeEach(() => setLocale('en'));
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
  vi.useRealTimers();
  setLocale('pl');
});

describe('farm assistant', () => {
  it('says how many targets the filters hide and why', async () => {
    await boot(() => ({ status: 200, body: { targets: [], hidden: { recent: 3, inbound: 1 } } }));
    render(<FarmAssistantView />);
    expect(await screen.findByText(/Hidden by the filters: 3 attacked recently · 1 with an attack on the way/)).toBeTruthy();
  });

  it('a failed refresh says so instead of keeping an old list quietly', async () => {
    await boot(() => ({ status: 500, body: { error: { code: 'internal', message: 'boom' } } }));
    render(<FarmAssistantView />);
    expect(await screen.findByText(/could not be refreshed/)).toBeTruthy();
  });

  it("yesterday's counters read zero", async () => {
    await boot(() => ({ status: 200, body: { targets: [], hidden: {} } }), (g) => {
      const yesterday = localDayIndex(g, g.now) - 1;
      g.stats.attacksToday = { day: yesterday, count: 40 };
      g.stats.lootedToday = { day: yesterday, total: 9000 };
    });
    render(<FarmAssistantView />);
    const line = (await screen.findByText(/Sent today/)).closest('span')!;
    expect(line.textContent).toMatch(/Sent today: 0 attacks · Looted today: 0/);
  });
});
