import { useEffect, useState } from 'react';
import { HUMAN_ID } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import type { ConquestRow, PlayerRow, Profile } from '../../net/types';
import { useRankings } from '../views/RankingView';
import { useUiStore } from '../../state/uiStore';
import { formatCoords, formatNumber } from '../format';
import { useClock } from '../useClock';
import { t, useT } from '../../i18n/i18n';
import { villageName } from '../../i18n/names';
import { BotTag } from './BotTag';

interface ProfileData {
  name: string; color: string; status: string; rank: number; points: number; isHuman: boolean; isBot: boolean;
  villages: { id: string; name: string; x: number; y: number; points: number }[];
  conquests: { t: number; label: string; gained: boolean }[];
  daily: number[];
}

function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return <span className="text-muted italic">{t('profile.tooFewData')}</span>;
  const W = 220, H = 40;
  const max = Math.max(1, ...values);
  const pts = values.map((v, i) => `${((i / (values.length - 1)) * W).toFixed(1)},${(H - (v / max) * (H - 4) - 2).toFixed(1)}`).join(' ');
  return <svg width={W} height={H} aria-label={t('profile.sparkline')}><polyline points={pts} fill="none" stroke="#4c8f2f" strokeWidth="2" /></svg>;
}

function profileData(p: Profile, playerId: string, players: PlayerRow[], conquests: ConquestRow[]): ProfileData {
  const ranked = [...players].sort((a, b) => b.points - a.points);
  return {
    name: p.player.name, color: p.player.color, isHuman: playerId === HUMAN_ID, isBot: p.player.isBot, points: p.player.points,
    status: p.player.status === 'quit' ? t('profile.statusQuit') : p.player.status === 'eliminated' ? t('profile.statusEliminated') : '',
    rank: ranked.findIndex((x) => x.id === playerId) + 1,
    villages: p.villages.map((v) => ({ ...v, name: villageName(v, p.player.name) })),
    conquests: conquests.filter((c) => c.toId === playerId || c.fromId === playerId).slice(0, 20)
      .map((c) => ({ t: c.t, gained: c.toId === playerId, label: `${formatCoords(c.x, c.y)} — ${c.toId === playerId ? t('profile.from', { name: c.from ?? t('profile.barbarians') }) : t('profile.took', { name: c.to })}` })),
    daily: p.history.daily,
  };
}

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
  const json = profile ? JSON.stringify(profileData(profile, playerId, players, conquests)) : '';
  const goToMap = useUiStore((s) => s.goToMap);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  if (!json) return null;
  const d = JSON.parse(json) as ProfileData;
  return (
    <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-2" onClick={onClose}>
      <div role="dialog" aria-label={t('profile.dialog', { name: d.name })} className="panel w-full max-w-[520px] max-h-[85vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="panel-header flex items-center gap-2">
          <span className="inline-block w-3 h-3 rounded-sm border border-ink" style={{ background: d.color }} />
          <span>{d.name}{d.isHuman ? t('profile.you') : ''}{d.isBot && <BotTag />}</span>
          {d.status && <span className="text-muted">· {d.status}</span>}
          <button className="btn !px-1.5 !py-0 ml-auto" onClick={onClose}>✕</button>
        </div>
        <div className="p-3 space-y-2 text-[12px]">
          <div>{t('profile.rank')}: <strong>{d.rank || '—'}</strong> · {t('profile.points')}: <strong>{formatNumber(d.points)}</strong> · {t('profile.villages')}: <strong>{d.villages.length}</strong></div>
          <Sparkline values={d.daily} />
          <h3 className="text-[14px]">{t('profile.villagesTitle')}</h3>
          {d.villages.length === 0 ? <p className="text-muted italic">{t('profile.none')}</p> : (
            <table className="table-game table-stack w-full">
              <thead><tr><th>{t('profile.colVillage')}</th><th>{t('profile.colCoords')}</th><th>{t('profile.colPoints')}</th></tr></thead>
              <tbody>
                {d.villages.map((v) => (
                  <tr key={v.id}>
                    <td data-label={t('profile.colVillage')}>{v.name}</td>
                    <td data-label={t('profile.colCoords')}><button className="underline decoration-dotted" onClick={() => { goToMap(v.x, v.y); onClose(); }}>{formatCoords(v.x, v.y)}</button></td>
                    <td data-label={t('profile.colPoints')} className="tabular-nums">{formatNumber(v.points)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <h3 className="text-[14px]">{t('profile.conquests')}</h3>
          {d.conquests.length === 0 ? <p className="text-muted italic">{t('profile.none')}</p> : (
            <ul className="space-y-0.5">
              {d.conquests.map((c, i) => <li key={i} className={c.gained ? 'text-ok' : 'text-bad'}>{clock.at(c.t)} · {c.label}</li>)}
            </ul>
          )}
          {d.isHuman && playerId === HUMAN_ID && <p className="text-muted italic">{t('profile.othersSee')}</p>}
        </div>
      </div>
    </div>
  );
}
