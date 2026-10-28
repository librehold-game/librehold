import type { CSSProperties } from 'react';
import sizes from '../../assets/pixel/map/villages.json';
import type { VillageSize } from './MapArt';

/**
 * A village on the classic skin's map: the pixel sprite of its point tier (`npm run assets:pixel-map`), drawn at the
 * terrain's art scale (96×64 art px per tile). Other players' roofs take the owner's colour: grey roofs of the `-tint`
 * sprite multiplied by that colour through the `-roof` mask. Banners mark the bonus and the active village.
 */
const URLS = import.meta.glob<string>('../../assets/pixel/map/{village-*,banner*}.png', { eager: true, query: '?url', import: 'default' });

function url(name: string): string {
  const u = URLS[`../../assets/pixel/map/${name}.png`];
  if (!u) throw new Error(`Missing pixel village sprite: ${name}`);
  return u;
}

/** Share of the art scale per tier: a lone cottage is drawn larger so it reads at day 1, a castle smaller so it fits. */
export const VILLAGE_SCALES: readonly number[] = [1.1, 1, 0.95, 0.95, 0.85, 0.85];
const BONUS_COLOR = '#e0b23a';
const ACTIVE_COLOR = '#2f5fa8';

export interface PixelVillageProps {
  village: { owner: 'player' | 'barbarian'; size: VillageSize; bonus: boolean; active: boolean; hidden: boolean; ownerColor?: string };
  tw: number;
  th: number;
}

function tinted(mask: string, color: string): CSSProperties {
  const image = `url(${url(mask)})`;
  return {
    position: 'absolute', inset: 0, backgroundColor: color, mixBlendMode: 'multiply',
    maskImage: image, WebkitMaskImage: image, maskSize: '100% 100%', WebkitMaskSize: '100% 100%',
  };
}

function Banner({ kind, left, top, kx, ky }: { kind: 'bonus' | 'active'; left: number; top: number; kx: number; ky: number }) {
  const [w, h] = sizes.banner as [number, number];
  return (
    <div data-banner={kind} style={{ position: 'absolute', left, top, width: Math.round(w * kx), height: Math.round(h * ky) }}>
      <img src={url('banner')} alt="" draggable={false} style={{ display: 'block', width: '100%', height: '100%' }} />
      <div style={tinted('banner-cloth', kind === 'bonus' ? BONUS_COLOR : ACTIVE_COLOR)} />
    </div>
  );
}

export function PixelVillage({ village, tw, th }: PixelVillageProps) {
  const { owner, size, bonus, active, hidden, ownerColor } = village;
  const kx = tw / 96, ky = th / 64;
  const [nw, nh] = (sizes as Record<string, number[]>)[`village-${size}`] as [number, number];
  const w = Math.round(nw * kx * VILLAGE_SCALES[size]!), h = Math.round(nh * ky * VILLAGE_SCALES[size]!);
  const left = Math.round((tw - w) / 2), top = Math.round((th - h) / 2);
  const tint = owner === 'player' && ownerColor ? ownerColor : undefined;
  const name = owner === 'barbarian' ? `village-${size}-barb` : tint ? `village-${size}-tint` : `village-${size}`;
  const bannerH = Math.round((sizes.banner as number[])[1]! * ky);
  return (
    <div data-pixel-village style={{ position: 'absolute', inset: 0, opacity: hidden ? 0.45 : 1 }}>
      {active && (
        <div style={{
          position: 'absolute', left: left - 2, top: top + Math.round(h * 0.35), width: w + 4, height: Math.round(h * 0.7),
          border: '1.5px solid #ffd34a', borderRadius: '50%', boxSizing: 'border-box',
        }} />
      )}
      <div style={{ position: 'absolute', left, top, width: w, height: h }}>
        <img data-village={name} src={url(name)} width={w} height={h} alt="" draggable={false} style={{ display: 'block' }} />
        {tint && <div data-roof style={tinted(`village-${size}-roof`, tint)} />}
      </div>
      {active && <Banner kind="active" left={left - 2} top={top - Math.round(bannerH * 0.45)} kx={kx} ky={ky} />}
      {bonus && <Banner kind="bonus" left={left + Math.round(w * 0.7)} top={top - Math.round(bannerH * 0.45)} kx={kx} ky={ky} />}
    </div>
  );
}
