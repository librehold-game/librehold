import { unitName } from '../../i18n/names';
import { UNITS } from '../../data/units.data';
import type { UnitId } from '../../engine/types';
import { InfoLink } from '../info/InfoLink';
import { PixelIcon } from '../pixelIcons';

/** With `info`, a click opens the unit's details window. */
export function UnitIcon({ unit, size = 18, info }: { unit: UnitId; size?: number; info?: boolean }) {
  const icon = <PixelIcon name={`unit-${unit}`} size={size} label={unitName(unit)} />;
  return info ? <InfoLink to={{ kind: 'unit', id: unit }} className="inline-flex cursor-pointer">{icon}</InfoLink> : icon;
}
