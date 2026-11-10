import { useState } from 'react';
import { t, useT } from '../../i18n/i18n';

/**
 * A village card: the wooden header bar with an italic title and a − / + collapse button, and its content below. The
 * Production, Units and Current parameters panels are drawn with it, and so is the Movements card.
 *
 * `open` can be controlled from outside (the Movements card remembers its fold as a browser preference, because the
 * card unmounts whenever the last movement lands); leave it out and the panel keeps the fold to itself.
 */
export function SidePanel({ title, children, open, onToggle, right }: {
  title: React.ReactNode;
  children: React.ReactNode;
  open?: boolean;
  onToggle?: (open: boolean) => void;
  right?: React.ReactNode;
}) {
  const [own, setOwn] = useState(true);
  useT();
  const isOpen = open ?? own;
  const toggle = () => {
    const next = !isOpen;
    if (open === undefined) setOwn(next);
    onToggle?.(next);
  };
  return (
    <section className="panel overflow-hidden">
      <div className="panel-header flex items-center justify-between !py-1 !px-2">
        <span className="italic font-bold">{title}</span>
        <span className="flex items-center gap-1">
          {right}
          <button className="btn !px-1.5 !py-0 !text-[11px] leading-4" onClick={toggle} title={isOpen ? t('village.collapse') : t('village.expand')}>{isOpen ? '−' : '+'}</button>
        </span>
      </div>
      {isOpen && children}
    </section>
  );
}
