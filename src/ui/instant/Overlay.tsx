import type { ReactNode } from "react";

/** A centred dialog over a dimmed game; titleId names the dialog by its heading. */
export function Overlay({
  titleId,
  label,
  wide,
  onClose,
  children,
}: {
  titleId?: string;
  label?: string;
  wide?: boolean;
  onClose?: () => void;
  children: ReactNode;
}) {
  return (
    <div
      className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-2"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose?.();
      }}
    >
      {/* `pb-px` covers the 1px a button sinks while pressed (`.btn:active`), which would otherwise poke out of this
          scroll box and flash a scrollbar whenever a dialog's content ends flush with its buttons. */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-label={label}
        className={`panel w-full ${wide ? "max-w-[560px]" : "max-w-[420px]"} max-h-[85vh] overflow-y-auto pb-px`}
      >
        {children}
      </div>
    </div>
  );
}
