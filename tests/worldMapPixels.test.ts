import { describe, expect, it } from 'vitest';
import type { PublicMap } from '../src/account/api';
import { mapPixels } from '../src/account/worlds/mapPixels';

const terrain = (size: number) => ({ size, data: new Uint8ClampedArray(size * size * 4).fill(10) });
const pixel = (data: Uint8ClampedArray, size: number, x: number, y: number) => [...data.slice((y * size + x) * 4, (y * size + x) * 4 + 4)];

describe('world map pixels', () => {
  it("owned villages take the owner's colour and barbarians grey", () => {
    const map: PublicMap = { size: 40, players: [{ name: 'Ala', color: '#ff0000', isBot: false }],
      villages: [[3, 4, 0], [10, 2, -1]] };
    const t = terrain(40);
    const out = mapPixels(t, map, 40);
    expect(pixel(out, 40, 3, 4)).toEqual([255, 0, 0, 255]);
    expect(pixel(out, 40, 10, 2)).toEqual([0x8c, 0x8c, 0x8c, 255]);
    expect(pixel(out, 40, 0, 0)).toEqual([10, 10, 10, 10]);
    expect(t.data[(4 * 40 + 3) * 4]).toBe(10);
  });

  it('a 400 map lands on the 200-pixel canvas', () => {
    const map: PublicMap = { size: 400, players: [{ name: 'Bot', color: '#00ff00', isBot: true }], villages: [[399, 0, 0]] };
    const out = mapPixels(terrain(200), map, 400);
    expect(pixel(out, 200, 199, 0)).toEqual([0, 255, 0, 255]);
  });
});
