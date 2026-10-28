import type { Timelapse } from './model';
import { frameAt } from './model';
import { effectMsFor, type Length } from './playback';
import { VIDEO_SIZE, drawVideoFrame, type VideoInfo } from './render';

/** MP4 first: phones' galleries and chat apps take it; WebM where the browser records nothing else. */
export const VIDEO_TYPES = ['video/mp4;codecs=avc1.42E01E', 'video/mp4', 'video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm'];

const FPS = 30;
const HOLD_FRAMES = FPS;
const BITS_PER_SECOND = 1_000_000;

export function pickVideoType(isSupported: (type: string) => boolean): string | null {
  return VIDEO_TYPES.find((type) => isSupported(type)) ?? null;
}

export function canRecord(): boolean {
  return typeof MediaRecorder !== 'undefined' && typeof HTMLCanvasElement !== 'undefined'
    && typeof HTMLCanvasElement.prototype.captureStream === 'function'
    && pickVideoType((type) => MediaRecorder.isTypeSupported(type)) !== null;
}

/**
 * The replay drawn into an offscreen square and recorded: `t` runs from the start to the end over `length` seconds and
 * the last frame stays a second. Frames come from a timer, not requestAnimationFrame, which stops in a background tab.
 */
export function recordTimelapse(
  tl: Timelapse, info: VideoInfo, legendNames: (owner: number) => string,
  opts: { length: Length; onProgress: (share: number) => void; signal?: AbortSignal },
): Promise<{ blob: Blob; ext: 'mp4' | 'webm' }> {
  const mimeType = pickVideoType((type) => MediaRecorder.isTypeSupported(type))!;
  const canvas = document.createElement('canvas');
  canvas.width = VIDEO_SIZE;
  canvas.height = VIDEO_SIZE;
  const ctx = canvas.getContext('2d');
  const recorder = new MediaRecorder(canvas.captureStream(FPS), { mimeType, videoBitsPerSecond: BITS_PER_SECOND });
  const chunks: Blob[] = [];
  const effectMs = effectMsFor(opts.length, tl.endAt);
  const frames = opts.length * FPS;
  return new Promise((resolve, reject) => {
    let i = 0;
    let timer: ReturnType<typeof setInterval> | undefined;
    const draw = () => {
      const t = Math.min(tl.endAt, (tl.endAt * i) / frames);
      if (ctx) drawVideoFrame(ctx, tl, frameAt(tl, t, effectMs), info, legendNames);
      opts.onProgress(Math.min(1, i / (frames + HOLD_FRAMES)));
    };
    const abort = () => {
      clearInterval(timer);
      recorder.onstop = null;
      if (recorder.state !== 'inactive') recorder.stop();
      reject(new DOMException('aborted', 'AbortError'));
    };
    if (opts.signal?.aborted) return abort();
    opts.signal?.addEventListener('abort', abort, { once: true });
    recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };
    recorder.onstop = () => {
      opts.signal?.removeEventListener('abort', abort);
      resolve({ blob: new Blob(chunks, { type: mimeType.split(';')[0] }), ext: mimeType.startsWith('video/mp4') ? 'mp4' : 'webm' });
    };
    draw();
    recorder.start();
    timer = setInterval(() => {
      i++;
      draw();
      if (i >= frames + HOLD_FRAMES) {
        clearInterval(timer);
        recorder.stop();
      }
    }, 1000 / FPS);
  });
}
