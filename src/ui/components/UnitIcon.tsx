import { unitName } from '../../i18n/names';
import { UNITS } from '../../data/units.data';
import type { UnitId } from '../../engine/types';
import { PixelIcon } from '../pixelIcons';

export function UnitIcon({ unit, size = 18 }: { unit: UnitId; size?: number }) {
  return <PixelIcon name={`unit-${unit}`} size={size} label={unitName(unit)} />;
}
