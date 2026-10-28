// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ArchivedGame } from '../src/account/api';
import { GamePage } from '../src/account/GamePage';
import { buildTimelapse, frameAt } from '../src/account/timelapse/model';
import { drawFrame } from '../src/account/timelapse/render';
import { TimelapsePlayer } from '../src/account/timelapse/TimelapsePlayer';
import type { TimelineFile } from '../src/account/timelapse/types';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';
import type { InstantView } from '../src/net/types';
import { useGameStore } from '../src/state/gameStore';
import { App as GameApp } from '../src/ui/App';
import { newTestGame } from './helpers';
import { stubServer } from './serverStub';

const MIN = 60_000;
const END = 60 * MIN;
const colors = ['#aa0000', '#00aa00', '#0000aa', '#aaaa00', '#00aaaa', '#aa00aa', '#555555', '#999999', '#222222', '#dddddd'];
const owners = colors.map((color, i) => ({ playerId: i === 1 ? 'bot_1' : `h${i}`, color, ...(i === 1 ? { bot: true } : {}) }));
const file: TimelineFile = {
  v: 1, mapSize: 40, endAt: END, owners, barbarians: [[1, 1], [2, 2]],
  events: [
    ...owners.map((_, i) => [0, 0, 5 + i, 5, i + 1]),
    [0, 0, 5, 9, 2],
    [10 * MIN, 1, 2, 2, 1],
    [20 * MIN, 2, 5, 5, 6, 5, 1, 2, 3],
  ],
};
const game: ArchivedGame = {
  worldId: 41, name: 'Game #41', gameNumber: 41, startedAt: 0, endedAt: END, winnerName: 'Ala', winnerIsBot: false,
  summary: { endReason: 'time', winnerId: 'h0', humans: 9, bots: 1, conquestsHumans: 1, conquestsBots: 0, attacksSent: 3,
    humanUnitsKilled: 10, humanUnitsLost: 5, largestBattleUnits: 300 },
  rows: owners.slice(0, 9).map((o, i) => ({ rank: i + 1, playerId: o.playerId, name: i === 1 ? 'Wolf' : i === 2 ? '' : `Player ${i}`,
    isBot: i === 1, points: 100 - i, villages: 1, conquered: 0, killed: 0 })),
} as ArchivedGame;

let frames: FrameRequestCallback[];
let now: number;
let calls: string[];

function runFrames(ms: number, step = 100) {
  for (let done = 0; done < ms; done += step) {
    now += step;
    const queue = frames;
    frames = [];
    act(() => queue.forEach((f) => f(now)));
  }
}

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  frames = [];
  now = 0;
  calls = [];
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => { frames.push(cb); return frames.length; });
  vi.stubGlobal('cancelAnimationFrame', () => undefined);
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} }));
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    calls.push(url);
    if (url === '/api/v1/public/games/41') return new Response(JSON.stringify(game), { status: 200 });
    if (url === '/api/v1/public/games/41/timeline') return new Response(JSON.stringify(file), { status: 200 });
    return new Response(JSON.stringify({ error: { code: 'notFound' } }), { status: 404 });
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const clock = () => screen.getByRole('timer').textContent!;

describe('timelapse player', () => {
  it('GamePage loads the timelapse only when Watch the map is pressed', async () => {
    render(<GamePage gameRef="41" />);
    const watch = await screen.findByRole('button', { name: 'Watch the map' });
    expect(calls).not.toContain('/api/v1/public/games/41/timeline');
    fireEvent.click(watch);
    expect(await screen.findByRole('img', { name: 'Map of the game' })).toBeTruthy();
    expect(calls).toContain('/api/v1/public/games/41/timeline');
  });

  it('play, pause, length and scrubber move the clock', async () => {
    render(<TimelapsePlayer gameRef="41" game={game} />);
    const pause = await screen.findByRole('button', { name: 'Pause' });
    fireEvent.click(pause);
    fireEvent.change(screen.getByRole('slider', { name: 'Game time' }), { target: { value: String(END / 2) } });
    expect(clock()).toContain('0:30:00');
    fireEvent.change(screen.getByRole('combobox', { name: 'Length' }), { target: { value: '30' } });
    fireEvent.click(screen.getByRole('button', { name: 'Play' }));
    runFrames(15_100);
    expect(clock()).toContain('1:00:00');
    expect(screen.getByRole('button', { name: 'Play' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Restart' }));
    expect(clock()).toContain('0:00:00');
  });

  it('opens paused with prefers-reduced-motion', async () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {} }));
    render(<TimelapsePlayer gameRef="41" game={game} />);
    expect(await screen.findByRole('button', { name: 'Play' })).toBeTruthy();
    runFrames(5000);
    expect(clock()).toContain('0:00:00');
  });

  it('the legend names the top eight with bot tags and former players', async () => {
    render(<TimelapsePlayer gameRef="41" game={game} />);
    const legend = await screen.findByRole('list', { name: 'Players on the map' });
    const items = within(legend).getAllByRole('listitem');
    expect(items).toHaveLength(8);
    expect(items[0]!.textContent).toContain('Wolf');
    expect(items[0]!.querySelector('.bot-tag')).toBeTruthy();
    expect(legend.textContent).toMatch(/former player/i);
  });

  it('drawFrame draws one square per village and one line per active battle', () => {
    const ops: string[] = [];
    const ctx = new Proxy({}, {
      get: (_, k: string) => (k in { fillRect: 1, strokeRect: 1, beginPath: 1, moveTo: 1, lineTo: 1, stroke: 1, arc: 1, drawImage: 1, clearRect: 1, fill: 1 }
        ? (...a: unknown[]) => { ops.push(k); return a; } : undefined),
      set: () => true,
    }) as unknown as CanvasRenderingContext2D;
    const tl = buildTimelapse(file, game.rows);
    const frame = frameAt(tl, 20 * MIN, MIN);
    drawFrame(ctx, tl, frame, { x: 0, y: 0, size: 400 }, null);
    expect(ops.filter((o) => o === 'fillRect')).toHaveLength(frame.villages.length);
    expect(ops.filter((o) => o === 'stroke')).toHaveLength(frame.lines.length + frame.flashes.length);
    expect(frame.lines).toHaveLength(1);
  });

  it('the end screen offers Watch the map', async () => {
    const g = newTestGame();
    const ended: InstantView = { endsAt: g.now, joinUntil: g.now, ended: { reason: 'time', at: g.now, winnerId: 'h0', winnerName: 'Ala', winnerIsBot: false } };
    stubServer(g, { instant: ended, routes: { 'GET /api/v1/public/games/1': () => ({ status: 200, body: { ...game, worldId: 1 } }) } });
    render(<GameApp />);
    const results = await screen.findByRole('dialog', { name: 'The game is over' });
    await waitFor(() => expect(within(results).getByRole('link', { name: 'Watch the map' }).getAttribute('href')).toBe('/games/1#watch'));
    useGameStore.getState().shutdown();
  });
});
