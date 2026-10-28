import { useSyncExternalStore, type AnchorHTMLAttributes, type MouseEvent } from 'react';
import { setLocale, type Locale } from '../i18n/i18n';
import { localeOf, matchPath, type RouteId } from './routes';

const listeners = new Set<() => void>();
let serverPath = '/';

function notify() {
  for (const l of listeners) l();
}

if (typeof window !== 'undefined') {
  window.addEventListener('popstate', () => {
    setLocale(localeOf(window.location.pathname));
    notify();
  });
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

const getPath = () => (typeof window !== 'undefined' ? window.location.pathname : serverPath);

/** The path the prerender renders, where there is no `window`. */
export function setServerPath(path: string): void {
  serverPath = path;
}

export function usePathname(): string {
  return useSyncExternalStore(subscribe, getPath, getPath);
}

export function useRoute(): { id: RouteId; locale: Locale; params: Record<string, string> } {
  const path = usePathname();
  return matchPath(path) ?? { id: 'notFound', locale: localeOf(path), params: {} };
}

export function navigate(path: string, replace = false): void {
  if (replace) history.replaceState(null, '', path);
  else history.pushState(null, '', path);
  setLocale(localeOf(window.location.pathname));
  if (!replace) window.scrollTo(0, 0);
  notify();
}

export function Link({ to, onClick, ...rest }: { to: string } & AnchorHTMLAttributes<HTMLAnchorElement>) {
  const handle = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e);
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || rest.target) return;
    e.preventDefault();
    navigate(to);
  };
  return <a href={to} onClick={handle} {...rest} />;
}
