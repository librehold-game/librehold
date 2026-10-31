import type { ReactNode } from 'react';
import { formatCoords, formatNumber } from '../../i18n/format';
import { t, useT } from '../../i18n/i18n';
import { villageName } from '../../i18n/names';
import './catalog';
import type { ConquestRow, Profile } from './types';

const CONQUESTS_SHOWN = 20;

export interface ProfileViewData {
  name: string; color: string; status: string; rank: number; points: number; isBot: boolean; isMe: boolean;
  villages: { id: string; name: string; x: number; y: number; points: number }[];
  conquests: { t: number; label: string; gained: boolean }[];
  daily: number[];
}

/** What the profile shows: `conquests` may hold anyone's, only this player's are kept, newest first as given. */
export function profileViewData(p: Profile, rank: number, conquests: ConquestRow[], meId?: string): ProfileViewData {
  const id = p.player.id;
  return {
    name: p.player.name, color: p.player.color, isBot: p.player.isBot, points: p.player.points, rank, isMe: meId === id,
    status: p.player.status === 'quit' ? t('profile.statusQuit') : p.player.status === 'eliminated' ? t('profile.statusEliminated') : '',
    villages: p.villages.map((v) => ({ ...v, name: villageName(v, p.player.name) })),
    conquests: conquests.filter((c) => c.toId === id || c.fromId === id).slice(0, CONQUESTS_SHOWN)
      .map((c) => ({
        t: c.t, gained: c.toId === id,
        label: `${formatCoords(c.x, c.y)} — ${c.toId === id ? t('profile.from', { name: c.from ?? t('profile.barbarians') }) : t('profile.took', { name: c.to })}`,
      })),
    daily: p.history.daily,
  };
}

export interface ProfileHost {
  renderCoords(x: number, y: number): ReactNode;
  formatTime(ms: number): string;
}

function Sparkline({ values }: { values: number[] }) {
  const t = useT();
  if (values.length < 2) return <span className="ranking-empty">{t('profile.tooFewData')}</span>;
  const W = 220, H = 40;
  const max = Math.max(1, ...values);
  const pts = values.map((v, i) => `${((i / (values.length - 1)) * W).toFixed(1)},${(H - (v / max) * (H - 4) - 2).toFixed(1)}`).join(' ');
  return <svg width={W} height={H} aria-label={t('profile.sparkline')}><polyline points={pts} fill="none" stroke="#4c8f2f" strokeWidth="2" /></svg>;
}

/** Who the profile is: colour, name, "you" and bot marks, and whether they have left the game. */
export function ProfileHeading({ data }: { data: ProfileViewData }) {
  const t = useT();
  return (
    <>
      <span className="profile-swatch" style={{ background: data.color }} />
      <span>{data.name}{data.isMe ? t('profile.you') : ''}{data.isBot && <span className="bot-tag">{t('ranking.bot')}</span>}</span>
      {data.status && <span className="muted">· {data.status}</span>}
    </>
  );
}

/** Rank and points, the points history, the villages and the conquests of one player. */
export function ProfileBody({ data, host }: { data: ProfileViewData; host: ProfileHost }) {
  const t = useT();
  return (
    <div className="profile-body">
      <div>
        {t('profile.rank')}: <strong>{data.rank || '—'}</strong> · {t('profile.points')}: <strong>{formatNumber(data.points)}</strong> · {t('profile.villages')}: <strong>{data.villages.length}</strong>
      </div>
      <Sparkline values={data.daily} />
      <h3>{t('profile.villagesTitle')}</h3>
      {data.villages.length === 0 ? <p className="ranking-empty">{t('profile.none')}</p> : (
        <table className="ranking-table">
          <thead><tr><th>{t('profile.colVillage')}</th><th>{t('profile.colCoords')}</th><th>{t('profile.colPoints')}</th></tr></thead>
          <tbody>
            {data.villages.map((v) => (
              <tr key={v.id}>
                <td data-label={t('profile.colVillage')}>{v.name}</td>
                <td data-label={t('profile.colCoords')}>{host.renderCoords(v.x, v.y)}</td>
                <td data-label={t('profile.colPoints')}>{formatNumber(v.points)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <h3>{t('profile.conquests')}</h3>
      {data.conquests.length === 0 ? <p className="ranking-empty">{t('profile.none')}</p> : (
        <ul className="profile-conquests">
          {data.conquests.map((c, i) => <li key={i} className={c.gained ? 'profile-gain' : 'profile-loss'}>{host.formatTime(c.t)} · {c.label}</li>)}
        </ul>
      )}
      {data.isMe && <p className="ranking-empty">{t('profile.othersSee')}</p>}
    </div>
  );
}
