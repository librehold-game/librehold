import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = join(__dirname, '..');

describe('own identity', () => {
  it('the game is called LibreHold', () => {
    expect(readFileSync(join(ROOT, 'game.html'), 'utf8')).toMatch(/<title>LibreHold<\/title>/);
    expect(JSON.parse(readFileSync(join(ROOT, 'package.json'), 'utf8')).name).toBe('librehold');
  });
});
