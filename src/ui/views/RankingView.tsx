import { useEffect, useState } from 'react';
import { HUMAN_ID } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import type { ConquestRow, DailyRow, PlayerRow } from '../../net/types';
import { useUiStore } from '../../state/uiStore';
import { formatCoords, formatNumber } from '../format';
import { useClock } from '../useClock';
import { t, useT } from '../../i18n/i18n';
import { BotTag } from '../components/BotTag';

type Tab = 'points' | 'villages' | 'od' | 'daily' | 'conquests';
const TABS: Tab[] = ['points', 'villages', 'od', 'daily', 'conquests'];
const PAGE = 50;


function PlayerName({ id, name, color, isBot }: { id: string; name: string; color?: string; isBot?: boolean }) {
  const open = useUiStore((s) => s.openProfile);
  return (
    <>
      <button className="inline-flex items-center gap-1 underline decoration-dotted" onClick={() => open(id)}>
        {color && <span className="inline-block w-2.5 h-2.5 rounded-sm border border-ink" style={{ background: color }} />}
        {name}
      </button>
      {isBot && <BotTag />}
    </>
  );
}

function Pager({ page, pages, setPage }: { page: number; pages: number; setPage: (p: number) => void }) {
  if (pages <= 1) return null;
  return (
    <div className="flex items-center gap-2 mt-2 text-[12px]">
      <button className="btn" disabled={page === 0} onClick={() => setPage(page - 1)}>←</button>
      <span>{t('ranking.page', { n: page + 1, pages })}</span>
      <button className="btn" disabled={page >= pages - 1} onClick={() => setPage(page + 1)}>→</button>
    </div>
  );
}

/** Rankings from the server, refetched whenever the world's ranking changes. */
export function useRankings(): { players: PlayerRow[]; daily: DailyRow[]; conquests: ConquestRow[] } {
  const rev = useGameStore((s) => s.rankingRev);
  const load = useGameStore((s) => s.loadRanking);
  const [data, setData] = useState<{ players: PlayerRow[]; daily: DailyRow[]; conquests: ConquestRow[] }>({ players: [], daily: [], conquests: [] });
  useEffect(() => {
    let live = true;
    void Promise.all([load('points'), load('daily'), load('conquests')]).then(([players, daily, conquests]) => {
      if (live) setData({ players: (players ?? []) as PlayerRow[], daily: (daily ?? []) as DailyRow[], conquests: (conquests ?? []) as ConquestRow[] });
    });
    return () => { live = false; };
  }, [rev, load]);
  return data;
}

/** World rankings — public data only (points, villages, defeated, daily, conquests). */
export function RankingView() {
  const clock = useClock();
  const [tab, setTab] = useState<Tab>('points');
  const [page, setPage] = useState(0);
  const [odKind, setOdKind] = useState<'att' | 'def' | 'sup'>('att');
  const [dailyKind, setDailyKind] = useState<'att' | 'def' | 'loot' | 'farm'>('att');
  const { players, daily, conquests } = useRankings();
  const short = useGameStore((s) => s.instant !== null);
  const choose = (next: Tab) => { setTab(next); setPage(0); };
  useT();

  let body: React.ReactNode;
  let total = 0;
  if (tab === 'points' || tab === 'villages' || tab === 'od') {
    const key = tab === 'points' ? 'points' : tab === 'villages' ? 'villages' : odKind;
    const sorted = [...players].sort((a, b) => b[key] - a[key] || b.points - a.points);
    total = sorted.length;
    const label = tab === 'points' ? t('ranking.points') : tab === 'villages' ? t('ranking.villages') : t(`ranking.od.${odKind}`);
    body = (
      <table className="table-game table-stack w-full text-[12px]">
        <thead><tr><th>#</th><th>{t('ranking.player')}</th><th>{label}</th>{tab !== 'points' && <th>{t('ranking.points')}</th>}</tr></thead>
        <tbody>
          {sorted.slice(page * PAGE, page * PAGE + PAGE).map((r, i) => (
            <tr key={r.id} className={r.id === HUMAN_ID ? 'ranking-me font-semibold' : r.quit ? 'opacity-60' : ''}>
              <td data-label="#" className="tabular-nums">{page * PAGE + i + 1}</td>
              <td data-label={t('ranking.player')}><PlayerName id={r.id} name={r.name} color={r.color} isBot={r.isBot} />{r.quit && <span className="text-muted">{t('ranking.inactive')}</span>}</td>
              <td data-label={label} className="tabular-nums">{formatNumber(r[key])}</td>
              {tab !== 'points' && <td data-label={t('ranking.points')} className="tabular-nums">{formatNumber(r.points)}</td>}
            </tr>
          ))}
        </tbody>
      </table>
    );
  } else if (tab === 'daily') {
    const sorted = [...daily].sort((a, b) => b[dailyKind] - a[dailyKind]).filter((r) => r[dailyKind] > 0);
    total = sorted.length;
    const label = t(`ranking.daily.${dailyKind}`);
    body = sorted.length === 0 ? <p className="text-muted italic">{t('ranking.nobodyToday')}</p> : (
      <table className="table-game table-stack w-full text-[12px]">
        <thead><tr><th>#</th><th>{t('ranking.player')}</th><th>{label}</th></tr></thead>
        <tbody>
          {sorted.slice(page * PAGE, page * PAGE + PAGE).map((r, i) => (
            <tr key={r.id} className={r.id === HUMAN_ID ? 'ranking-me font-semibold' : ''}>
              <td data-label="#" className="tabular-nums">{page * PAGE + i + 1}</td>
              <td data-label={t('ranking.player')}><PlayerName id={r.id} name={r.name} isBot={r.isBot} /></td>
              <td data-label={label} className="tabular-nums">{formatNumber(r[dailyKind])}</td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  } else {
    total = conquests.length;
    body = conquests.length === 0 ? <p className="text-muted italic">{t('ranking.noConquests')}</p> : (
      <table className="table-game table-stack w-full text-[12px]">
        <thead><tr><th>{t('ranking.when')}</th><th>{t('ranking.village')}</th><th>{t('ranking.newOwner')}</th><th>{t('ranking.previous')}</th></tr></thead>
        <tbody>
          {conquests.slice(page * PAGE, page * PAGE + PAGE).map((c, i) => (
            <tr key={i} className={c.toId === HUMAN_ID || c.fromId === HUMAN_ID ? 'ranking-me' : ''}>
              <td data-label={t('ranking.when')} className="tabular-nums">{clock.at(c.t)}</td>
              <td data-label={t('ranking.village')}>{formatCoords(c.x, c.y)}</td>
              <td data-label={t('ranking.newOwner')}><PlayerName id={c.toId} name={c.to} isBot={c.toIsBot} /></td>
              <td data-label={t('ranking.previous')}>{c.fromId ? <>{t('ranking.from')} <PlayerName id={c.fromId} name={c.from!} isBot={c.fromIsBot} /></> : t('ranking.barbarian')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 flex-wrap">
        <h2 className="text-lg mr-2">{t('ranking.title')}</h2>
        {TABS.map((id) => <button key={id} className={`btn ${tab === id ? '!bg-gold/40' : ''}`} onClick={() => choose(id)}>{t(`ranking.tab.${id}`)}</button>)}
      </div>
      {tab === 'od' && (
        <div className="flex gap-1 text-[12px]">
          {(['att', 'def', 'sup'] as const).map((k) => (
            <button key={k} className={`btn ${odKind === k ? '!bg-gold/40' : ''}`} onClick={() => { setOdKind(k); setPage(0); }}>{t(`ranking.odKind.${k}`)}</button>
          ))}
        </div>
      )}
      {tab === 'daily' && (
        <div className="flex gap-1 text-[12px] flex-wrap">
          {(['att', 'def', 'loot', 'farm'] as const).map((k) => (
            <button key={k} className={`btn ${dailyKind === k ? '!bg-gold/40' : ''}`} onClick={() => { setDailyKind(k); setPage(0); }}>{t(`ranking.dailyKind.${k}`)}</button>
          ))}
        </div>
      )}
      <p className="text-muted text-[11px]">{t(short ? 'ranking.refreshShort' : 'ranking.refresh')}</p>
      <div className="overflow-x-auto">{body}</div>
      <Pager page={page} pages={Math.ceil(total / PAGE)} setPage={setPage} />
    </div>
  );
}
