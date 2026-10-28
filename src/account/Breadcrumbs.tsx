import { Link } from './router';

/** Where the page sits: links up the way, the page itself last. */
export function Breadcrumbs({ items, label }: { items: { name: string; path: string }[]; label: string }) {
  return (
    <nav className="breadcrumbs" aria-label={label}>
      {items.map((it, i) => (i < items.length - 1
        ? <span key={it.path}><Link to={it.path}>{it.name}</Link> › </span>
        : <span key={it.path} aria-current="page">{it.name}</span>))}
    </nav>
  );
}
