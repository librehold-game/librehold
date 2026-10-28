import { useCallback, useEffect, useState } from 'react';
import { getLocale, useT } from '../../i18n/i18n';
import '../../i18n/reportCatalog';
import { errorText, type ProblemReport } from '../api';
import { adminApi } from './adminApi';
import { REPORT_KIND_KEYS, type ReportKind } from '../../net/report';

const STATUSES = ['new', 'read', 'done'] as const;

/** Every report sent to the admin, newest first; the admin marks each read or done. */
export function ReportsTab() {
  const t = useT();
  const [list, setList] = useState<ProblemReport[] | null>(null);
  const [error, setError] = useState('');
  const load = useCallback(() => {
    adminApi.problemReports().then(setList, (err) => setError(errorText(err)));
  }, []);
  useEffect(load, [load]);
  const setStatus = async (id: number, status: ProblemReport['status']) => {
    try {
      await adminApi.setProblemReportStatus(id, status);
      load();
    } catch (err) {
      setError(errorText(err));
    }
  };
  const date = new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' });
  if (error) return <p className="error" role="alert">{error}</p>;
  if (!list) return <p className="muted">{t('loading')}</p>;
  if (list.length === 0) return <p className="muted">{t('admin.reports.empty')}</p>;
  return (
    <table className="reports-table">
      <thead>
        <tr>
          {['date', 'kind', 'sender', 'world', 'text', 'email', 'status'].map((k) => <th key={k}>{t(`admin.reports.${k}`)}</th>)}
        </tr>
      </thead>
      <tbody>
        {list.map((r) => (
          <tr key={r.id}>
            <td>{date.format(new Date(r.createdAt))}</td>
            <td>{REPORT_KIND_KEYS[r.kind as ReportKind] ? t(REPORT_KIND_KEYS[r.kind as ReportKind]) : r.kind}</td>
            <td>{r.username || t('admin.reports.visitor')}</td>
            <td>{r.worldName || (r.worldId ?? '')}</td>
            <td>
              <span className="report-text">{r.text}</span>
              {(r.screen || r.build || r.userAgent) && (
                <details>
                  <summary>{t('admin.reports.details')}</summary>
                  <div>{r.screen}</div><div>{r.build}</div><div>{r.userAgent}</div>
                </details>
              )}
            </td>
            <td>{r.email}</td>
            <td>
              <select value={r.status} aria-label={t('admin.reports.status')} onChange={(e) => void setStatus(r.id, e.target.value as ProblemReport['status'])}>
                {STATUSES.map((s) => <option key={s} value={s}>{t(`admin.reports.statuses.${s}`)}</option>)}
              </select>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
