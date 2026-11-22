import { markKey, type PlayerMark } from "../../state/uiStore";

/**
 * Other players picked out by name (`UiPrefs.mapPlayerMarks`) so their villages stand out on the map and the minimap.
 * The marks are a display choice kept in the browser, so they are matched against the names the world shows.
 */

/** The colour a player is marked with, by the player's name; undefined for anyone unmarked. */
export type MarkLookup = (ownerName: string | undefined) => string | undefined;

/**
 * A lookup from a player's name to the colour the player marked them with. Names are compared the way a player types
 * them — trimmed and without regard to case — because the mark is remembered separately from the world it was made in.
 */
export function markLookup(marks: readonly PlayerMark[]): MarkLookup {
  if (!marks.length) return () => undefined;
  const byName = new Map<string, string>();
  for (const m of marks) byName.set(markKey(m.name), m.color);
  return (name) => (name ? byName.get(markKey(name)) : undefined);
}

/**
 * Your own villages carry a corner dot too, so you can find them without reading roofs: yellow for every village of
 * yours. They are not marks — the player cannot recolour or unmark them — but they share the corner with a mark, so
 * they live with the mark colours. The village you are in needs no colour of its own: it already has a gold outline.
 */
export const OWN_VILLAGE_COLOR = "#ffd400";

/** Colours a new mark starts from, in order: distinct enough that neighbouring marks stay apart on the map. */
export const MARK_COLORS: readonly string[] = [
  "#e6194b",
  "#3cb44b",
  "#4363d8",
  "#f58231",
  "#911eb4",
  "#42d4f4",
  "#f032e6",
  "#9a6324",
];

/** The colour the next mark starts with: the first one no mark uses yet, and after the palette runs out, its turn. */
export function nextMarkColor(marks: readonly PlayerMark[]): string {
  const used = new Set(marks.map((m) => m.color.toLowerCase()));
  return (
    MARK_COLORS.find((c) => !used.has(c)) ??
    MARK_COLORS[marks.length % MARK_COLORS.length]!
  );
}
