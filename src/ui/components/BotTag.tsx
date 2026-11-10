import type { ReactNode } from 'react';
import { useT } from '../../i18n/i18n';

function Tag({ className, children, title }: { className: string; children: ReactNode; title?: string }) {
  return <span className={`ml-1 px-1 rounded-sm text-[10px] align-middle whitespace-nowrap ${className}`} title={title}>{children}</span>;
}

/** A small mark after a computer player's name, so nobody mistakes a bot for a person. */
export function BotTag() {
  const t = useT();
  return <Tag className="bot-tag font-semibold uppercase">{t('player.bot')}</Tag>;
}

/** The auto-farm's mark in a report, in words: the heading and the report list have room for them. */
export function AutoFarmTag() {
  const t = useT();
  return <Tag className="border border-wood-border font-body font-normal text-muted">{t('reports.autoFarmTag')}</Tag>;
}

/**
 * The same mark where the line is tight: a Movements row puts it after a route that already fills the cell, so the
 * words wrapped mid-word and pushed the row taller. Two letters, never broken; the words are the tooltip.
 */
export function AutoFarmBadge() {
  const t = useT();
  return <Tag className="border border-wood-border font-body font-normal text-muted uppercase" title={t('reports.autoFarmTag')}>{t('movements.autoFarmBadge')}</Tag>;
}
