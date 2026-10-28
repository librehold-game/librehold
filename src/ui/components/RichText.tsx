import { Fragment } from 'react';

/** A translated sentence with <b>…</b> parts shown bold; everything else is plain text, never HTML. */
export function RichText({ text }: { text: string }) {
  const parts = text.split(/<b>(.*?)<\/b>/g);
  return <>{parts.map((p, i) => (i % 2 === 1 ? <strong key={i}>{p}</strong> : <Fragment key={i}>{p}</Fragment>))}</>;
}
