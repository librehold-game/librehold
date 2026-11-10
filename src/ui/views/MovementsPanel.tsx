import { useState } from 'react';
import { commandHome } from '../../engine/commands';
import { humanVillageIds } from '../../engine/players';
import type { Command } from '../../engine/types';
import { t, useT } from '../../i18n/i18n';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { CommandsTable, commandRows, type CommandRow } from '../components/CommandsTable';
import { SidePanel } from '../components/SidePanel';

/** Resources on their way to one of my villages, whoever sent them — a shipment between two of my villages counts too. */
function isArrivingHaul(c: Command, own: ReadonlySet<string>): boolean {
  return c.type === 'trade' && c.returning !== true && own.has(c.targetId);
}

/** A shipment another player is bringing to one of my villages, which the panel flags so it is not mistaken for mine. */
function isIncomingHaul(c: Command, own: ReadonlySet<string>): boolean {
  return isArrivingHaul(c, own) && !own.has(c.originId);
}

/** What a Movements panel lists. `trade` keeps merchant shipments only, for screens where troops make no sense. */
export type MovementScope = 'all' | 'trade';

/**
 * The village a command is homed at: where a march or a cart started, or where a returning army — or the empty
 * merchants of a return leg — is heading. A trade is stored with its origin, so `returning` is what says whether
 * its merchants belong to the village it left or the one it is coming back to.
 */
function movementHome(c: Command): string {
  return c.type === 'trade' && c.returning === true ? c.targetId : commandHome(c);
}

/**
 * Whether a command belongs on a Movements list covering these villages: what they have on the road, plus a shipment
 * arriving at one of them. A single-village screen passes just that village; the Overview passes the villages it is
 * showing, so a group sees all of their movements and nothing from outside the group.
 */
export function isMovementOf(c: Command, homes: ReadonlySet<string>, own: ReadonlySet<string>, scope: MovementScope = 'all'): boolean {
  if (scope === 'trade' && c.type !== 'trade') return false;
  return homes.has(movementHome(c)) || (homes.has(c.targetId) && isArrivingHaul(c, own));
}

interface Row extends CommandRow {}

/** How many movements one page holds. A village can have dozens of marches out; the panel is a card, not a log. */
export const MOVEMENTS_PER_PAGE = 8;

/**
 * What is on the road right now for one village, or for the villages named in `villageIds`: their attacks, support and
 * returns with the troops they carry, and every merchant shipment they sent or are about to receive. Only those
 * villages' movements — another village's march belongs on that village's list. Sorted by time remaining, soonest
 * first. Returns null when nothing is under way.
 *
 * `scope` narrows it to merchant shipments: the Market screen shows this panel too, where a march is not its business.
 */
export function MovementsPanel({ scope = 'all', villageIds, hideAutoFarm }: {
  scope?: MovementScope;
  villageIds?: readonly string[];
  /** Leave the auto-farm's marches off the list; the section's toggle passes its preference in. */
  hideAutoFarm?: boolean;
}) {
  useT();
  const activeId = useGameStore((s) => selectActiveVillage(s).id);
  // The panel follows one village in the sidebar and a whole group on the Overview. Both go through `homes`, so there
  // is one rule for what belongs here; the key only exists to drop the page number when the set being shown changes.
  const homes = villageIds ?? [activeId];
  const key = homes.join(',');
  const [paging, setPaging] = useState<{ key: string; page: number }>({ key, page: 0 });
  const json = useGameStore((s) => {
    const g = s.game!;
    const own = new Set(humanVillageIds(g));
    const shown = new Set(homes);
    const rows = commandRows(g, g.commands.filter((c) => isMovementOf(c, shown, own, scope)), (c) => isIncomingHaul(c, own));
    return JSON.stringify(rows);
  });
  const rows = JSON.parse(json) as Row[];
  if (rows.length === 0) return null;
  // Hiding the auto-farm can empty a list that the heading still announces, so the panel says why rather than
  // leaving an empty card — and the toggle above it stays reachable.
  const visible = hideAutoFarm ? rows.filter((r) => !r.autoFarm) : rows;
  if (visible.length === 0) return <p className="text-muted italic text-[11px]">{t('movements.autoFarmHidden')}</p>;
  const pages = Math.ceil(visible.length / MOVEMENTS_PER_PAGE);
  // Switching village (or group) starts at the first page again; otherwise the old page number would point into a
  // list that has nothing to do with what the player is now looking at.
  const page = paging.key === key ? paging.page : 0;
  // Movements leave the list all the time, so the last page can empty out under the player.
  const p = Math.max(0, Math.min(page, pages - 1));
  const pageRows = visible.slice(p * MOVEMENTS_PER_PAGE, (p + 1) * MOVEMENTS_PER_PAGE);
  const turn = (to: number) => setPaging({ key, page: to });
  return (
    <>
      <CommandsTable rows={pageRows} />
      {pages > 1 && (
        <div className="flex items-center justify-end gap-2 mt-1">
          <button className="btn" disabled={p === 0} onClick={() => turn(p - 1)}>{t('market.pagePrev')}</button>
          <span className="text-muted font-body text-[10px]">{p + 1} / {pages}</span>
          <button className="btn" disabled={p >= pages - 1} onClick={() => turn(p + 1)}>{t('market.pageNext')}</button>
        </div>
      )}
    </>
  );
}

/**
 * The Movements panel plus its heading, or nothing at all when the village has nothing of that kind under way. The
 * heading carries a − / + toggle, as the village side panels do; the choice is a browser preference, so it survives
 * the card unmounting when the last movement lands. The same goes for the auto-farm filter beside it.
 */
export function MovementsSection({ scope = 'all', heading }: { scope?: MovementScope; heading?: string }) {
  const moving = useGameStore((s) => {
    const g = s.game!;
    return g.commands.some((c) => isMovementOf(c, new Set([selectActiveVillage(s).id]), new Set(humanVillageIds(g)), scope));
  });
  const collapsed = useUiStore((s) => s.prefs.movementsCollapsed);
  const hideAutoFarm = useUiStore((s) => s.prefs.movementsHideAutoFarm);
  const setPrefs = useUiStore((s) => s.setPrefs);
  const t = useT();
  // Only worth offering when there is something for it to hide: the filter sits in the heading, and the player is
  // looking for the auto-farm's marches among their own.
  const someAutoFarm = useGameStore((s) => {
    const g = s.game!;
    const own = new Set(humanVillageIds(g));
    return g.commands.some((c) => !!c.autoFarm && isMovementOf(c, new Set([selectActiveVillage(s).id]), own, scope));
  });
  if (!moving) return null;
  return (
    <div className="mt-3">
      <SidePanel
        title={heading ?? t('village.movements')}
        open={!collapsed}
        onToggle={(open) => setPrefs({ movementsCollapsed: !open })}
        right={someAutoFarm || hideAutoFarm ? (
          <label className="inline-flex items-center gap-1 font-body font-normal not-italic text-[11px] cursor-pointer">
            <input type="checkbox" className="align-middle" checked={hideAutoFarm}
              onChange={(e) => setPrefs({ movementsHideAutoFarm: e.target.checked })} />
            {t('movements.hideAutoFarm')}
          </label>
        ) : undefined}
      >
        <div className="p-2"><MovementsPanel scope={scope} hideAutoFarm={hideAutoFarm} /></div>
      </SidePanel>
    </div>
  );
}
