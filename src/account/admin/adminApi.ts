import { call, type AdminAccount, type AdminWorld, type ProblemReport } from '../api';
import type { WorldConfig } from '../worldConfig';

/** The admin panel's calls, kept out of the front page's bundle. */
export const adminApi = {
  accounts: () => call<AdminAccount[]>('GET', '/api/v1/admin/accounts'),
  resetPassword: (id: number, password: string) => call<void>('POST', `/api/v1/admin/accounts/${id}/password`, { password }),
  worlds: () => call<AdminWorld[]>('GET', '/api/v1/admin/worlds'),
  createWorld: (name: string, settings: WorldConfig) => call<{ id: number }>('POST', '/api/v1/admin/worlds', { name, settings }),
  updateWorld: (id: number, patch: { settings?: Partial<WorldConfig>; status?: 'open' | 'closed' }) =>
    call<void>('PATCH', `/api/v1/admin/worlds/${id}`, patch),
  pauseWorld: (id: number) => call<void>('POST', `/api/v1/admin/worlds/${id}/pause`),
  resumeWorld: (id: number) => call<void>('POST', `/api/v1/admin/worlds/${id}/resume`),
  problemReports: () => call<ProblemReport[]>('GET', '/api/v1/admin/reports'),
  setProblemReportStatus: (id: number, status: 'new' | 'read' | 'done') =>
    call<void>('PATCH', `/api/v1/admin/reports/${id}`, { status }),
};
