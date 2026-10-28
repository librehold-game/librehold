import { Link } from '../router';

export interface InlineLink { to: string; label: string }

/** A text with its "{reportLink}" token turned into a link. */
export function withLink(text: string, link?: InlineLink) {
  if (!link || !text.includes('{reportLink}')) return text;
  const [before, after = ''] = text.split('{reportLink}');
  return <>{before}<Link to={link.to}>{link.label}</Link>{after}</>;
}

/** Paragraphs separated by blank lines. */
export function RichText({ text, link }: { text: string; link?: InlineLink }) {
  return <>{text.split(/\n\s*\n/).map((p, i) => <p key={i}>{withLink(p, link)}</p>)}</>;
}
