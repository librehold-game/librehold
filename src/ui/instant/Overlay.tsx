import type { ReactNode } from 'react';

/** A centred dialog over a dimmed game; titleId names the dialog by its heading. */
export function Overlay({ titleId, label, wide, children }: { titleId?: string; label?: string; wide?: boolean; children: ReactNode }) {
  return (
    <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-2">
      <div role="dialog" aria-modal="true" aria-labelledby={titleId} aria-label={label}
        className={`panel w-full ${wide ? 'max-w-[560px]' : 'max-w-[420px]'} max-h-[85vh] overflow-y-auto`}>
        {children}
      </div>
    </div>
  );
}
