import type { Command, GameState, ResourceSet } from '../../engine/types';
import { t, useT } from '../../i18n/i18n';
import { COMMAND_ICON, commandEnds } from '../villageName';
import { useClock } from '../useClock';
import { AutoFarmBadge } from './BotTag';
import { Countdown } from './Countdown';
import { PixelIcon } from '../pixelIcons';
import { ResourceList } from './ResourceList';
import { UnitCountList } from './UnitCountList';

/** "1 merchant returning" vs "2 merchants returning": the count picks the singular key. */
export function merchantText(merchants: number): string {
  return t(merchants === 1 ? 'overview.merchantsBackOne' : 'overview.merchantsBack', { m: merchants });
}

/** "1 merchant" vs "2 merchants": the count picks the singular key. */
export function merchantCount(merchants: number): string {
  return t(merchants === 1 ? 'movements.merchantOne' : 'movements.merchantMany', { m: merchants });
}

/** One line of a command table: the fields both the Overview tabs and the village Movements panel display. */
export interface CommandRow {
  id: string;
  type: Command['type'];
  ends: { from: string; to: string };
  units: Command['units'];
  haul: ResourceSet | null;
  merchants: number;
  incoming: boolean;
  arrivesAt: number;
  /** The auto-farm sent this march — or, on the way home, that it came from one. */
  autoFarm: boolean;
}

/** Turns commands into table rows, soonest arrival first. `isIncoming` marks a haul another player is bringing. */
export function commandRows(
  game: GameState,
  cmds: readonly Command[],
  isIncoming: (c: Command) => boolean,
): CommandRow[] {
  return [...cmds]
    // Soonest arrival first; the id only breaks a tie so the order does not shuffle between renders.
    .sort((a, b) => a.arrivesAt - b.arrivesAt || a.id.localeCompare(b.id))
    .map((c) => ({
      id: c.id,
      type: c.type,
      ends: commandEnds(game, c),
      units: c.units,
      haul: c.haul ? { wood: c.haul.wood, clay: c.haul.clay, iron: c.haul.iron } : null,
      merchants: c.merchants ?? 0,
      incoming: isIncoming(c),
      arrivesAt: c.arrivesAt,
      autoFarm: !!c.autoFarm,
    }));
}

/**
 * The one table every command list is drawn with: the Overview's Commands and Trading tabs and the Movements panel on
 * the village page and the Market. `table-stack` turns the rows into cards on a phone.
 */
export function CommandsTable({ rows }: { rows: readonly CommandRow[] }) {
  const t = useT();
  const clock = useClock();
  return (
    <table className="table-game table-stack">
      <thead>
        <tr>
          <th>{t('overview.type')}</th>
          <th>{t('overview.route')}</th>
          <th>{t('overview.unitsLoot')}</th>
          <th>{t('overview.arrival')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id}>
            <td><PixelIcon name={COMMAND_ICON[r.type]} size={16} label={t(`rally.cmd.${r.type}`)} /></td>
            <td className="text-[11px]" data-label={t('overview.route')}>
              {r.ends.from} → {r.ends.to}{r.autoFarm && <AutoFarmBadge />}
            </td>
            <td data-label={t('overview.units')}>
              <span className="inline-flex items-center gap-1.5 flex-wrap">
                {r.type === 'trade'
                  // The exact resources, not a total: a sum says nothing about what actually arrived.
                  ? (r.haul
                    ? <><ResourceList set={r.haul} size={13} /><span className="text-muted">· {merchantCount(r.merchants)}</span></>
                    : <span className="text-muted">{merchantText(r.merchants)}</span>)
                  : <UnitCountList units={r.units} />}
                {r.incoming && <span className="text-ok font-semibold text-[11px]">{t('movements.incoming')}</span>}
              </span>
            </td>
            <td className="tabular-nums" data-label={t('overview.arrival')}>
              <span className={r.incoming ? 'text-ok font-semibold' : undefined}>
                <Countdown until={r.arrivesAt} />
              </span>{' '}
              <span className="text-muted text-[10px]">{clock.at(r.arrivesAt)}</span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
