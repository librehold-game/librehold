// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { WorldForm, liveChanges } from '../src/account/admin/WorldForm';
import { DEFAULT_CONFIG, FIELDS, type WorldConfig } from '../src/account/worldConfig';
import { DEFAULT_WORLD_CONFIG, WORLD_LIMITS } from '../src/data/world.config';

afterEach(() => cleanup());

describe('world settings form', () => {
  it('defaults and ranges are the server\'s', () => {
    // The server leaves both at zero: field battles on, and a sight radius of 0 means 1.
    expect({ ...DEFAULT_CONFIG, noFieldBattles: undefined, sightRadius: undefined }).toEqual(DEFAULT_WORLD_CONFIG);
    expect(DEFAULT_CONFIG.sightRadius).toBe(1);
    const field = (key: keyof WorldConfig) => FIELDS.find((f) => f.key === key)!;
    for (const key of ['worldSpeed', 'unitSpeed', 'botCount', 'rampUpDays', 'beginnerProtectionDays'] as const) {
      expect([field(key).min, field(key).max], key).toEqual([WORLD_LIMITS[key].min, WORLD_LIMITS[key].max]);
    }
    for (const key of ['humanModifiers', 'botModifiers'] as const) {
      expect([field(key).min, field(key).max], key).toEqual([WORLD_LIMITS.modifier.min, WORLD_LIMITS.modifier.max]);
    }
    expect(field('mapSize').choices).toEqual(WORLD_LIMITS.mapSizes.map(String));
    expect([field('sightRadius').min, field('sightRadius').max]).toEqual([1, WORLD_LIMITS.sightRadius.max]);
  });

  it('field battles can be switched off and the sight radius set', () => {
    setLocale('en');
    let value: WorldConfig = { ...DEFAULT_CONFIG };
    const { rerender } = render(<WorldForm idPrefix="w" value={value} onChange={(v) => { value = v; }} mode="create" />);
    const off = screen.getByLabelText('No field battles') as HTMLInputElement;
    expect(off.checked).toBe(false);
    fireEvent.click(off);
    expect(value.noFieldBattles).toBe(true);
    rerender(<WorldForm idPrefix="w" value={value} onChange={(v) => { value = v; }} mode="create" />);
    fireEvent.change(screen.getByLabelText(/Sight radius/), { target: { value: '5' } });
    expect(value.sightRadius).toBe(5);
  });

  it('switching field battles off is a live change, the sight radius is fixed at creation', () => {
    const before: WorldConfig = { ...DEFAULT_CONFIG };
    expect(liveChanges(before, { ...before, noFieldBattles: true, sightRadius: 3 })).toEqual({ noFieldBattles: true });
  });
});
