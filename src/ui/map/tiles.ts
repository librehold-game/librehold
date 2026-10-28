import { hashSeed } from '../../engine/rng';
import type { GameState } from '../../engine/types';
import type { TileInfo } from '../../engine/world';
import { mapCache } from '../../state/gameStore';

/**
 * A map tile as the client knows it: its own villages first, then the tiles fetched from the server. A tile
 * whose part of the map is not loaded yet shows as empty ground. Terrain is decoration only.
 */
export function tileAt(g: GameState, x: number, y: number): TileInfo {
  const id = `v:${x}:${y}`;
  const known = g.villages[id] ?? mapCache()?.tileAt(x, y);
  if (known && known !== 'empty') {
    const owner = known.ownerId ? g.players[known.ownerId] : undefined;
    return {
      kind: 'village', x, y, id, ownerId: known.ownerId, ownerName: owner?.name, ownerIsBot: owner ? !owner.isHuman : undefined, ownerStatus: owner?.status ?? null,
      ownerColor: owner?.color, name: known.name, points: known.points, bonus: known.bonus, materialized: true,
    };
  }
  return { kind: 'empty', x, y, terrain: hashSeed(g.worldSeed, 'terrain', x, y) % 7 };
}
