import type { PublicEvent } from './api';
import { pathFor } from './routes';
import { formatDurationHuman, formatNumber } from '../i18n/format';
import { t, type Locale } from '../i18n/i18n';

/** "just now" for anything under a minute old (or a little in the future, with a fast server clock), else "5 min ago". */
export function agoText(at: number, now: number): string {
  return now - at < 60_000 ? t('site.live.justNow') : t('site.live.ago', { time: formatDurationHuman(now - at) });
}

const playerName = (name: string, bot: boolean) => (bot ? t('site.live.botName', { name }) : name);

/** The event's sentence key and its words; "{world}" stays in, for the caller to put a link there. */
export function eventMessage(e: PublicEvent): { key: string; params: Record<string, string | number> } {
  const barbarian = e.defender === '';
  const key = `site.live.event.${e.kind}${barbarian ? 'Barbarian' : ''}`;
  const params: Record<string, string | number> = { attacker: playerName(e.attacker, e.attackerBot), troops: formatNumber(e.troops) };
  if (!barbarian) params.defender = playerName(e.defender, e.defenderBot);
  return { key, params };
}

/** "3 players · 18 bots" */
export function playersAndBots(w: { humans: number; bots: number }): string {
  return `${t('site.live.humans', { count: w.humans })} · ${t('site.live.bots', { count: w.bots })}`;
}

/** An instant game's page, or a long world's. */
export function worldPath(e: { worldId: number; instant: boolean }, locale: Locale): string {
  return pathFor(e.instant ? 'game' : 'world', locale, { id: String(e.worldId) });
}
