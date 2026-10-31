import { Fragment, type ReactNode } from 'react';
import { PixelIcon } from '../pixelIcons';

/** Plain text with every [icon:name] drawn as that pixel icon. */
function withIcons(text: string, size: number): ReactNode[] {
  return text.split(/\[icon:([a-z0-9-]+)\]/g).map((p, i) => (i % 2 === 1 ? <PixelIcon key={i} name={p} size={size} /> : <Fragment key={i}>{p}</Fragment>));
}

/** A translated sentence with <b>…</b> parts shown bold and [icon:name] markers drawn as pixel icons; never HTML. */
export function RichText({ text, iconSize = 16 }: { text: string; iconSize?: number }) {
  const parts = text.split(/<b>(.*?)<\/b>/g);
  return <>{parts.map((p, i) => (i % 2 === 1 ? <strong key={i}>{withIcons(p, iconSize)}</strong> : <Fragment key={i}>{withIcons(p, iconSize)}</Fragment>))}</>;
}
