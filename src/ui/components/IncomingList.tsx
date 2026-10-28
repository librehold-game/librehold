import { useState } from 'react';
import { travelTimeForUnits, type UnitCounts } from '../../engine/commands';
import { incomingTrains, labelIncoming, type IncomingLabel } from '../../engine/incoming';
import { useT } from '../../i18n/i18n';
import { villageName } from '../../i18n/names';
import { villageLabelOf } from '../villageName';
import { humanVillages, isHumanVillage } from '../../engine/players';
import type { UnitId } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { formatCoords } from '../format';
import { tileAt } from '../map/tiles';
import { coordsOfVillageId } from '../../engine/types';
import { Countdown } from './Countdown';

const DEFENSIVE: UnitId[] = ['spear', 'sword', 'archer', 'heavyCav'];

interface Row { id: string; from: string; label: IncomingLabel; arrivesAt: number; train: string[] | null; snipeAt: number | null }
interface Helper { id: string; name: string; units: UnitCounts }

/** Incoming attacks on the village: origin, a label from the march time, countdown; one-click dodge and snipe. */
export function IncomingList({ villageId }: { villageId: string }) {
  const json = useGameStore((s) => {
    const g = s.game!;
    const trains = incomingTrains(g, villageId);
    const rows: Row[] = g.commands
      .filter((c) => c.type === 'attack' && c.targetId === villageId && !(g.villages[c.originId] && isHumanVillage(g.villages[c.originId]!)))
      .sort((a, b) => a.arrivesAt - b.arrivesAt)
      .map((c) => {
        const at = coordsOfVillageId(c.originId);
        const o = g.villages[c.originId] ?? (at ? tileAt(g, at.x, at.y) : undefined);
        const train = trains.find((t) => t.commands.some((x) => x.id === c.id));
        const [n1, n2] = train ? [...train.commands].sort((a, b) => a.arrivesAt - b.arrivesAt) : [];
        return {
          id: c.id, from: o && 'name' in o ? `${'ownerName' in o ? villageName(o, o.ownerName) : villageLabelOf(g, o)} ${formatCoords(o.x, o.y)}` : at ? formatCoords(at.x, at.y) : '?', label: labelIncoming(g, c), arrivesAt: c.arrivesAt,
          train: train ? train.commands.map((x) => x.id) : null,
          snipeAt: n1 && n2 ? n1.arrivesAt + Math.floor((n2.arrivesAt - n1.arrivesAt) / 2) : null,
        };
      });
    const target = g.villages[villageId];
    const helpers: Helper[] = humanVillages(g).filter((v) => v.id !== villageId).map((v) => {
      const units: UnitCounts = {};
      for (const u of DEFENSIVE) if (v.units[u] > 0) units[u] = v.units[u];
      return { id: v.id, name: `${villageLabelOf(g, v)} ${formatCoords(v.x, v.y)}`, units };
    }).filter((h) => Object.keys(h.units).length > 0);
    const reach = Object.fromEntries(helpers.map((h) => [h.id, target ? g.now + travelTimeForUnits(g, g.villages[h.id]!, target.x, target.y, h.units) : Infinity]));
    return JSON.stringify({ rows, helpers, reach });
  });
  const { rows, helpers, reach } = JSON.parse(json) as { rows: Row[]; helpers: Helper[]; reach: Record<string, number> };
  const t = useT();
  const dodge = useGameStore((s) => s.dodge);
  const snipe = useGameStore((s) => s.snipe);
  const [picking, setPicking] = useState<string | null>(null);
  if (rows.length === 0) return null;
  return (
    <div className="mt-3">
      <h3 className="text-[15px] mb-1 text-bad">{t('incoming.title')}</h3>
      <div className="overflow-x-auto"><table className="table-game table-stack w-full text-[12px]">
        <thead><tr><th>{t('incoming.from')}</th><th>{t('incoming.kind')}</th><th>{t('incoming.arrival')}</th><th>{t('incoming.defence')}</th></tr></thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td data-label={t('incoming.from')}>{r.from}</td>
              <td data-label={t('incoming.kind')}>{t(`incomingLabel.${r.label}`)}{r.train && <span className="text-bad font-semibold"> · {t('incoming.train', { n: r.train.length })}</span>}</td>
              <td data-label={t('incoming.arrival')} className="tabular-nums"><Countdown until={r.arrivesAt} /></td>
              <td data-label={t('incoming.defence')}>
                <div className="flex flex-wrap gap-1">
                  <button className="btn" onClick={() => dodge(r.id)} title={t('incoming.dodgeTitle')}>{t('incoming.dodge')}</button>
                  {r.train && r.snipeAt !== null && (
                    <button className="btn" onClick={() => setPicking(picking === r.id ? null : r.id)} title={t('incoming.snipeTitle')}>{t('incoming.snipe')}</button>
                  )}
                </div>
                {picking === r.id && r.train && r.snipeAt !== null && (
                  <ul className="mt-1 space-y-0.5">
                    {helpers.filter((h) => (reach[h.id] ?? Infinity) <= r.snipeAt!).map((h) => (
                      <li key={h.id}>
                        <button className="btn !py-0" onClick={() => { snipe(r.train!, h.id, h.units); setPicking(null); }}>{h.name}</button>
                      </li>
                    ))}
                    {helpers.every((h) => (reach[h.id] ?? Infinity) > r.snipeAt!) && <li className="text-muted italic">{t('incoming.noneInTime')}</li>}
                  </ul>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table></div>
    </div>
  );
}
