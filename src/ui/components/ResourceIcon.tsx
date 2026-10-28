import type { Resource } from '../../engine/types';
import { PixelIcon } from '../pixelIcons';

export function ResourceIcon({ r, size = 18 }: { r: Resource; size?: number }) {
  return <PixelIcon name={r} size={size} />;
}

export function PopIcon({ size = 18 }: { size?: number }) {
  return <PixelIcon name="pop" size={size} />;
}
