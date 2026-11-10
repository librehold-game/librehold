import { RESOURCES, type Resource, type ResourceSet } from '../../engine/types';
import { resourceName } from '../../i18n/names';
import { formatNumber } from '../format';
import { ResourceIcon } from './ResourceIcon';

/** Icon + amount per resource, one entry per resource that is actually in the set; `tone` tints each entry. */
export function ResourceList({ set, tone, size = 12 }: { set: ResourceSet; tone?: (r: Resource) => string; size?: number }) {
  const parts = RESOURCES.filter((r) => set[r] > 0).map((r) => (
    <strong key={r} className={`inline-flex items-center gap-0.5 ${tone?.(r) ?? ''}`} title={resourceName(r)}>
      <ResourceIcon r={r} size={size} />{formatNumber(set[r])}
    </strong>
  ));
  if (parts.length === 0) return <span className="text-muted">—</span>;
  return <span className="inline-flex items-center gap-1.5 flex-wrap">{parts}</span>;
}
