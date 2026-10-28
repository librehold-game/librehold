import { useId } from 'react';
import { useT } from '../../i18n/i18n';
import { REPORT_KIND_KEYS, REPORT_KINDS, REPORT_TEXT_MAX, REPORT_TEXT_MIN, type ReportKind } from '../../net/report';
import '../../i18n/reportCatalog';

/** The fields of a problem report, for the site's page and the game's dialog; each passes its own field class. */
export function ReportFields({ kind, text, email, onKind, onText, onEmail, fieldClass = 'field' }: {
  kind: ReportKind; text: string; email?: string; fieldClass?: string;
  onKind: (k: ReportKind) => void; onText: (t: string) => void; onEmail?: (e: string) => void;
}) {
  const t = useT();
  const id = useId();
  return (
    <>
      <div className={fieldClass}>
        <label htmlFor={`${id}-kind`}>{t('report.kind')}</label>
        <select id={`${id}-kind`} value={kind} onChange={(e) => onKind(e.target.value as ReportKind)}>
          {REPORT_KINDS.map((k) => <option key={k} value={k}>{t(REPORT_KIND_KEYS[k])}</option>)}
        </select>
      </div>
      <div className={fieldClass}>
        <label htmlFor={`${id}-text`}>{t('report.text')}</label>
        <textarea id={`${id}-text`} value={text} maxLength={REPORT_TEXT_MAX} rows={6} onChange={(e) => onText(e.target.value)} />
      </div>
      <p className="muted">
        {t('report.count', { n: [...text.trim()].length, max: REPORT_TEXT_MAX })} · {t('report.hint', { min: REPORT_TEXT_MIN })}
      </p>
      {onEmail && (
        <div className={fieldClass}>
          <label htmlFor={`${id}-email`}>{t('report.email')}</label>
          <input id={`${id}-email`} type="email" value={email ?? ''} onChange={(e) => onEmail(e.target.value)} autoComplete="email" />
        </div>
      )}
    </>
  );
}
