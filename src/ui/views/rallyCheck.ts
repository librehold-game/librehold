import { checkSend, type SendBlocker, type SendCheck, type UnitCounts } from '../../engine/commands';
import type { GameState } from '../../engine/types';
import { mapCache } from '../../state/gameStore';

/**
 * The engine's send preview, corrected by what the client knows and by the server's rules: whether a village
 * stands on the target comes from the client's own villages and the loaded map (unknown until loaded — the
 * server decides), and support may go to any player's village but not to barbarians.
 */
export function checkSendClient(g: GameState, originId: string, x: number, y: number, units: UnitCounts, type: 'attack' | 'support'): SendCheck {
  const check = checkSend(g, originId, x, y, units, type);
  const blockers: SendBlocker[] = check.blockers.filter((b) => b.kind !== 'noTarget' && b.kind !== 'supportTargetNotOwned');
  const own = g.villages[`v:${x}:${y}`];
  const tile = own ?? mapCache()?.tileAt(x, y) ?? null;
  if (tile === 'empty') blockers.push({ kind: 'noTarget' });
  else if (tile && type === 'support' && tile.ownerId === null) blockers.push({ kind: 'supportTargetBarbarian' });
  return { ...check, blockers };
}
