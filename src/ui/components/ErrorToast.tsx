import { useEffect } from 'react';
import { useGameStore } from '../../state/gameStore';
import { RichText } from './RichText';

export function ErrorToast() {
  const error = useGameStore((s) => s.lastError);
  const clear = useGameStore((s) => s.clearError);
  const notice = useGameStore((s) => s.notice);
  const clearNotice = useGameStore((s) => s.clearNotice);
  useEffect(() => {
    if (!error) return;
    const t = window.setTimeout(clear, 4000);
    return () => window.clearTimeout(t);
  }, [error, clear]);
  useEffect(() => {
    if (!notice) return;
    const t = window.setTimeout(clearNotice, 8000);
    return () => window.clearTimeout(t);
  }, [notice, clearNotice]);
  return (
    // On a phone the tab bar is fixed to the bottom edge, so a toast at `bottom-4` is half covered by it: the same
    // offset the page padding and the More menu use keeps the message clear of the bar. `max-md` is exactly where
    // the bar exists, since `Nav` hides it from `md` up.
    <div className="fixed bottom-4 max-md:bottom-[calc(62px+env(safe-area-inset-bottom,0px))] right-4 flex flex-col gap-2 items-end">
      {notice && (
        <div className="panel px-3 py-2 text-ok font-semibold shadow-lg cursor-pointer" onClick={clearNotice}><RichText text={notice} /></div>
      )}
      {error && (
        <div className="panel px-3 py-2 text-bad font-semibold shadow-lg cursor-pointer" onClick={clear}>{error}</div>
      )}
    </div>
  );
}
