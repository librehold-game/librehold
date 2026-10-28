// Writes brotli and gzip copies of the builds' text assets, so the server sends them compressed without
// compressing on every request. Usage: node scripts/compress.mjs <dist dir>...
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { brotliCompressSync, constants, gzipSync } from 'node:zlib';

const TEXT = /\.(js|css|svg|json|txt|html)$/;
const MIN_BYTES = 1024;

function files(dir) {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    return statSync(full).isDirectory() ? files(full) : [full];
  });
}

/** Compresses every large text file under `<dir>/assets/`; returns the paths written. */
export function compressDir(dir) {
  const assets = join(dir, 'assets');
  let all;
  try {
    all = files(assets);
  } catch {
    return [];
  }
  const written = [];
  for (const file of all.filter((f) => TEXT.test(f))) {
    const data = readFileSync(file);
    if (data.length < MIN_BYTES) continue;
    const copies = [
      [`${file}.br`, brotliCompressSync(data, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } })],
      [`${file}.gz`, gzipSync(data, { level: 9 })],
    ];
    for (const [path, packed] of copies) {
      if (packed.length >= data.length) continue;
      writeFileSync(path, packed);
      written.push(path);
    }
  }
  return written;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  for (const dir of process.argv.slice(2)) console.log(`${dir}: ${compressDir(dir).length} compressed copies`);
}
