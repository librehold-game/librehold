import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

// The server is the only rules engine: the browser keeps display helpers in src/engine and reads its tables from
// the generated src/data/balance.json. This pins that surface so the simulation does not creep back.

function filesUnder(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    return statSync(path).isDirectory() ? filesUnder(path) : [path];
  });
}

describe('the browser keeps no simulation', () => {
  it('src/engine holds only the display helpers', () => {
    expect(readdirSync('src/engine').sort()).toEqual([
      'blockers.ts', 'buildings.ts', 'clock.ts', 'commands.ts', 'conquest.ts', 'farm.ts', 'fieldGeometry.ts', 'incoming.ts',
      'market.ts', 'modifiers.ts', 'players.ts', 'quests.ts', 'queues.ts', 'reports.ts', 'research.ts', 'resources.ts', 'rng.ts', 'types.ts',
      'units.ts', 'world.ts',
    ]);
  });

  it('src/data holds the generated tables and their typed readers', () => {
    expect(readdirSync('src/data').sort()).toEqual([
      'balance.json', 'buildings.data.ts', 'conquest.data.ts', 'quests.data.ts', 'research.data.ts', 'units.data.ts', 'world.config.ts',
    ]);
  });

  it('no source file mentions the bot or playbook tables', () => {
    const offenders = filesUnder('src').filter((f) => /\.(ts|tsx)$/.test(f))
      .filter((f) => /bots\.config|playbook|OPENING|MISTAKES|offShare/.test(readFileSync(f, 'utf8')));
    expect(offenders).toEqual([]);
  });
});
