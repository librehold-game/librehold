import { useEffect, useMemo, useRef } from 'react';
import type { PublicMap } from '../api';
import { useT } from '../../i18n/i18n';
import { minimapPixels } from '../../shared/terrain/minimap';
import { ONLINE_TERRAIN_SEED } from '../../shared/terrain/seed';
import { mapPixels } from './mapPixels';
import './catalog';

/** The whole world at one pixel per field (at most 200 a side): terrain and a dot per village. */
export function WorldMap({ map, name }: { map: PublicMap; name: string }) {
  const t = useT();
  const canvas = useRef<HTMLCanvasElement>(null);
  const terrain = useMemo(() => minimapPixels(ONLINE_TERRAIN_SEED, map.size), [map.size]);
  useEffect(() => {
    const ctx = canvas.current?.getContext('2d');
    if (!ctx || typeof ImageData === 'undefined') return;
    ctx.putImageData(new ImageData(new Uint8ClampedArray(mapPixels(terrain, map, map.size)), terrain.size, terrain.size), 0, 0);
  }, [map, terrain]);
  return (
    <figure className="world-map">
      <canvas ref={canvas} width={terrain.size} height={terrain.size} role="img" aria-label={t('site.worlds.mapLabel', { name })} />
      <figcaption className="muted">{t('site.worlds.legend')}</figcaption>
    </figure>
  );
}
