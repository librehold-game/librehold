import { useEffect, useState } from 'react';
import { HUMAN_ID } from '../../engine/types';
import { PixelIcon } from '../pixelIcons';
import { useT } from '../../i18n/i18n';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import type { PlayerRow } from '../../net/types';
import { formatDuration } from '../format';

function Nickname({ name }: { name: string }) {
  const t = useT();
  const rename = useGameStore((s) => s.renameGuest);
  const [draft, setDraft] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  if (draft === null) {
    return <button className="btn !px-1.5 !py-0" title={t('instant.renameHint')} onClick={() => setDraft(name)}>{name}</button>;
  }
  return (
    <form className="inline-flex items-center gap-1" onSubmit={async (e) => {
      e.preventDefault();
      const err = await rename(draft.trim());
      setError(err);
      if (!err) setDraft(null);
    }}>
      <input autoFocus aria-label={t('instant.nickname')} className="px-1 border border-wood-border rounded bg-white/60 w-32"
        value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={(e) => { if (e.key === 'Escape') setDraft(null); }} />
      {error && <span role="alert" className="text-bad">{error}</span>}
    </form>
  );
}

/** The player's place in the points ranking (bots count too, as in the ranking table), refetched when the ranking changes. */
function usePlace(): { place: number; of: number } | null {
  const rev = useGameStore((s) => s.rankingRev);
  const load = useGameStore((s) => s.loadRanking);
  const [place, setPlace] = useState<{ place: number; of: number } | null>(null);
  useEffect(() => {
    let live = true;
    void load('points').then((rows) => {
      if (!live || !rows) return;
      const sorted = [...(rows as PlayerRow[])].sort((a, b) => b.points - a.points);
      const i = sorted.findIndex((r) => r.id === HUMAN_ID);
      setPlace(i < 0 ? null : { place: i + 1, of: sorted.length });
    });
    return () => { live = false; };
  }, [rev, load]);
  return place;
}

function Place() {
  const t = useT();
  const place = usePlace();
  const open = useUiStore((s) => s.setView);
  if (!place) return null;
  return (
    <button className="hover:underline" title={t('instant.placeHint')} onClick={() => open('ranking')}>
      <PixelIcon name="nav-ranking" size={16} /> {t('instant.place')}: <strong>{place.place} / {place.of}</strong>
    </button>
  );
}

/** An instant world's strip under the header: time left, the place in the ranking, the guest's nickname and account, and the way out. */
export function InstantBar() {
  const t = useT();
  const endsAt = useGameStore((s) => s.instant?.endsAt ?? null);
  const now = useGameStore((s) => s.game?.now ?? 0);
  const account = useGameStore((s) => s.account);
  const openClaim = useGameStore((s) => s.openClaim);
  const leave = useGameStore((s) => s.leaveWorld);
  const resultsClosed = useGameStore((s) => !!s.instant?.ended && s.resultsHidden);
  const showResults = useGameStore((s) => s.setResultsHidden);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (endsAt === null) return null;
  return (
    <div className="px-2 sm:px-3 py-1 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-wood-border font-body text-[12px]">
      <span><PixelIcon name="hourglass" size={16} /> {t('instant.timeLeft')}: <strong role="timer" aria-label={t('instant.timeLeft')}>{formatDuration(Math.max(0, endsAt - now))}</strong></span>
      <Place />
      {resultsClosed && <button className="btn !py-0" onClick={() => showResults(false)}>{t('instant.results')}</button>}
      {account?.isGuest && <Nickname name={account.username} />}
      {account?.isGuest && <button className="btn !py-0" onClick={openClaim}>{t('instant.createAccount')}</button>}
      <span className="ml-auto flex items-center gap-1">
        {confirming ? (
          <>
            <span>{t('instant.leaveConfirm')}</span>
            <button className="btn !py-0" onClick={async () => setError(await leave())}>{t('instant.leaveYes')}</button>
            <button className="btn !py-0" onClick={() => setConfirming(false)}>{t('instant.leaveNo')}</button>
          </>
        ) : (
          <button className="btn !py-0" onClick={() => setConfirming(true)}>{t('instant.leave')}</button>
        )}
        {error && <span role="alert" className="text-bad">{error}</span>}
      </span>
    </div>
  );
}
