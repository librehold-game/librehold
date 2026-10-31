import type { GameState, Report, ReportVillage } from './types';

export function unreadReports(state: GameState): number {
  let n = 0;
  for (const r of state.reports) if (!r.read) n++;
  return n;
}

export function reportVillageOf(state: GameState, id: string): ReportVillage {
  const v = state.villages[id];
  if (v) {
    const rv: ReportVillage = { id, name: v.name, x: v.x, y: v.y };
    if (v.ownerId) {
      rv.ownerId = v.ownerId;
      const p = state.players[v.ownerId];
      if (p) {
        rv.ownerName = p.name;
        if (!p.isHuman) rv.ownerIsBot = true;
      }
    }
    return rv;
  }
  const m = id.match(/^v:(\d+):(\d+)$/);
  return { id, name: '', x: m ? Number(m[1]) : 0, y: m ? Number(m[2]) : 0 };
}

/** The pixel icon of a report's outcome, for the report list. */
export function reportIcon(r: Report): string {
  if (r.type === 'conquest') return 'nav-village';
  if (r.field?.missed) return 'dust';
  if (r.type === 'scout' || r.type === 'fieldScout') return r.outcome === 'attackerWon' ? 'magnifier' : 'eye-closed';
  if (r.type === 'support') return 'shield';
  if (r.outcome === 'defenderWon') return 'skull';
  const lost = Object.values(r.attacker.losses).some((n) => (n ?? 0) > 0);
  return lost ? 'attack' : 'laurel';
}
