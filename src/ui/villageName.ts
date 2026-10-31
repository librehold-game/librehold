import { legEnds } from '../engine/fieldGeometry';
import { coordsOfVillageId, type Command, type GameState } from '../engine/types';
import { villageName } from '../i18n/names';
import { formatCoords } from './format';
import { tileAt } from './map/tiles';

/** A village's name as players read it; an empty stored name is worded from its owner. */
export function villageLabelOf(game: GameState, v: { name: string; ownerId: string | null }): string {
  return villageName(v, v.ownerId ? game.players[v.ownerId]?.name ?? null : null);
}

/** A village by id: its label and coordinates, and its owner when the client knows them. */
export function villageLabelById(g: GameState, id: string): { label: string; owner: string | null } {
  const own = g.villages[id];
  if (own) return { label: `${villageLabelOf(g, own)} ${formatCoords(own.x, own.y)}`, owner: own.ownerId ? g.players[own.ownerId]?.name ?? null : null };
  const at = coordsOfVillageId(id);
  if (!at) return { label: '?', owner: null };
  const tile = tileAt(g, at.x, at.y);
  if (tile.kind !== 'village') return { label: formatCoords(at.x, at.y), owner: null };
  return { label: `${villageName(tile, tile.ownerName ?? null)} ${formatCoords(at.x, at.y)}`, owner: tile.ownerName ?? null };
}

/** A command's two ends as players read them: villages by name, places in the field by their coordinates. */
export function commandEnds(g: GameState, c: Command): { from: string; to: string } {
  const ends = legEnds(c);
  const end = (id: string, at?: { x: number; y: number }) =>
    id ? villageLabelById(g, id).label : at ? formatCoords(Math.round(at.x), Math.round(at.y)) : '?';
  return { from: end(c.originId, ends?.from), to: end(c.targetId, ends?.to) };
}

/** The pixel icon of each kind of army on the move. */
export const COMMAND_ICON: Record<Command['type'], string> = { attack: 'attack', support: 'shield', return: 'return', trade: 'cart', intercept: 'lightning' };
