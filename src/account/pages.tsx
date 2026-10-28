import { lazy, Suspense, type ComponentType, type LazyExoticComponent } from 'react';
import { useT } from '../i18n/i18n';
import type { Account } from './api';
import type { RouteId } from './routes';

export interface PageProps {
  params: Record<string, string>;
  me: Account | null | undefined; // undefined while /me loads
  onLogin: (a: Account) => void;
  onClaimed: (a: Account) => void;
}

export type PageComponent = ComponentType<PageProps>;

interface Entry {
  page?: PageComponent;
  lazy?: LazyExoticComponent<PageComponent>;
  load?: () => Promise<void>;
}

// The browser registers most pages as lazy (pagesLazy), the prerender every page as provided (pagesEager).
const entries = new Map<RouteId, Entry>();

export function provideRoute(id: RouteId, page: PageComponent): void {
  entries.set(id, { page });
}

export function lazyRoute(id: RouteId, load: () => Promise<{ default: PageComponent }>): void {
  const entry: Entry = {};
  let pending: Promise<void> | undefined;
  entry.load = () => {
    pending ??= load().then(
      (m) => {
        entry.page = m.default;
      },
      (err: unknown) => {
        pending = undefined;
        throw err;
      },
    );
    return pending;
  };
  entry.lazy = lazy(async () => {
    await entry.load!();
    return { default: entry.page! };
  });
  entries.set(id, entry);
}

/** Loads a lazy route's page, so it then renders at once (before hydration); resolves at once for provided routes. */
export function preloadRoute(id: RouteId): Promise<void> {
  const entry = entries.get(id) ?? entries.get('notFound');
  return entry?.page || !entry?.load ? Promise.resolve() : entry.load();
}

function PageLoading() {
  const t = useT();
  return (
    <section className="card page-loading" role="progressbar" aria-busy="true" aria-label={t('loading')}>
      <p className="muted">{t('loading')}</p>
      <div className="progress-track"><div className="progress-bar" /></div>
    </section>
  );
}

/** The page of a route, or of notFound when the route has none. */
export function RoutePage({ id, ...props }: PageProps & { id: RouteId }) {
  const entry = entries.get(id) ?? entries.get('notFound');
  const Page = entry?.page ?? entry?.lazy;
  if (!Page) return null;
  return (
    <Suspense fallback={<PageLoading />}>
      <Page {...props} />
    </Suspense>
  );
}
