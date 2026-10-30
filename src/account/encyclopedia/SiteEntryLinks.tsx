import type { ReactNode } from 'react';
import { useLocale } from '../../i18n/i18n';
import { Link } from '../router';
import { EntryLinkContext, type EntryRef } from '../../shared/encyclopedia/EntryLink';
import { buildingPath, unitPath } from './entries';

/** On the site a reference to a building or unit is a link to its page. */
export function SiteEntryLinks({ children }: { children: ReactNode }) {
  const locale = useLocale();
  const render = (to: EntryRef, label: ReactNode) => (
    <Link to={to.kind === 'unit' ? unitPath(to.id, locale) : buildingPath(to.id, locale)}>{label}</Link>
  );
  return <EntryLinkContext.Provider value={render}>{children}</EntryLinkContext.Provider>;
}
