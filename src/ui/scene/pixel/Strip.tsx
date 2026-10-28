import { pixelUrl } from './layout';

export interface StripProps {
  name: string;
  file: string;
  w: number;
  h: number;
  frames: number;
  x: number;
  y: number;
  ms?: number;
  frame?: number;
  className?: string;
}

/** Horizontal frame strip seen through a one-frame viewport: a fixed `frame`, or a looping `steps()` animation. */
export function Strip({ name, file, w, h, frames, x, y, ms = frames * 90, frame, className }: StripProps) {
  const style = frame !== undefined
    ? { transform: `translateX(${-frame * w}px)` }
    : { animation: `px-${name} ${ms}ms steps(${frames}, end) infinite` };
  return (
    <svg x={x} y={y} width={w} height={h} overflow="hidden" data-anim={name} className={className}>
      {frame === undefined && <style>{`@keyframes px-${name} { to { transform: translateX(${-w * frames}px) } }`}</style>}
      <image href={pixelUrl(file)} width={w * frames} height={h} style={style} />
    </svg>
  );
}
