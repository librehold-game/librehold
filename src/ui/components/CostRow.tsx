import { RESOURCES, type ResourceSet } from '../../engine/types';
import { formatNumber } from '../format';
import { ResourceIcon } from './ResourceIcon';

/** Wood/clay/iron cost; missing resources in red. */
export function CostRow({ cost, have }: { cost: ResourceSet; have?: ResourceSet }) {
  return (
    <span className="inline-flex items-center gap-2 tabular-nums">
      {RESOURCES.map((r) => (
        <span
          key={r}
          className={`inline-flex items-center gap-0.5 ${have && have[r] < cost[r] ? 'text-bad font-semibold' : ''}`}
        >
          <ResourceIcon r={r} size={14} />
          {formatNumber(cost[r])}
        </span>
      ))}
    </span>
  );
}
