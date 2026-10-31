import { useEffect, useMemo, useState } from 'react';
import { HUMAN_ID } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import type { ConquestRow, DailyRow, PlayerRow } from '../../net/types';
import { useUiStore } from '../../state/uiStore';
import { RankingTable, type RankingData, type RankingHost } from '../../shared/ranking/RankingTable';
import { useClock } from '../useClock';
import { useT } from '../../i18n/i18n';

function PlayerName({ id, name, color }: { id: string; name: string; color?: string }) {
  const open = useUiStore((s) => s.openProfile);
  return (
    <button className="inline-flex items-center gap-1 underline decoration-dotted" onClick={() => open(id)}>
      {color && <span className="inline-block w-2.5 h-2.5 rounded-sm border border-ink" style={{ background: color }} />}
      {name}
    </button>
  );
}

/** Rankings from the server, refetched whenever the world's ranking changes. */
export function useRankings(): RankingData {
  const rev = useGameStore((s) => s.rankingRev);
  const load = useGameStore((s) => s.loadRanking);
  const [data, setData] = useState<RankingData>({ players: [], daily: [], conquests: [] });
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
  const t = useT();
  const clock = useClock();
  const data = useRankings();
  const short = useGameStore((s) => s.instant !== null);
  const host = useMemo<RankingHost>(() => ({
    renderPlayer: (p) => <PlayerName id={p.id} name={p.name} color={p.color} />,
    isMe: (id) => id === HUMAN_ID,
    formatTime: (ms) => clock.at(ms),
    refreshNote: t(short ? 'ranking.refreshShort' : 'ranking.refresh'),
  }), [clock, short, t]);
  return <RankingTable data={data} host={host} />;
}
