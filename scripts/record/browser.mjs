// Shared by the recording scripts: a headless Chrome driven by puppeteer-core, which is not a dependency of the
// project. PUPPETEER_CORE points at a local puppeteer-core (default: resolved from node_modules), CHROME at a Chrome
// or Chromium binary.
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const puppeteer = require(process.env.PUPPETEER_CORE ?? 'puppeteer-core');
const CHROME = process.env.CHROME;
if (!CHROME) throw new Error('Set CHROME to the path of a Chrome or Chromium binary.');

export function launch() {
  return puppeteer.launch({ executablePath: CHROME, headless: true, args: ['--hide-scrollbars', '--mute-audio'] });
}

/** The value of `--name` on the command line. */
export function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : fallback;
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** A fresh visitor (own cookies) who presses Play now and lands in the village view, tutorial skipped. */
export async function playAsGuest(browser, baseUrl, viewport) {
  const context = await browser.createBrowserContext();
  const page = await context.newPage();
  await page.setViewport(viewport);
  await page.setCookie({ name: 'lang', value: 'en', url: baseUrl });
  await page.goto(baseUrl, { waitUntil: 'networkidle0' });
  await page.locator('button.cta:not([disabled])').click();
  await page.waitForSelector('svg.scene[role=img]', { timeout: 30_000 });
  const skip = await page.waitForSelector('::-p-text(Skip tutorial)', { timeout: 5_000 }).catch(() => null);
  if (skip) {
    await skip.click();
    await page.waitForSelector('[role=dialog][aria-label="Tutorial"]', { hidden: true });
  }
  return page;
}

/** The world id and the home village's coordinates, read from the address and the header. */
export async function whereAmI(page) {
  const worldId = Number(new URL(page.url()).searchParams.get('world'));
  const title = await page.$eval('header.panel', (h) => h.textContent ?? '');
  const m = /\((\d+)\|(\d+)\)/.exec(title);
  if (!m) throw new Error(`no coordinates in the header: ${title}`);
  return { worldId, x: Number(m[1]), y: Number(m[2]) };
}

/** Villages around (x, y) within r, nearest first, from the map API. */
export async function villagesNear(page, worldId, x, y, r = 10) {
  const tiles = await page.evaluate(async (q) => {
    const res = await fetch(`/api/v1/worlds/${q.worldId}/map?x0=${q.x - q.r}&y0=${q.y - q.r}&x1=${q.x + q.r}&y1=${q.y + q.r}`);
    return res.json();
  }, { worldId, x, y, r });
  return tiles.filter((t) => t.x !== x || t.y !== y).sort((a, b) => Math.hypot(a.x - x, a.y - y) - Math.hypot(b.x - x, b.y - y));
}

export async function tab(page, name) {
  await page.locator(`::-p-xpath(//nav//button[contains(@class,'tab') and starts-with(normalize-space(),'${name}')])`).click();
  await sleep(400);
}

async function fill(page, selector, value) {
  const el = await page.waitForSelector(selector);
  await el.click({ count: 3 });
  await el.type(String(value));
}

/** Sends an attack (or support) from the Rally Point; units by English name. */
export async function send(page, x, y, units, kind = 'Attack') {
  await tab(page, 'Rally Point');
  await fill(page, 'input[placeholder="x"]', x);
  await fill(page, 'input[placeholder="y"]', y);
  for (const [name, n] of Object.entries(units)) {
    await fill(page, `::-p-xpath(//tr[td[1][normalize-space()='${name}']]//input[@type='number'])`, n);
  }
  await page.locator(kind === 'Attack' ? '::-p-text(⚔ Attack)' : '::-p-text(🛡 Support)').click();
  await page.waitForSelector('::-p-text(Sent a)', { timeout: 10_000 }).catch(async (e) => {
    if (process.env.DEBUG_DIR) await page.screenshot({ path: `${process.env.DEBUG_DIR}/send-fail.png` });
    throw e;
  });
}

/** Scrolls an element to the top of the window. */
export async function scrollTo(page, selector, offset = 0) {
  await page.$eval(selector, (el, off) => window.scrollTo(0, el.getBoundingClientRect().top + window.scrollY - off), offset);
  await sleep(200);
}
