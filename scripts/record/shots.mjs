// Screenshots of the real game for the front page's features and the guides. Two guests share one instant game, so an
// incoming attack, a field battle and the reports really happen. Usage: node scripts/record/shots.mjs --url <base>
import { mkdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { arg, launch, playAsGuest, scrollTo, send, sleep, tab, villagesNear, whereAmI } from './browser.mjs';

const BASE = arg('url', 'http://localhost:8080');
const OUT = join(import.meta.dirname, '..', '..', 'src', 'assets', 'pixel', 'account');
const WIDE = { width: 1200, height: 675 };
const MAP_SIZE = 40; // an instant game's map

/** Centres the map near (x, y), but far enough from the edges that no empty ground beyond the map shows. */
async function centre(page, x, y) {
  const keep = (v) => Math.round(Math.min(Math.max(v, 9), MAP_SIZE - 10));
  const goTo = await page.waitForSelector('input[placeholder="x|y"]');
  await goTo.click({ count: 3 });
  await goTo.type(`${keep(x)}|${keep(y)}`);
  await page.locator('::-p-text(Go)').click();
}
mkdirSync(join(OUT, 'guides'), { recursive: true });

const MAX_BYTES = 145_000;

/** A WebP at quality 80, or lower where the pixel art is too busy to fit in MAX_BYTES. */
async function shot(page, name, clip) {
  const path = join(OUT, `${name}.webp`);
  for (const quality of [80, 70, 60, 50]) {
    await page.screenshot({ path, type: 'webp', quality, clip });
    if (statSync(path).size < MAX_BYTES) return;
  }
  throw new Error(`${name} stays over ${MAX_BYTES} bytes`);
}
const guide = (name, page) => shot(page, `guides/${name}`);

/** Closes what would cover a screenshot: the keep-this-village offer and the toasts. */
async function tidy(page) {
  const notNow = await page.$('::-p-text(Not now)');
  if (notNow) await notNow.click();
  for (const toast of await page.$$('div.fixed.bottom-4.right-4 > div.panel')) await toast.click().catch(() => undefined);
  await sleep(300);
}

/** Waits for a report the predicate picks, then opens it in the Reports view. */
async function openReport(page, worldId, pick, timeoutMs = 300_000) {
  const end = Date.now() + timeoutMs;
  for (;;) {
    const list = await page.evaluate(async (wid) => (await fetch(`/api/v1/worlds/${wid}/reports?limit=50`)).json(), worldId);
    const i = list.findIndex(pick);
    if (i >= 0) {
      await tidy(page);
      await tab(page, 'Village');
      await tab(page, 'Reports');
      const back = await page.$('::-p-text(← List)');
      if (back) await back.click();
      await page.waitForFunction((n) => document.querySelectorAll('table.table-game tbody tr.cursor-pointer').length > n, { timeout: 10_000 }, i)
        .catch(async (e) => {
          const ui = await page.$$eval('table.table-game tbody tr.cursor-pointer', (r) => r.map((x) => x.textContent.slice(0, 60)));
          await page.screenshot({ path: `${process.env.DEBUG_DIR ?? '.'}/report-fail.png` });
          console.error('api', i, list.slice(0, 6).map((r) => `${r.type} ${r.target?.x}|${r.target?.y} ${r.conquered ?? ''}`), 'ui', ui.slice(0, 6));
          throw e;
        });
      const rows = await page.$$('table.table-game tbody tr.cursor-pointer');
      await rows[i].click();
      await sleep(500);
      await tidy(page);
      await scrollTo(page, 'main.panel', 8);
      return list[i];
    }
    if (Date.now() > end) throw new Error('the report never came');
    await sleep(1000);
  }
}

const browser = await launch();
try {
  const a = await playAsGuest(browser, BASE, WIDE);
  const home = await whereAmI(a);
  await guide('first-hour-start', a);

  const b = await playAsGuest(browser, BASE, WIDE);
  // Beginner protection keeps both guests out of each other's reach for ten minutes after joining.
  const unprotectedAt = Date.now() + 10.5 * 60_000;
  const enemy = await whereAmI(b);
  if (enemy.worldId !== home.worldId) throw new Error('the two guests play different games');

  // The village with its build queue.
  for (const id of ['wood', 'clay', 'iron']) await a.locator(`#building-row-${id} td:last-child button.btn`).click();
  await scrollTo(a, '::-p-xpath(//h3[starts-with(normalize-space(),"Build queue")])', 360);
  await shot(a, 'feature-village');

  await a.evaluate(() => window.scrollTo(0, 0));
  await a.locator('#building-row-academy td:first-child button').click();
  await scrollTo(a, '::-p-xpath(//h3[starts-with(normalize-space(),"Hetman coins")])', 120);
  await guide('nobles-academy', a);
  await tab(a, 'Assistant');
  await scrollTo(a, 'main.panel', 8);
  await guide('raiding-farm', a);

  // A raid and a conquest of the two nearest barbarians.
  const [raided, taken] = (await villagesNear(a, home.worldId, home.x, home.y)).filter((t) => !t.ownerId);
  await send(a, raided.x, raided.y, { Axeman: 40 });
  await send(a, taken.x, taken.y, { Axeman: 50, Hetman: 1 });
  const at = (v) => (r) => r.target?.x === v.x && r.target?.y === v.y;
  await openReport(a, home.worldId, (r) => r.type === 'attack' && at(raided)(r));
  await guide('raiding-report', a);
  await openReport(a, home.worldId, (r) => r.conquered && at(taken)(r));
  await guide('nobles-conquest', a);
  await tab(a, 'Map');
  await centre(a, (home.x + taken.x) / 2, (home.y + taken.y) / 2);
  await sleep(800);
  await tidy(a);
  await guide('first-hour-conquest', a);

  // Incoming attacks: the other guest's spearmen march on the first one's village.
  await sleep(Math.max(0, unprotectedAt - Date.now()));
  await send(b, home.x, home.y, { Spearman: 50 });
  await tab(a, 'Village');
  await a.waitForSelector('::-p-xpath(//h3[normalize-space()="Incoming attacks"])', { timeout: 30_000 });
  await tidy(a);
  await scrollTo(a, '::-p-xpath(//h3[normalize-space()="Incoming attacks"])', 200);
  await shot(a, 'feature-incoming');
  await tab(a, 'Rally Point');
  await scrollTo(a, '::-p-xpath(//h3[normalize-space()="Incoming attacks"])', 160);
  await guide('night-incoming', a);

  // The new village gets support, then the first guest's cavalry meets the attack in the field.
  await tab(a, 'Rally Point');
  for (const [sel, v] of [['input[placeholder="x"]', taken.x], ['input[placeholder="y"]', taken.y],
    ['::-p-xpath(//tr[td[1][normalize-space()="Spearman"]]//input[@type="number"])', 20]]) {
    const el = await a.waitForSelector(sel);
    await el.click({ count: 3 });
    await el.type(String(v));
  }
  await scrollTo(a, 'main.panel', 8);
  await guide('night-support', a);
  await a.locator('::-p-text(🛡 Support)').click();

  // After the conquest the active village is the new one; the attack goes for the first.
  await tab(a, 'Map');
  await centre(a, home.x, home.y);
  await a.locator('::-p-text(－)').click();
  await sleep(800);
  await tidy(a);
  const marker = 'svg[data-army-layer] g[data-army][data-kind="incomingAttack"] [data-marker]';
  await a.waitForSelector(marker, { timeout: 15_000 }).catch(async (e) => {
    if (process.env.DEBUG_DIR) await a.screenshot({ path: `${process.env.DEBUG_DIR}/marker-fail.png` });
    console.error('armies', await a.$$eval('svg[data-army-layer] g[data-army]', (g) => g.map((x) => x.dataset.kind)));
    throw e;
  });
  // The attack may still be off the visible map; its card opens all the same.
  await a.$eval(marker, (el) => el.dispatchEvent(new MouseEvent('click', { bubbles: true })));
  await a.waitForSelector('[role=dialog][data-army-card]');
  await scrollTo(a, 'main.panel', 8);
  await guide('scouting-army-card', a);
  await a.locator('::-p-text(Attack in the field)').click();
  const cav = await a.waitForSelector('[data-army-card] input[aria-label="Light cavalryman"]');
  await cav.click({ count: 3 });
  // A handful, so the spearmen survive the meeting and still reach the village.
  await cav.type('5');
  await a.locator('[data-army-card] ::-p-text(Send)').click();
  await openReport(a, home.worldId, (r) => r.type === 'field' && !r.field?.missed);
  await guide('scouting-field-report', a);

  // The report: whatever is left of the attack reaches the village.
  await openReport(a, home.worldId, (r) => r.type === 'attack' && at(home)(r));
  await shot(a, 'feature-reports');

  // The army, on a phone: the rally point.
  const phone = await b.browserContext().newPage(); // the second guest still has one village, with its army
  await phone.setViewport({ width: 390, height: 844, isMobile: true, hasTouch: true, deviceScaleFactor: 1 });
  await phone.goto(`${BASE}/game/?world=${home.worldId}`, { waitUntil: 'networkidle0' });
  await phone.waitForSelector('svg.scene[role=img]');
  await phone.locator('nav[aria-label="Navigation"] .navbar-bottom button.navbtn ::-p-text(Rally)').click();
  await sleep(600);
  await tidy(phone);
  await shot(phone, 'feature-army', { x: 0, y: 0, width: 390, height: 690 });
} finally {
  await browser.close();
}
