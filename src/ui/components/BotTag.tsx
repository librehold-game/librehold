import type { ReactNode } from 'react';
import { useT } from '../../i18n/i18n';

function Tag({ className, children }: { className: string; children: ReactNode }) {
  return <span className={`ml-1 px-1 rounded-sm text-[10px] align-middle ${className}`}>{children}</span>;
}

/** A small mark after a computer player's name, so nobody mistakes a bot for a person. */
export function BotTag() {
  const t = useT();
  return <Tag className="bot-tag font-semibold uppercase">{t('player.bot')}</Tag>;
}

export function AutoFarmTag() {
  const t = useT();
  return <Tag className="border border-wood-border font-body font-normal text-muted">{t('reports.autoFarmTag')}</Tag>;
}
