// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import type { ArchivedGame } from '../src/account/api';
import { pickVideoType } from '../src/account/timelapse/record';
import { TimelapsePlayer } from '../src/account/timelapse/TimelapsePlayer';
import type { TimelineFile } from '../src/account/timelapse/types';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

const MIN = 60_000;
const file: TimelineFile = {
  v: 1, mapSize: 20, endAt: 60 * MIN, owners: [{ playerId: 'h0', color: '#aa0000' }], barbarians: [[1, 1]],
  events: [[0, 0, 5, 5, 1], [30 * MIN, 1, 1, 1, 1]],
};
const game = {
  worldId: 41, name: 'Game #41', gameNumber: 41, startedAt: 0, endedAt: 60 * MIN, winnerName: 'Ala', winnerIsBot: false,
  summary: { endReason: 'time', winnerId: 'h0', humans: 1, guests: 0, bots: 0, conquestsHumans: 1, conquestsBots: 0, attacksSent: 1,
    humanUnitsKilled: 0, humanUnitsLost: 0, largestBattleUnits: 0 },
  rows: [{ rank: 1, playerId: 'h0', name: 'Ala', isBot: false, points: 10, villages: 2, conquered: 1, killed: 0 }],
} as ArchivedGame;

let recorders: { options: MediaRecorderOptions; started: boolean }[];
let canvases: { width: number; height: number; fps: number }[];

class FakeRecorder {
  static supported: (type: string) => boolean = (type) => type === 'video/webm;codecs=vp9';
  static isTypeSupported(type: string) { return FakeRecorder.supported(type); }
  ondataavailable: ((e: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  state = 'inactive';
  constructor(_stream: unknown, readonly options: MediaRecorderOptions) { recorders.push(this as never); }
  start() { this.state = 'recording'; (this as unknown as { started: boolean }).started = true; }
  stop() {
    this.state = 'inactive';
    this.ondataavailable?.({ data: new Blob(['clip'], { type: this.options.mimeType }) });
    this.onstop?.();
  }
}

function withRecorder() {
  vi.stubGlobal('MediaRecorder', FakeRecorder);
  Object.defineProperty(HTMLCanvasElement.prototype, 'captureStream', {
    configurable: true,
    value(this: HTMLCanvasElement, fps: number) { canvases.push({ width: this.width, height: this.height, fps }); return {}; },
  });
}

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  recorders = [];
  canvases = [];
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {} }));
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(file), { status: 200 })));
  URL.createObjectURL = vi.fn(() => 'blob:clip');
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  delete (HTMLCanvasElement.prototype as { captureStream?: unknown }).captureStream;
  setLocale('pl');
});

async function recordClip() {
  render(<TimelapsePlayer gameRef="41" game={game} />);
  const download = await screen.findByRole('button', { name: 'Download video' });
  vi.useFakeTimers();
  fireEvent.click(download);
  expect(screen.getByRole('status').textContent).toContain('Recording');
  const seen: string[] = [];
  for (let ms = 0; ms < 32_000; ms += 100) {
    await act(async () => { vi.advanceTimersByTime(100); });
    const status = screen.queryByRole('status');
    if (status) seen.push(status.textContent!);
  }
  expect(seen.at(-1)).toContain('100 %');
  vi.useRealTimers();
}

describe('timelapse video', () => {
  it('pickVideoType prefers MP4, then WebM, else null', () => {
    expect(pickVideoType(() => true)).toBe('video/mp4;codecs=avc1.42E01E');
    expect(pickVideoType((t) => t.startsWith('video/webm'))).toBe('video/webm;codecs=vp9');
    expect(pickVideoType((t) => t === 'video/webm')).toBe('video/webm');
    expect(pickVideoType(() => false)).toBeNull();
  });

  it('no MediaRecorder: no download button, the player still works', async () => {
    render(<TimelapsePlayer gameRef="41" game={game} />);
    expect(await screen.findByRole('button', { name: 'Play' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Download video' })).toBeNull();
  });

  it('a fake MediaRecorder and captureStream: progress reaches 100 and a save link with the mp4 or webm name appears', async () => {
    withRecorder();
    await recordClip();
    const save = await screen.findByRole('link', { name: 'Save video' });
    expect(save.getAttribute('download')).toBe('game-41-timelapse.webm');
    expect(save.getAttribute('href')).toBe('blob:clip');
    FakeRecorder.supported = (type) => type === 'video/mp4';
    cleanup();
    await recordClip();
    expect((await screen.findByRole('link', { name: 'Save video' })).getAttribute('download')).toBe('game-41-timelapse.mp4');
    cleanup();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:clip');
    FakeRecorder.supported = (type) => type === 'video/webm;codecs=vp9';
  });

  it('the recording asks for 1080×1080 and 1 Mbit/s', async () => {
    withRecorder();
    await recordClip();
    expect(canvases).toEqual([{ width: 1080, height: 1080, fps: 30 }]);
    expect(recorders).toHaveLength(1);
    expect(recorders[0]!.options).toEqual({ mimeType: 'video/webm;codecs=vp9', videoBitsPerSecond: 1_000_000 });
    expect(recorders[0]!.started).toBe(true);
  });
});
