import { mkdirSync, mkdtempSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { brotliDecompressSync, gunzipSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';
// @ts-expect-error a plain Node script without types
import { compressDir } from '../scripts/compress.mjs';

describe('pre-compressed assets', () => {
  it('compressDir writes brotli and gzip copies of large text assets only', () => {
    const dir = mkdtempSync(join(tmpdir(), 'compress-'));
    mkdirSync(join(dir, 'assets'));
    const js = Array.from({ length: 200 }, (_, i) => `console.log("line ${i}");`).join('\n');
    expect(js.length).toBeGreaterThan(4000);
    writeFileSync(join(dir, 'assets', 'big.js'), js);
    writeFileSync(join(dir, 'assets', 'small.css'), 'a{color:red}'.repeat(8));
    writeFileSync(join(dir, 'assets', 'pic.png'), Buffer.alloc(5000, 1));
    writeFileSync(join(dir, 'index.html'), js);
    const written: string[] = compressDir(dir);
    expect(written.sort()).toEqual([join(dir, 'assets', 'big.js.br'), join(dir, 'assets', 'big.js.gz')]);
    expect(brotliDecompressSync(readFileSync(join(dir, 'assets', 'big.js.br'))).toString()).toBe(js);
    expect(gunzipSync(readFileSync(join(dir, 'assets', 'big.js.gz'))).toString()).toBe(js);
    expect(existsSync(join(dir, 'assets', 'small.css.br'))).toBe(false);
    expect(existsSync(join(dir, 'assets', 'pic.png.br'))).toBe(false);
  });
});
