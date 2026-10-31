import { useState, type ReactNode } from 'react';
import { formatCoords, formatNumber } from '../../i18n/format';
import { useT } from '../../i18n/i18n';
import './catalog';
import type { ConquestRow, DailyRow, PlayerRow } from './types';

export type RankingTab = 'points' | 'villages' | 'od' | 'daily' | 'conquests';
const TABS: RankingTab[] = ['points', 'villages', 'od', 'daily', 'conquests'];
const PAGE = 50;

export interface RankingData { players: PlayerRow[]; daily: DailyRow[]; conquests: ConquestRow[] }

/** What differs between the game and the public site: how a name looks and behaves, who is "me", how a time reads. */
export interface RankingHost {
  renderPlayer(p: { id: string; name: string; color?: string; isBot?: boolean }): ReactNode;
  isMe(id: string): boolean;
  formatTime(ms: number): string;
  refreshNote?: string;
}

function Player({ host, id, name, color, isBot }: { host: RankingHost; id: string; name: string; color?: string; isBot: boolean }) {
  const t = useT();
  return (
    <>
      {host.renderPlayer({ id, name, color, isBot })}
      {isBot && <span className="bot-tag">{t('ranking.bot')}</span>}
    </>
  );
}

function Pager({ page, pages, setPage }: { page: number; pages: number; setPage: (p: number) => void }) {
  const t = useT();
  if (pages <= 1) return null;
  return (
    <div className="ranking-pager">
      <button className="ranking-btn" disabled={page === 0} onClick={() => setPage(page - 1)}>←</button>
      <span>{t('ranking.page', { n: page + 1, pages })}</span>
      <button className="ranking-btn" disabled={page >= pages - 1} onClick={() => setPage(page + 1)}>→</button>
    </div>
  );
}

function Kinds<K extends string>({ kinds, value, onPick, label }: { kinds: readonly K[]; value: K; onPick: (k: K) => void; label: (k: K) => string }) {
  return (
    <div className="ranking-kinds">
      {kinds.map((k) => <button key={k} className="ranking-btn" aria-pressed={value === k} onClick={() => onPick(k)}>{label(k)}</button>)}
    </div>
  );
}

const OD_KINDS = ['att', 'def', 'sup'] as const;
const DAILY_KINDS = ['att', 'def', 'loot', 'farm'] as const;

/** The world's rankings: points, villages, defeated, daily and conquests, fifty rows a page. */
export function RankingTable({ data, host, title }: { data: RankingData; host: RankingHost; title?: string }) {
  const t = useT();
  const [tab, setTab] = useState<RankingTab>('points');
  const [page, setPage] = useState(0);
  const [odKind, setOdKind] = useState<(typeof OD_KINDS)[number]>('att');
  const [dailyKind, setDailyKind] = useState<(typeof DAILY_KINDS)[number]>('att');
  const choose = (next: RankingTab) => { setTab(next); setPage(0); };
  const pick = <K,>(set: (k: K) => void) => (k: K) => { set(k); setPage(0); };

  let body: ReactNode;
  let total = 0;
  if (tab === 'points' || tab === 'villages' || tab === 'od') {
    const key = tab === 'od' ? odKind : tab;
    const sorted = [...data.players].sort((a, b) => b[key] - a[key] || b.points - a.points);
    total = sorted.length;
    const label = tab === 'points' ? t('ranking.points') : tab === 'villages' ? t('ranking.villages') : t(`ranking.od.${odKind}`);
    body = (
      <table className="ranking-table">
        <thead><tr><th>#</th><th>{t('ranking.player')}</th><th>{label}</th>{tab !== 'points' && <th>{t('ranking.points')}</th>}</tr></thead>
        <tbody>
          {sorted.slice(page * PAGE, page * PAGE + PAGE).map((r, i) => (
            <tr key={r.id} className={host.isMe(r.id) ? 'ranking-me' : r.quit ? 'ranking-quit' : undefined}>
              <td data-label="#">{page * PAGE + i + 1}</td>
              <td data-label={t('ranking.player')}>
                <Player host={host} id={r.id} name={r.name} color={r.color} isBot={r.isBot} />
                {r.quit && <span className="muted">{t('ranking.inactive')}</span>}
              </td>
              <td data-label={label}>{formatNumber(r[key])}</td>
              {tab !== 'points' && <td data-label={t('ranking.points')}>{formatNumber(r.points)}</td>}
            </tr>
          ))}
        </tbody>
      </table>
    );
  } else if (tab === 'daily') {
    const sorted = [...data.daily].sort((a, b) => b[dailyKind] - a[dailyKind]).filter((r) => r[dailyKind] > 0);
    total = sorted.length;
    const label = t(`ranking.daily.${dailyKind}`);
    body = sorted.length === 0 ? <p className="ranking-empty">{t('ranking.nobodyToday')}</p> : (
      <table className="ranking-table">
        <thead><tr><th>#</th><th>{t('ranking.player')}</th><th>{label}</th></tr></thead>
        <tbody>
          {sorted.slice(page * PAGE, page * PAGE + PAGE).map((r, i) => (
            <tr key={r.id} className={host.isMe(r.id) ? 'ranking-me' : undefined}>
              <td data-label="#">{page * PAGE + i + 1}</td>
              <td data-label={t('ranking.player')}><Player host={host} id={r.id} name={r.name} isBot={r.isBot} /></td>
              <td data-label={label}>{formatNumber(r[dailyKind])}</td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  } else {
    total = data.conquests.length;
    body = data.conquests.length === 0 ? <p className="ranking-empty">{t('ranking.noConquests')}</p> : (
      <table className="ranking-table">
        <thead><tr><th>{t('ranking.when')}</th><th>{t('ranking.village')}</th><th>{t('ranking.newOwner')}</th><th>{t('ranking.previous')}</th></tr></thead>
        <tbody>
          {data.conquests.slice(page * PAGE, page * PAGE + PAGE).map((c, i) => (
            <tr key={i} className={host.isMe(c.toId) || (c.fromId !== null && host.isMe(c.fromId)) ? 'ranking-me' : undefined}>
              <td data-label={t('ranking.when')}>{host.formatTime(c.t)}</td>
              <td data-label={t('ranking.village')}>{formatCoords(c.x, c.y)}</td>
              <td data-label={t('ranking.newOwner')}><Player host={host} id={c.toId} name={c.to} isBot={c.toIsBot} /></td>
              <td data-label={t('ranking.previous')}>
                {c.fromId ? <>{t('ranking.from')} <Player host={host} id={c.fromId} name={c.from!} isBot={c.fromIsBot} /></> : t('ranking.barbarian')}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  return (
    <div className="ranking">
      <div className="ranking-tabs">
        <h2>{title ?? t('ranking.title')}</h2>
        {TABS.map((id) => <button key={id} className="ranking-btn" aria-pressed={tab === id} onClick={() => choose(id)}>{t(`ranking.tab.${id}`)}</button>)}
      </div>
      {tab === 'od' && <Kinds kinds={OD_KINDS} value={odKind} onPick={pick(setOdKind)} label={(k) => t(`ranking.odKind.${k}`)} />}
      {tab === 'daily' && <Kinds kinds={DAILY_KINDS} value={dailyKind} onPick={pick(setDailyKind)} label={(k) => t(`ranking.dailyKind.${k}`)} />}
      {host.refreshNote && <p className="ranking-note">{host.refreshNote}</p>}
      <div className="ranking-scroll">{body}</div>
      <Pager page={page} pages={Math.ceil(total / PAGE)} setPage={setPage} />
    </div>
  );
}
