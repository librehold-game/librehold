import { newsText } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { humanPlayer } from '../../engine/players';
import type { NewsItem } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { formatDurationHuman } from '../format';
import { useClock } from '../useClock';

export function NewsView() {
  const clock = useClock();
  const t = useT();
  const json = useGameStore((s) => {
    const g = s.game!;
    return JSON.stringify({
      news: [...g.news].reverse().slice(0, 100),
      protection: Math.max(0, humanPlayer(g).protectedUntil - g.now),
    });
  });
  const m = JSON.parse(json) as { news: NewsItem[]; protection: number };
  return (
    <div className="text-[12px]">
      <div className="flex items-center gap-3 flex-wrap mb-2">
        <h2 className="text-lg">{t('news.title')}</h2>
        {m.protection > 0 && <span className="text-ok font-semibold">{t('news.protection', { time: formatDurationHuman(m.protection) })}</span>}
      </div>
      <div className="space-y-1">
        {m.news.length === 0 ? <p className="text-muted italic">{t('news.empty')}</p> : m.news.map((n) => (
          <div key={n.id} className="panel px-2 py-1 flex gap-2 items-start">
            <span>🗣️</span>
            <span className="flex-1"><span className="text-muted italic">{clock.day(n.time)}</span> — {newsText(n)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
