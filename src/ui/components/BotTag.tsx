import { useT } from '../../i18n/i18n';

/** A small mark after a computer player's name, so nobody mistakes a bot for a person. */
export function BotTag() {
  const t = useT();
  return <span className="bot-tag ml-1 px-1 rounded-sm text-[10px] font-semibold uppercase align-middle">{t('player.bot')}</span>;
}
