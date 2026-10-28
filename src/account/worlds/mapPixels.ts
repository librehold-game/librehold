import type { PublicMap } from '../api';

const BARBARIAN: [number, number, number] = [0x8c, 0x8c, 0x8c];

const rgb = (hex: string): [number, number, number] => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [number, number, number];

/** The terrain with a dot per village: its owner's colour, or grey for barbarians. */
export function mapPixels(terrain: { size: number; data: Uint8ClampedArray }, map: PublicMap, mapSize: number): Uint8ClampedArray {
  const { size } = terrain;
  const out = new Uint8ClampedArray(terrain.data);
  const colors = map.players.map((p) => rgb(p.color));
  for (const [x, y, owner] of map.villages) {
    const px = Math.floor((x * size) / mapSize), py = Math.floor((y * size) / mapSize);
    if (px < 0 || py < 0 || px >= size || py >= size) continue;
    out.set([...(colors[owner] ?? BARBARIAN), 255], (py * size + px) * 4);
  }
  return out;
}
