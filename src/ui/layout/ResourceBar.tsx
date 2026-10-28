import { RESOURCE_BALANCE } from '../../data/buildings.data';
import { msUntilFull, villageProductionPerHour } from '../../engine/resources';
import { RESOURCES, type Resource } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { selectActiveStorageCap, selectConfig } from '../../state/selectors';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatDuration, formatNumber } from '../format';
import { useT } from '../../i18n/i18n';
import { resourceName } from '../../i18n/names';

function ResourceCell({ r, compact }: { r: Resource; compact: boolean }) {
  const t = useT();
  const amount = useGameStore((s) => Math.floor(selectActiveVillage(s).resources[r]));
  const cap = useGameStore(selectActiveStorageCap);
  const perHour = useGameStore((s) => villageProductionPerHour(selectActiveVillage(s), r, selectConfig(s)));
  const untilFull = useGameStore((s) => msUntilFull(selectActiveVillage(s), r, selectConfig(s)));

  const ratio = cap > 0 ? amount / cap : 1;
  const warn = ratio >= RESOURCE_BALANCE.storageWarningRatio;
  const tooltip = t('resourceBar.tooltip', {
    name: resourceName(r), amount: formatNumber(amount), cap: formatNumber(cap), perHour: formatNumber(perHour),
    full: untilFull === 0 ? t('resourceBar.full') : t('resourceBar.fullIn', { time: formatDuration(untilFull) }),
  });

  return (
    <span
      className={`flex items-center gap-1 tabular-nums ${warn ? 'text-bad font-semibold' : ''}`}
      title={tooltip}
    >
      <ResourceIcon r={r} />
      {formatNumber(amount)}
      {!compact && <span className="text-muted font-normal">/{formatNumber(cap)}</span>}
      {warn && <span aria-hidden title={t('resourceBar.nearlyFull')}>⚠</span>}
    </span>
  );
}

/** `compact` — amount only, no capacity. */
export function ResourceBar({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`flex items-center ${compact ? 'gap-3' : 'gap-3 sm:gap-5'}`}>
      {RESOURCES.map((r) => <ResourceCell key={r} r={r} compact={compact} />)}
    </div>
  );
}
