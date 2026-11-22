import type { PlayerMark } from "./types";

/** A name as marks are compared: trimmed and case-folded, since players type it from memory. */
export function markKey(name: string): string {
  return name.trim().toLowerCase();
}

/**
 * Marks as they may be drawn: only a name and a hex colour survive, so a state from an older server, or one that
 * was tampered with, cannot put anything else on the map. The server keeps the same list clean; this is the client's
 * own guard, since the colours go straight into the map's drawing.
 */
export function cleanMarks(raw: unknown): PlayerMark[] {
  if (!Array.isArray(raw)) return [];
  const out: PlayerMark[] = [];
  const seen = new Set<string>();
  for (const m of raw) {
    if (!m || typeof m !== "object") continue;
    const { name, color } = m as Partial<PlayerMark>;
    if (typeof name !== "string" || typeof color !== "string") continue;
    const key = markKey(name);
    if (!key || seen.has(key) || !/^#[0-9a-f]{6}$/i.test(color)) continue;
    seen.add(key);
    out.push({ name: name.trim(), color });
  }
  return out;
}
