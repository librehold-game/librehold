import { existsSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const DIR = join(__dirname, '..', 'src', 'assets', 'pixel', 'account');
const head = (file: string, n: number) => readFileSync(file).subarray(0, n);

function expectWebp(file: string, maxBytes: number) {
  expect(existsSync(file), file).toBe(true);
  expect(statSync(file).size, file).toBeLessThan(maxBytes);
  const h = head(file, 12);
  expect(h.subarray(0, 4).toString('ascii'), file).toBe('RIFF');
  expect(h.subarray(8, 12).toString('ascii'), file).toBe('WEBP');
}

describe('site media', () => {
  it('the clip is real video under a megabyte', () => {
    const mp4 = join(DIR, 'clip.mp4');
    const webm = join(DIR, 'clip.webm');
    for (const f of [mp4, webm]) {
      expect(existsSync(f), f).toBe(true);
      expect(statSync(f).size, f).toBeLessThan(1_000_000);
    }
    expect(head(mp4, 8).subarray(4, 8).toString('ascii')).toBe('ftyp');
    expect([...head(webm, 4)]).toEqual([0x1a, 0x45, 0xdf, 0xa3]);
  });

  it('poster and feature shots are small WebP', () => {
    for (const name of ['clip-poster', 'clip-poster-480', 'feature-village', 'feature-army', 'feature-reports', 'feature-incoming']) {
      expectWebp(join(DIR, `${name}.webp`), 150_000);
    }
  });

  it('every guide shot listed in shots.mjs exists', () => {
    const script = readFileSync(join(__dirname, '..', 'scripts', 'record', 'shots.mjs'), 'utf8');
    const listed = [...script.matchAll(/guide\(\s*'([a-z-]+)'/g)].map((m) => m[1]!);
    expect(listed.length).toBe(10);
    for (const name of listed) expectWebp(join(DIR, 'guides', `${name}.webp`), 150_000);
  });
});
