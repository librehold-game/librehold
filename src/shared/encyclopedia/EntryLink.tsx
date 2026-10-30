import { createContext, useContext, type ReactNode } from 'react';
import type { BuildingId, UnitId } from '../../engine/types';

export type EntryRef = { kind: 'building'; id: BuildingId } | { kind: 'unit'; id: UnitId };

type RenderLink = (to: EntryRef, children: ReactNode) => ReactNode;

/** How a host turns a reference to a building or unit into a link: the site goes to its page, the game opens its window. */
export const EntryLinkContext = createContext<RenderLink>((_to, children) => children);

export function EntryLink({ to, children }: { to: EntryRef; children: ReactNode }) {
  return <>{useContext(EntryLinkContext)(to, children)}</>;
}
