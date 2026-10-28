import { useRef, useState, type SubmitEvent } from 'react';
import { useT } from '../../i18n/i18n';
import { errorMessage, worldDisplayName } from '../../i18n/names';
import { api, ApiError } from '../../net/api';
import { BUILD_VERSION, reportTextOk, type ReportInput, type ReportKind } from '../../net/report';
import { ReportFields } from '../../shared/report/ReportFields';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { Overlay } from '../instant/Overlay';

/** Report a problem from inside the game; the world, the screen, the build and the browser go along, shown first. */
export function ReportDialog() {
  const t = useT();
  const open = useUiStore((s) => s.reportOpen);
  const close = useUiStore((s) => s.setReportOpen);
  const view = useUiStore((s) => s.view);
  const worldId = useGameStore((s) => s.worldId);
  const worldName = useGameStore((s) => worldDisplayName({ name: s.worldName, gameNumber: s.gameNumber }));
  const [kind, setKind] = useState<ReportKind>('bug');
  const [text, setText] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState('');
  const sending = useRef(false);
  if (!open) return null;

  const userAgent = navigator.userAgent;
  const dismiss = () => {
    close(false);
    setSent(false);
    setText('');
    setEmail('');
    setError('');
  };
  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (sending.current || !reportTextOk(text)) return;
    sending.current = true;
    setBusy(true);
    setError('');
    try {
      const input: ReportInput = { kind, text: text.trim(), ...(email.trim() ? { email: email.trim() } : {}), ...(worldId ? { worldId } : {}),
        screen: view, build: BUILD_VERSION, userAgent };
      await api.post<{ id: number }>('/api/v1/reports', input);
      setSent(true);
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.code, err.params) : t('errors.unknown'));
    } finally {
      sending.current = false;
      setBusy(false);
    }
  };

  return (
    <Overlay titleId="report-title">
      <div className="p-3 space-y-2">
        <h2 id="report-title" className="text-lg">{t('report.button')}</h2>
        {sent ? <p role="status">{t('report.sent')}</p> : (
          <form className="space-y-2" onSubmit={submit}>
            <ReportFields kind={kind} text={text} email={email} onKind={setKind} onText={setText} onEmail={setEmail}
              fieldClass="flex flex-col gap-1" />
            <p className="text-[12px] font-semibold">{t('report.attached')}</p>
            <ul className="text-[12px] text-muted" aria-label={t('report.attached')}>
              <li>{t('report.world')}: {worldName}</li>
              <li>{t('report.screen')}: {t(`nav.${view}.label`)}</li>
              <li>{t('report.build')}: {BUILD_VERSION}</li>
              <li className="break-all">{t('report.browser')}: {userAgent}</li>
            </ul>
            {error && <p className="text-bad" role="alert">{error}</p>}
            <button type="submit" className="btn" disabled={busy || !reportTextOk(text)}>{t(busy ? 'report.sending' : 'report.send')}</button>
          </form>
        )}
        <button type="button" className="btn" onClick={dismiss}>{t('report.close')}</button>
      </div>
    </Overlay>
  );
}
