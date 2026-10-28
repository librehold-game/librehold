import { BUILDING_IDS, UNIT_IDS, coordsOfVillageId, type UnitId } from '../../engine/types';
import { useT } from '../../i18n/i18n';
import { buildingName, unitName, villageName } from '../../i18n/names';
import type { CommandView, PlayerRow, PublicPlayer, SpectatorState, WatchedVillage } from '../../net/types';
import { BotTag } from '../components/BotTag';
import { formatCoords, formatDuration, formatNumber } from '../format';

/** Whose army it is: a return belongs to the village it goes home to, any other army to the one it left. */
export function armyOwnerId(c: CommandView, villageOwner: (id: string) => string): string {
  return villageOwner(c.type === 'return' ? c.targetId : c.originId);
}

/** A village or field point as the god view names it: its owner, or barbarians, with its coordinates. */
export function placeLabel(id: string, ownerOf: (id: string) => PublicPlayer | undefined, barbarian: string): string {
  const at = coordsOfVillageId(id);
  const coords = at ? ` ${formatCoords(at.x, at.y)}` : '';
  const owner = ownerOf(id);
  return `${owner ? owner.name : barbarian}${coords}`;
}

function UnitList({ units }: { units: Partial<Record<UnitId, number>> | undefined }) {
  const t = useT();
  const list = UNIT_IDS.filter((u) => (units?.[u] ?? 0) > 0);
  if (list.length === 0) return <span className="text-muted">{t('watch.noTroops')}</span>;
  return (
    <ul className="flex flex-wrap gap-x-2">
      {list.map((u) => <li key={u}>{formatNumber(units![u]!)} {unitName(u, units![u]!)}</li>)}
    </ul>
  );
}

export function RankingTable({ rows }: { rows: PlayerRow[] }) {
  const t = useT();
  return (
    <table aria-label={t('watch.ranking')} className="w-full text-[12px]">
      <thead>
        <tr className="text-left text-muted"><th>#</th><th>{t('watch.col.player')}</th><th>{t('watch.col.points')}</th><th>{t('watch.col.villages')}</th></tr>
      </thead>
      <tbody>
        {rows.map((r, i) => (
          <tr key={r.id}>
            <td>{i + 1}</td>
            <td><span style={{ color: r.color }} aria-hidden>■ </span><span>{r.name}</span>{r.isBot && <BotTag />}</td>
            <td className="tabular-nums">{formatNumber(r.points)}</td>
            <td className="tabular-nums">{r.villages}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

interface Lookups {
  ownerOf: (villageId: string) => PublicPlayer | undefined;
  playerOf: (playerId: string) => PublicPlayer | undefined;
}

export function ArmyList({ commands, lookups, onOpen }: { commands: CommandView[]; lookups: Lookups; onOpen: (id: string) => void }) {
  const t = useT();
  const barbarian = t('village.barbarian');
  if (commands.length === 0) return <p className="text-muted text-[12px]">{t('watch.noArmies')}</p>;
  return (
    <ul aria-label={t('watch.armies')} className="text-[12px] flex flex-col gap-0.5 max-h-[360px] overflow-y-auto">
      {commands.map((c) => {
        const owner = lookups.playerOf(armyOwnerId(c, (id) => lookups.ownerOf(id)?.id ?? ''));
        return (
          <li key={c.id}>
            <button className="w-full text-left px-1 rounded hover:bg-white/40" onClick={() => onOpen(c.id)}>
              {t(`watch.kind.${c.type}`)}: {t('watch.route', { owner: owner?.name ?? barbarian, target: placeLabel(c.targetId, lookups.ownerOf, barbarian) })}
              <span className="text-muted"> · {t('watch.arrives', { time: formatDuration(c.arrivesAt) })}</span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function Card({ label, onClose, children }: { label: string; onClose: () => void; children: React.ReactNode }) {
  const t = useT();
  return (
    <section aria-label={label} className="panel p-2 text-[12px]">
      <div className="flex items-start justify-between gap-1">
        <h3 className="font-heading text-[14px]">{label}</h3>
        <button className="btn !px-1.5 !py-0" onClick={onClose} title={t('watch.close')} aria-label={t('watch.close')}>✕</button>
      </div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">{children}</dl>
    </section>
  );
}

export function ArmyCardView({ command, lookups, onClose }: { command: CommandView; lookups: Lookups; onClose: () => void }) {
  const t = useT();
  const barbarian = t('village.barbarian');
  const owner = lookups.playerOf(armyOwnerId(command, (id) => lookups.ownerOf(id)?.id ?? ''));
  return (
    <Card label={t('watch.army')} onClose={onClose}>
      <dt className="text-muted">{t('watch.owner')}</dt>
      <dd>{owner ? <>{owner.name}{owner.isBot && <BotTag />}</> : barbarian}</dd>
      <dt className="text-muted">{t('watch.target')}</dt>
      <dd>{t(`watch.kind.${command.type}`)} · {placeLabel(command.targetId, lookups.ownerOf, barbarian)}</dd>
      <dt className="text-muted">{t('watch.arrival')}</dt>
      <dd className="tabular-nums">{formatDuration(command.arrivesAt)}</dd>
      <dt className="text-muted">{t('watch.troops')}</dt>
      <dd><UnitList units={command.units} /></dd>
    </Card>
  );
}

export function VillageCardView({ village, onClose }: { village: WatchedVillage | null; onClose: () => void }) {
  const t = useT();
  return (
    <Card label={t('watch.village')} onClose={onClose}>
      {!village ? <dd className="text-muted col-span-2">{t('watch.loading')}</dd> : (
        <>
          <dt className="text-muted">{t('watch.village')}</dt>
          <dd>{villageName(village, village.ownerName)} {formatCoords(village.x, village.y)}</dd>
          <dt className="text-muted">{t('watch.owner')}</dt>
          <dd>{village.ownerName ? <>{village.ownerName}{village.ownerIsBot && <BotTag />}</> : t('village.barbarian')}</dd>
          <dt className="text-muted">{t('watch.points')}</dt>
          <dd className="tabular-nums">{formatNumber(village.points)}</dd>
          <dt className="text-muted">{t('watch.buildings')}</dt>
          <dd>
            <ul className="flex flex-wrap gap-x-2">
              {BUILDING_IDS.filter((b) => (village.buildings[b] ?? 0) > 0).map((b) => <li key={b}>{buildingName(b)} {village.buildings[b]}</li>)}
            </ul>
          </dd>
          <dt className="text-muted">{t('watch.troopsHome')}</dt>
          <dd><UnitList units={village.unitsHome} /></dd>
        </>
      )}
    </Card>
  );
}

/** Who owns what, read from the spectator state. */
export function lookupsOf(s: SpectatorState): Lookups {
  const owners = new Map(s.villages.map((v) => [v.id, v.ownerId]));
  const playerOf = (id: string) => (id ? s.players[id] : undefined);
  return { playerOf, ownerOf: (villageId) => playerOf(owners.get(villageId) ?? '') };
}
