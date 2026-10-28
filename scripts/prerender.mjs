// Writes the account site's public pages as ready-made HTML, plus the files the server's gate reads.
// Runs after both account builds: the client build gives the shell, the SSR build gives render().
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist', 'account');
const ssr = await import(pathToFileURL(join(root, 'dist', 'account-ssr', 'entry-server.js')).href);
const template = readFileSync(join(dist, 'account.html'), 'utf8');

const fileFor = (path) => (path === '/' ? 'index.html' : path === '/pl/' ? 'pl/index.html' : `${path.slice(1)}/index.html`);

function write(file, content) {
  mkdirSync(dirname(join(dist, file)), { recursive: true });
  writeFileSync(join(dist, file), content);
}

function page(url, path, locale) {
  const { html, head, hoisted } = ssr.render(url);
  const content = template
    .replace('__LANG__', () => locale)
    .replace('<!--dynamic-head-->', () => hoisted + ssr.headTags(head, path))
    .replace('<div id="root"></div>', () => `<div id="root">${html}</div>`);
  return { head, content };
}

const written = new Map();
for (const url of ssr.prerenderUrls()) {
  const m = ssr.matchPath(url);
  const { head, content } = page(url, url, m.locale);
  if (head.notFound) continue;
  write(fileFor(url), content);
  written.set(url, { route: m.id, locale: m.locale, head });
}
write('404.html', page('/404', '/404', 'en').content);
write('pl/404.html', page('/pl/404', '/pl/404', 'pl').content);

const pairs = [];
for (const [url, info] of written) {
  if (info.locale !== 'en') continue;
  const twin = ssr.twinPath(url);
  if (written.has(twin)) pairs.push({ route: info.route, en: url, pl: twin });
}
write('routes.json', JSON.stringify(ssr.ROUTES, null, 2) + '\n');
write('pages.json', JSON.stringify(pairs, null, 2) + '\n');
write('head-text.json', JSON.stringify(ssr.headText(), null, 2) + '\n');

const problems = [];
for (const locale of ['en', 'pl']) {
  const seen = { title: new Map(), description: new Map() };
  for (const [url, info] of written) {
    if (info.locale !== locale) continue;
    if (!info.head.title) problems.push(`${url}: empty title`);
    for (const field of ['title', 'description']) {
      const other = seen[field].get(info.head[field]);
      if (other) problems.push(`${url} and ${other} share a ${field}`);
      seen[field].set(info.head[field], url);
    }
  }
}
if (problems.length > 0) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log(`prerendered ${written.size} pages, ${pairs.length} language pairs`);
