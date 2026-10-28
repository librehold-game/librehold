// @vitest-environment happy-dom
import { existsSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { loadGameCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';
import type { SpectatorState, WatchVillage } from '../src/net/types';
import { armyAt, hitTest } from '../src/ui/watch/geometry';
import { GodView } from '../src/ui/watch/GodView';
import { useWatchStore } from '../src/ui/watch/watchStore';

const MIN = 60_000;
const villages: WatchVillage[] = [
  { id: 'v:6:6', x: 6, y: 6, ownerId: 'h1', points: 300 },
  { id: 'v:10:10', x: 10, y: 10, ownerId: 'bot_0', points: 900 },
  { id: 'v:12:8', x: 12, y: 8, ownerId: '', points: 50 },
];
function spectator(): SpectatorState {
  return {
    worldId: 7, worldName: 'Friday night', now: 10 * MIN, serverTime: Date.now(), paused: false,
    config: { mapSize: 20, worldSpeed: 100, unitSpeed: 100 } as SpectatorState['config'],
    hosted: { worldId: 7, name: 'Friday night', status: 'running', humans: 1, seats: 10, bots: 1, inviteCode: 'abcdefghijklmnopqrst',
      registeredOnly: false, pauseLeftMs: 30 * MIN, players: [{ id: 'h1', name: 'Ala', isGuest: true }], settings: {} as never },
    instant: { endsAt: 60 * MIN, joinUntil: 10 * MIN },
    players: {
      h1: { id: 'h1', name: 'Ala', color: '#d23b2f', status: 'active', points: 300, villageIds: ['v:6:6'], isBot: false },
      bot_0: { id: 'bot_0', name: 'Borko', color: '#2f6fd2', status: 'active', points: 900, villageIds: ['v:10:10'], isBot: true },
    },
    villages,
    commands: [
      { id: 'c_1', type: 'attack', originId: 'v:6:6', targetId: 'v:10:10', departedAt: 5 * MIN, arrivesAt: 15 * MIN, units: { axe: 10 } },
      { id: 'c_2', type: 'support', originId: 'v:10:10', targetId: 'v:12:8', departedAt: 8 * MIN, arrivesAt: 20 * MIN, units: { spear: 5 } },
    ] as SpectatorState['commands'],
    ranking: [
      { id: 'bot_0', name: 'Borko', color: '#2f6fd2', points: 900, villages: 1, att: 0, def: 0, sup: 0, quit: false, isBot: true },
      { id: 'h1', name: 'Ala', color: '#d23b2f', points: 300, villages: 1, att: 0, def: 0, sup: 0, quit: false, isBot: false },
    ],
  };
}

let calls: string[];

/** The player's order buttons, named as the map and the village name them. */
const ORDER_BUTTON = /^(send|attack|support|scout|build|recruit)$/i;

beforeEach(() => {
  loadGameCatalogs();
  setLocale('en');
  calls = [];
  history.replaceState(null, '', '/game/?world=7&watch=1');
  vi.stubGlobal('WebSocket', class { onmessage = null; onclose = null; close() {} });
  vi.stubGlobal('requestAnimationFrame', () => 0);
  vi.stubGlobal('cancelAnimationFrame', () => undefined);
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    calls.push(url);
    if (url === '/api/v1/worlds/7/watch') return new Response(JSON.stringify(spectator()), { status: 200 });
    if (url === '/api/v1/worlds/7/watch/villages/v%3A12%3A8' || url === '/api/v1/worlds/7/watch/villages/v:12:8') {
      return new Response(JSON.stringify({ id: 'v:12:8', name: '', x: 12, y: 8, ownerId: '', points: 50,
        buildings: { main: 2, wood: 3 }, unitsHome: { spear: 4 } }), { status: 200 });
    }
    return new Response(JSON.stringify({ error: { code: 'notFound' } }), { status: 404 });
  }));
});

afterEach(() => {
  cleanup();
  useWatchStore.getState().stop();
  vi.unstubAllGlobals();
  setLocale('pl');
});

async function open() {
  render(<GodView />);
  return screen.findByRole('heading', { name: 'Friday night', level: 1 });
}

describe('the god view', () => {
  it('shows the ranking with bot tags and every army in the list', async () => {
    await open();
    const ranking = screen.getByRole('table', { name: 'Ranking' });
    expect(within(ranking).getByText('Borko').parentElement!.querySelector('.bot-tag')).toBeTruthy();
    const armies = screen.getByRole('list', { name: 'Armies on the map' });
    const rows = within(armies).getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    expect(rows[0]!.textContent).toContain('Ala');
    expect(rows[0]!.textContent).toContain('Borko');
  });

  it('an army row opens its card with owner, target, arrival and units', async () => {
    await open();
    fireEvent.click(within(screen.getByRole('list', { name: 'Armies on the map' })).getAllByRole('button')[0]!);
    const card = await screen.findByRole('region', { name: 'Army' });
    expect(card.textContent).toContain('Ala');
    expect(card.textContent).toContain('Borko');
    expect(card.textContent).toContain('0:15:00');
    expect(card.textContent).toMatch(/10\s*Axemen/);
  });

  it('a map click on a village opens its read-only card', async () => {
    await open();
    const map = screen.getByRole('img', { name: 'The whole map' }) as HTMLCanvasElement;
    const cell = map.width / 20;
    fireEvent.click(map, { clientX: (12 + 0.5) * cell, clientY: (8 + 0.5) * cell });
    const card = await screen.findByRole('region', { name: 'Village' });
    await waitFor(() => expect(card.textContent).toContain('Barbarian village'));
    expect(card.textContent).toContain('50');
    expect(card.textContent).toMatch(/4\s*Spearmen/);
    expect(calls.some((c) => c.startsWith('/api/v1/worlds/7/watch/villages/'))).toBe(true);
  });

  it('shows no order forms or send buttons', async () => {
    await open();
    expect(screen.queryByRole('button', { name: ORDER_BUTTON })).toBeNull();
    expect(document.querySelector('form input[type="number"]')).toBeNull();
    expect(screen.queryByRole('link', { name: 'Open the god view' })).toBeNull();
  });

  it('the host can switch to the normal map and it shows every village and army', async () => {
    await open();
    fireEvent.click(screen.getByRole('button', { name: 'Normal map' }));
    const map = await screen.findByRole('region', { name: 'Normal map' });
    expect(map.querySelectorAll('[data-tile-village]')).toHaveLength(3);
    expect(map.querySelectorAll('[data-army]')).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', { name: 'Whole map' }));
    expect(screen.getByRole('img', { name: 'The whole map' })).toBeTruthy();
  });

  it('the normal map in the god view offers no Attack, Support or Send', async () => {
    await open();
    fireEvent.click(screen.getByRole('button', { name: 'Normal map' }));
    const map = await screen.findByRole('region', { name: 'Normal map' });
    fireEvent.click(map.querySelector('[data-tile-village="v:12:8"]')!);
    await screen.findByRole('region', { name: 'Village' });
    expect(screen.queryByRole('button', { name: ORDER_BUTTON })).toBeNull();
  });
});

describe('god view geometry', () => {
  it('armyAt moves along the route and stops at the target', () => {
    const c = { type: 'attack' as const, originId: 'v:0:0', targetId: 'v:10:0', departedAt: 0, arrivesAt: 1000 };
    expect(armyAt(c, 500)).toEqual({ x: 5.5, y: 0.5 });
    expect(armyAt(c, 5000)).toEqual({ x: 10.5, y: 0.5 });
    expect(armyAt({ ...c, originId: 'nowhere' }, 500)).toBeNull();
    expect(armyAt({ type: 'intercept' as const, originId: 'v:0:0', targetId: '', point: { x: 4, y: 0 }, departedAt: 0, arrivesAt: 1000 }, 1000))
      .toEqual({ x: 4, y: 0 });
  });

  it('hitTest prefers an army marker over the village under it', () => {
    const layout = { x: 0, y: 0, size: 200 };
    const cell = 200 / 20;
    const army = { id: 'c_1', x: 6.5, y: 6.5 };
    expect(hitTest(6.5 * cell + 3, 6.5 * cell, layout, 20, villages, [army])).toEqual({ kind: 'army', id: 'c_1' });
    expect(hitTest(6.5 * cell + 3, 6.5 * cell, layout, 20, villages, [])).toEqual({ kind: 'village', id: 'v:6:6' });
    expect(hitTest(0.5 * cell, 19.5 * cell, layout, 20, villages, [army])).toBeNull();
  });
});

describe('the god view bundle', () => {
  it('the god view never imports the player store', () => {
    const root = process.cwd();
    const seen = new Set<string>();
    const queue = [join(root, 'src/ui/watch/GodView.tsx')];
    const pattern = /\bimport\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]|\bimport\(\s*['"]([^'"]+)['"]\s*\)/g;
    while (queue.length > 0) {
      const file = queue.pop()!;
      if (seen.has(file)) continue;
      seen.add(file);
      if (!/\.(ts|tsx)$/.test(file)) continue;
      for (const m of readFileSync(file, 'utf8').matchAll(pattern)) {
        const spec = (m[1] ?? m[2])!;
        if (!spec.startsWith('.')) continue;
        const base = resolve(dirname(file), spec);
        const hit = [base, `${base}.ts`, `${base}.tsx`].find((c) => existsSync(c) && statSync(c).isFile());
        if (hit) queue.push(hit);
      }
    }
    const files = [...seen].map((f) => relative(root, f));
    expect(files).toContain('src/ui/watch/WatchTileMap.tsx');
    expect(files).not.toContain('src/state/gameStore.ts');
  });
});
