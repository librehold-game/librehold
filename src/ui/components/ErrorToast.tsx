import { useEffect } from 'react';
import { useGameStore } from '../../state/gameStore';

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
    <div className="fixed bottom-4 right-4 flex flex-col gap-2 items-end">
      {notice && (
        <div className="panel px-3 py-2 text-ok font-semibold shadow-lg cursor-pointer" onClick={clearNotice}>{notice}</div>
      )}
      {error && (
        <div className="panel px-3 py-2 text-bad font-semibold shadow-lg cursor-pointer" onClick={clear}>{error}</div>
      )}
    </div>
  );
}
