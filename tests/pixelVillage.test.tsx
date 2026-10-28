// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { PixelVillage, VILLAGE_SCALES, type PixelVillageProps } from '../src/ui/map/PixelVillage';
import sizes from '../src/assets/pixel/map/villages.json';
import { TileArt } from '../src/ui/map/MapArt';

afterEach(cleanup);

const base: PixelVillageProps['village'] = { owner: 'player', size: 2, bonus: false, active: false, hidden: false };
const show = (village: Partial<PixelVillageProps['village']>) =>
  render(<PixelVillage village={{ ...base, ...village }} tw={53} th={38} />).container;
const sprite = (c: HTMLElement) => c.querySelector<HTMLImageElement>('img[data-village]')!;
const roofTint = (c: HTMLElement) => c.querySelector<HTMLElement>('[data-roof]');
const banners = (c: HTMLElement) => [...c.querySelectorAll<HTMLElement>('[data-banner]')].map((b) => b.dataset.banner);

describe('pixel village on the map', () => {
  it('your village has the red-roofed tier sprite', () => {
    const c = show({ size: 3 });
    expect(sprite(c).src).toContain('village-3.png');
    expect(roofTint(c)).toBeNull();
  });

  it('a barbarian village has the slate-roofed variant', () => {
    expect(sprite(show({ owner: 'barbarian', size: 0 })).src).toContain('village-0-barb.png');
  });

  it("another player's village has roofs in the owner's colour", () => {
    const c = show({ size: 5, ownerColor: '#123456' });
    expect(sprite(c).src).toContain('village-5-tint.png');
    const roof = roofTint(c)!;
    expect(roof.style.backgroundColor).toMatch(/#123456|rgb\(18, 52, 86\)/);
    expect(roof.style.maskImage || roof.style.webkitMaskImage).toContain('village-5-roof.png');
  });

  it('every tier is drawn at the map art scale of the tile', () => {
    for (let t = 0; t < 6; t++) {
      const [w, h] = (sizes as Record<string, number[]>)[`village-${t}`]!;
      const img = sprite(show({ size: t as PixelVillageProps['village']['size'] }));
      expect(img.width, `tier ${t}`).toBe(Math.round(w! * (53 / 96) * VILLAGE_SCALES[t]!));
      expect(img.height, `tier ${t}`).toBe(Math.round(h! * (38 / 64) * VILLAGE_SCALES[t]!));
      cleanup();
    }
  });

  it('a lone cottage stays readable and a castle still fits its tile', () => {
    expect(sprite(show({ size: 0 })).width).toBeGreaterThanOrEqual(0.4 * 53);
    cleanup();
    const castle = sprite(show({ size: 5 }));
    expect(castle.width).toBeLessThanOrEqual(53);
    expect(castle.height).toBeLessThanOrEqual(42);
  });

  it('bonus villages carry a gold banner, the active village a blue one', () => {
    expect(banners(show({ bonus: true }))).toEqual(['bonus']);
    cleanup();
    expect(banners(show({ active: true }))).toEqual(['active']);
    cleanup();
    expect(banners(show({ active: true, bonus: true })).sort()).toEqual(['active', 'bonus']);
    cleanup();
    expect(banners(show({}))).toEqual([]);
  });

  it('a hidden village is faded', () => {
    expect(show({ hidden: true }).querySelector<HTMLElement>('[data-pixel-village]')!.style.opacity).toBe('0.45');
  });
});

describe('map tile in the classic skin', () => {
  it('draws the pixel village and the command dot, nothing on an empty tile', () => {
    const c = render(<TileArt village={{ ...base, size: 4 }} dot="a" tw={53} th={38} />).container;
    expect(sprite(c).src).toContain('village-4.png');
    expect(c.querySelector('circle')).not.toBeNull();
    cleanup();
    expect(render(<TileArt tw={53} th={38} />).container.querySelector('img')).toBeNull();
  });
});
