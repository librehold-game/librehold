import { buildingName } from '../../i18n/names';
import { t } from '../../i18n/i18n';
import type { ReqNode } from '../../shared/encyclopedia/facts';

const label = (n: ReqNode) => `${buildingName(n.building)} ${n.level}${n.met ? '' : ` (${n.have ?? 0})`}`;

/** One chip per requirement, green when met and red with the village's level when not; a missing building's own gaps go underneath. */
export function RequirementChips({ nodes }: { nodes: ReqNode[] }) {
  const gaps = nodes.map((n) => ({ n, below: n.children.filter((c) => !c.met) })).filter((x) => x.below.length > 0);
  return (
    <>
      <div>
        {nodes.map((n) => <span key={n.building} className={`chip ${n.met ? 'req-met' : 'req-missing'}`}>{label(n)}</span>)}
      </div>
      {gaps.map(({ n, below }) => (
        <div key={n.building} className="text-[11px] text-muted">
          {`↳ ${t('buildings.subNeeds', { building: buildingName(n.building), list: below.map(label).join(', ') })}`}
        </div>
      ))}
    </>
  );
}
