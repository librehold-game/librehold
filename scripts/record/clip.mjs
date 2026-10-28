// Records the front page's clip from a running game: a build finishing in the village, then an army marching on
// the map. Usage: node scripts/record/clip.mjs --url http://localhost:8080
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { arg, launch, playAsGuest, scrollTo, send, sleep, tab, villagesNear, whereAmI } from './browser.mjs';

const BASE = arg('url', 'http://localhost:8080');
const OUT = join(import.meta.dirname, '..', '..', 'src', 'assets', 'pixel', 'account');
const FFMPEG = process.env.FFMPEG ?? '/opt/homebrew/bin/ffmpeg';
const FFPROBE = process.env.FFPROBE ?? FFMPEG.replace(/ffmpeg$/, 'ffprobe');
const FPS = 12;
const frames = [];

/** Screenshots as fast as the browser gives them, timed so each scene starts where the last one ended; with clip,
 * that part of the page scaled to the clip's width. */
async function capture(page, ms, clip) {
  const begin = Date.now();
  const offset = frames.length ? frames.at(-1).at + 1000 / FPS - begin : 0;
  while (Date.now() < begin + ms) {
    const at = Date.now() + offset;
    frames.push({ at, png: await page.screenshot({ type: 'png', clip }) });
  }
}

const browser = await launch();
try {
  const page = await playAsGuest(browser, BASE, { width: 960, height: 540 });
  const me = await whereAmI(page);
  const target = (await villagesNear(page, me.worldId, me.x, me.y)).find((t) => !t.ownerId);
  if (!target) throw new Error('no barbarian village nearby');

  await page.locator('#building-row-wood td:last-child button.btn').click();
  await scrollTo(page, 'svg.scene', 8);
  await capture(page, 4000);

  await send(page, target.x, target.y, { Axeman: 60, 'Light cavalryman': 10 });
  // The map gets a taller window and fills the frame: a 16:9 piece of it, scaled to 960 pixels.
  await page.setViewport({ width: 960, height: 1000 });
  await tab(page, 'Map');
  for (let i = 0; i < 2; i++) await page.locator('::-p-text(＋)').click();
  await page.locator('::-p-text(My village)').click();
  await scrollTo(page, '[data-map-surface]', 0);
  await sleep(800);
  await page.waitForSelector('svg[data-army-layer] g[data-army]');
  const box = await page.$eval('[data-map-surface]', (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y + window.scrollY, width: r.width, height: r.height };
  });
  const height = Math.min(box.height, (box.width * 9) / 16);
  const width = (height * 16) / 9;
  await capture(page, 6000, { x: box.x + (box.width - width) / 2, y: box.y + (box.height - height) / 2, width, height,
    scale: 960 / width });
} finally {
  await browser.close();
}

// Resample to a steady frame rate: each output frame is the latest screenshot taken by its time.
const dir = mkdtempSync(join(process.env.TMPDIR ?? tmpdir(), 'clip-'));
const start = frames[0].at;
const total = Math.floor(((frames.at(-1).at - start) / 1000) * FPS);
for (let i = 0, j = 0; i <= total; i++) {
  while (j + 1 < frames.length && frames[j + 1].at - start <= (i * 1000) / FPS) j++;
  writeFileSync(join(dir, `${String(i).padStart(4, '0')}.png`), frames[j].png);
}
const input = ['-y', '-framerate', String(FPS), '-i', join(dir, '%04d.png')];
const ff = (...args) => execFileSync(FFMPEG, args, { stdio: 'inherit' });
ff(...input, '-vf', 'scale=960:-2:flags=neighbor,format=yuv420p', '-c:v', 'libx264', '-profile:v', 'main', '-crf', '30',
  '-movflags', '+faststart', '-an', join(OUT, 'clip.mp4'));
ff(...input, '-vf', 'format=yuv420p', '-c:v', 'libvpx-vp9', '-b:v', '0', '-crf', '40', '-an', join(OUT, 'clip.webm'));
ff('-y', '-i', join(dir, '0000.png'), '-frames:v', '1', '-c:v', 'libwebp', '-quality', '70', join(OUT, 'clip-poster.webp'));
// Phones get a half-size poster: it is the front page's largest paint.
ff('-y', '-i', join(dir, '0000.png'), '-frames:v', '1', '-vf', 'scale=480:270:flags=neighbor', '-c:v', 'libwebp', '-quality', '80',
  join(OUT, 'clip-poster-480.webp'));
rmSync(dir, { recursive: true });

let ok = true;
for (const name of ['clip.mp4', 'clip.webm']) {
  const file = join(OUT, name);
  const seconds = Number(execFileSync(FFPROBE, ['-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', file]).toString());
  const bytes = statSync(file).size;
  console.log(`${name}: ${seconds.toFixed(1)} s, ${bytes} bytes`);
  if (seconds < 8 || seconds > 12 || bytes >= 1_000_000) ok = false;
}
process.exit(ok ? 0 : 1);
