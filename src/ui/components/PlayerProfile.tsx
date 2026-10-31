import { useEffect, useMemo, useState } from 'react';
import { HUMAN_ID } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import type { Profile } from '../../net/types';
import { useRankings } from '../views/RankingView';
import { useUiStore } from '../../state/uiStore';
import { formatCoords } from '../format';
import { useClock } from '../useClock';
import { t, useT } from '../../i18n/i18n';
import { ProfileBody, ProfileHeading, profileViewData, type ProfileHost } from '../../shared/ranking/ProfileBody';

/** Player profile — public server data only: points, rank, villages, conquests, points history. */
export function PlayerProfile({ playerId, onClose }: { playerId: string; onClose: () => void }) {
  useT();
  const clock = useClock();
  const load = useGameStore((s) => s.loadProfile);
  const rev = useGameStore((s) => s.rankingRev);
  const { players, conquests } = useRankings();
  const [profile, setProfile] = useState<Profile | null>(null);
  useEffect(() => {
    let live = true;
    void load(playerId).then((p) => { if (live) setProfile(p); });
    return () => { live = false; };
  }, [playerId, rev, load]);
  const goToMap = useUiStore((s) => s.goToMap);
  const host = useMemo<ProfileHost>(() => ({
    renderCoords: (x, y) => <button className="underline decoration-dotted" onClick={() => { goToMap(x, y); onClose(); }}>{formatCoords(x, y)}</button>,
    formatTime: (ms) => clock.at(ms),
  }), [clock, goToMap, onClose]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  if (!profile) return null;
  const rank = [...players].sort((a, b) => b.points - a.points).findIndex((x) => x.id === playerId) + 1;
  const d = profileViewData(profile, rank, conquests, HUMAN_ID);
  return (
    <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-2" onClick={onClose}>
      <div role="dialog" aria-label={t('profile.dialog', { name: d.name })} className="panel w-full max-w-[520px] max-h-[85vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="panel-header flex items-center gap-2">
          <ProfileHeading data={d} />
          <button className="btn !px-1.5 !py-0 ml-auto" onClick={onClose}>✕</button>
        </div>
        <div className="p-3">
          <ProfileBody data={d} host={host} />
        </div>
      </div>
    </div>
  );
}
