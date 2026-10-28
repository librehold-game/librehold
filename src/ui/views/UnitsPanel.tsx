import { useT } from '../../i18n/i18n';
import { unitName } from '../../i18n/names';
import { UNITS } from '../../data/units.data';
import { unitsAway } from '../../engine/commands';
import { UNIT_IDS, type UnitId } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { UnitIcon } from '../components/UnitIcon';
import { UnitCountList } from '../components/UnitCountList';
import { humanVillageIds } from '../../engine/players';
import { villageLabelById } from '../villageName';
import { formatNumber } from '../format';

/** The "Units" panel: troops present in the village (queued and away from the village — separately, greyed out). */
export function UnitsPanel({ onRecruit }: { onRecruit?: () => void }) {
  const json = useGameStore((s) => {
    const v = selectActiveVillage(s);
    const queued: Partial<Record<UnitId, number>> = {};
    for (const q of Object.values(v.recruitQueues)) for (const j of q) queued[j.unit] = (queued[j.unit] ?? 0) + (j.total - j.done);
    const g = s.game!;
    const away = unitsAway(g, v.id);
    const mine = new Set(humanVillageIds(g));
    const arriving: Partial<Record<UnitId, number>> = {};
    let foreignComing = false;
    for (const c of g.commands) {
      if (c.type !== 'support' || c.targetId !== v.id) continue;
      if (!mine.has(c.originId)) { foreignComing = true; continue; }
      for (const u of UNIT_IDS) if ((c.units[u] ?? 0) > 0) arriving[u] = (arriving[u] ?? 0) + c.units[u]!;
    }
    const stationed = Object.entries(v.supportFrom)
      .filter(([, us]) => UNIT_IDS.some((u) => (us[u] ?? 0) > 0))
      .map(([from, us]) => ({ from: villageLabelById(g, from).label, units: us }));
    return JSON.stringify({ units: v.units, queued, away, arriving, foreignComing, stationed, barracks: v.buildings.barracks });
  });
  const { units, queued, away, arriving, foreignComing, stationed, barracks } = JSON.parse(json) as {
    units: Record<UnitId, number>; queued: Partial<Record<UnitId, number>>; away: Record<UnitId, number>; arriving: Partial<Record<UnitId, number>>;
    foreignComing: boolean; stationed: { from: string; units: Partial<Record<UnitId, number>> }[]; barracks: number;
  };
  const present = UNIT_IDS.filter((u) => units[u] > 0 || (queued[u] ?? 0) > 0 || away[u] > 0 || (arriving[u] ?? 0) > 0);
  const t = useT();
  return (
    <div className="px-2 py-1.5 text-[12px]">
      {present.length === 0 && stationed.length === 0 ? (
        <p className="text-muted italic">{t('unitsPanel.none')} {barracks > 0 ? t('unitsPanel.recruitHint') : t('unitsPanel.buildHint')}</p>
      ) : (
        <ul className="divide-y divide-wood-border/40">
          {present.map((u) => (
            <li key={u} className="flex items-center gap-1.5 py-[3px] tabular-nums" title={unitName(u)}>
              <UnitIcon unit={u} size={16} />
              {u === 'paladin' ? <span className="font-semibold">{unitName(u)}</span> : <><span className="font-bold">{formatNumber(units[u])}</span> <span>{unitName(u, units[u])}</span></>}
              {(queued[u] ?? 0) > 0 && <span className="text-muted text-[11px]" title={t('unitsPanel.queued')}>(+{formatNumber(queued[u] ?? 0)})</span>}
              {away[u] > 0 && <span className="text-muted text-[11px]" title={t('unitsPanel.away')}>{t('unitsPanel.awayCount', { n: formatNumber(away[u]) })}</span>}
              {(arriving[u] ?? 0) > 0 && <span className="text-ok text-[11px]">{t('unitsPanel.arriving', { n: formatNumber(arriving[u] ?? 0) })}</span>}
            </li>
          ))}
        </ul>
      )}
      {stationed.length > 0 && (
        <div data-support className="mt-1.5 pt-1 border-t border-wood-border/60">
          <div className="font-semibold">{t('unitsPanel.support')}</div>
          {stationed.map((st) => (
            <div key={st.from} className="py-[2px]">
              <div className="text-muted text-[11px]">{st.from}</div>
              <UnitCountList units={st.units} />
            </div>
          ))}
        </div>
      )}
      {foreignComing && <p className="text-muted italic text-[11px] mt-1">{t('unitsPanel.supportComing')}</p>}
      {barracks > 0 && onRecruit && (
        <button className="mt-1 font-bold text-ink hover:underline" onClick={onRecruit}>{t('unitsPanel.recruit')}</button>
      )}
    </div>
  );
}
