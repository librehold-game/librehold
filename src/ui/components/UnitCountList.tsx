import { unitName } from '../../i18n/names';
import { UNIT_IDS, type UnitId } from '../../engine/types';
import { UNITS } from '../../data/units.data';
import { UnitIcon } from './UnitIcon';
import { formatNumber } from '../format';

export function UnitCountList({ units, size = 16 }: { units: Partial<Record<UnitId, number>>; size?: number }) {
  const present = UNIT_IDS.filter((u) => (units[u] ?? 0) > 0);
  if (present.length === 0) return <span className="text-muted">—</span>;
  return (
    <span className="inline-flex flex-wrap gap-2">
      {present.map((u) => (
        <span key={u} className="inline-flex items-center gap-0.5 tabular-nums" title={unitName(u)}>
          <UnitIcon unit={u} size={size} />{formatNumber(units[u] ?? 0)}
        </span>
      ))}
    </span>
  );
}
