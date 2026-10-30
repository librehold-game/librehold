import type { ReactNode } from 'react';
import { useUiStore } from '../../state/uiStore';
import type { EntryRef } from '../../shared/encyclopedia/EntryLink';

/** A name that opens the details window of a building or unit. */
export function InfoLink({ to, children, className }: { to: EntryRef; children: ReactNode; className?: string }) {
  const openInfo = useUiStore((s) => s.openInfo);
  return (
    <button type="button" className={className ?? 'font-semibold underline decoration-dotted cursor-pointer'} onClick={() => openInfo(to)}>
      {children}
    </button>
  );
}
