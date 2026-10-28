import { useRef, useState, type SubmitEvent } from 'react';
import { api, errorText } from './api';
import { Head } from './Head';
import type { PageProps } from './pages';
import { reportTextOk, type ReportKind } from '../net/report';
import { ReportFields } from '../shared/report/ReportFields';
import { useT } from '../i18n/i18n';

/** Anyone may write to the admin here: a bug, a player, a question. */
export function ReportPage({ me }: PageProps) {
  const t = useT();
  const [kind, setKind] = useState<ReportKind>('bug');
  const [text, setText] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState('');
  const sending = useRef(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (sending.current || !reportTextOk(text)) return;
    sending.current = true;
    setBusy(true);
    setError('');
    try {
      await api.report({ kind, text: text.trim(), ...(email.trim() ? { email: email.trim() } : {}) });
      setSent(true);
    } catch (err) {
      setError(errorText(err));
    } finally {
      sending.current = false;
      setBusy(false);
    }
  };

  return (
    <section className="card narrow report-form">
      <Head title={t('head.report.title')} description={t('head.report.description')} noIndex />
      <h1>{t('report.button')}</h1>
      {sent ? <p role="status">{t('report.sent')}</p> : (
        <form onSubmit={submit}>
          <p>{t('report.intro')}</p>
          <p className="muted">{me ? t('report.asAccount', { name: me.username }) : t('report.asVisitor')}</p>
          <ReportFields kind={kind} text={text} email={email} onKind={setKind} onText={setText} onEmail={setEmail} />
          {error && <p className="error" role="alert">{error}</p>}
          <button type="submit" className="cta" disabled={busy || !reportTextOk(text)}>{t(busy ? 'report.sending' : 'report.send')}</button>
        </form>
      )}
    </section>
  );
}

export default ReportPage;
