// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { PopIcon, ResourceIcon } from '../src/ui/components/ResourceIcon';
import { UnitIcon } from '../src/ui/components/UnitIcon';
import { NavIcon } from '../src/ui/layout/Nav';
import { PixelIcon, pixelIcon } from '../src/ui/pixelIcons';

afterEach(cleanup);

describe('pixel icons', () => {
  it('pixelIcon resolves an exported icon and throws for a missing one', () => {
    expect(pixelIcon('unit-spear')).toMatch(/unit-spear/);
    expect(() => pixelIcon('nope')).toThrow(/Missing pixel icon: nope/);
  });

  it('units, resources and the population render the pixel icons, pixelated', () => {
    const { container } = render(<><UnitIcon unit="spear" /><ResourceIcon r="clay" /><PopIcon /></>);
    const srcs = [...container.querySelectorAll('img')].map((i) => i.getAttribute('src') ?? '');
    expect(srcs).toEqual([expect.stringMatching(/unit-spear/), expect.stringMatching(/clay/), expect.stringMatching(/pop/)]);
    expect(container.querySelector('img')?.getAttribute('class')).toContain('pixel-icon');
    expect(container.querySelector('svg')).toBeNull();
  });

  it('icons drawn at 24px or more stay pixelated, smaller ones are smoothed so details do not break up', () => {
    const { container } = render(<><PixelIcon name="wood" size={32} /><PixelIcon name="wood" size={14} /></>);
    const [big, small] = [...container.querySelectorAll('img')];
    expect(big!.style.imageRendering).toBe('pixelated');
    expect(small!.style.imageRendering).toBe('auto');
  });

  it('the unit icon keeps the unit name as its accessible label', () => {
    const { container } = render(<UnitIcon unit="noble" />);
    expect(container.querySelector('img')?.getAttribute('alt')).toBe('Hetman');
  });

  it('nav tabs and the menu button use the pixel icons', () => {
    const { container } = render(<><NavIcon id="map" /><NavIcon id="menu" /></>);
    const srcs = [...container.querySelectorAll('img')].map((i) => i.getAttribute('src') ?? '');
    expect(srcs).toEqual([expect.stringMatching(/nav-map/), expect.stringMatching(/nav-menu/)]);
  });
});

describe('icons named in text and tables', () => {
  it('every [icon:name] in the catalogues and every army icon is an exported icon', async () => {
    const { COMMAND_ICON } = await import('../src/ui/villageName');
    const catalogues = import.meta.glob<string>('../src/i18n/catalog/*/*.json', { eager: true, query: '?raw', import: 'default' });
    const names = new Set(Object.values(COMMAND_ICON));
    for (const text of Object.values(catalogues)) for (const m of text.matchAll(/\[icon:([a-z0-9-]+)\]/g)) names.add(m[1]!);
    expect(names.size).toBeGreaterThan(5);
    for (const n of names) expect(() => pixelIcon(n), n).not.toThrow();
  });
});
