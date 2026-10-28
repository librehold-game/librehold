// Shared by the game and the site: what a problem report holds and how long its text may be. It imports nothing.

export const REPORT_KINDS = ['bug', 'player', 'question', 'other'] as const;
export type ReportKind = (typeof REPORT_KINDS)[number];
/** Each kind's catalogue key. */
export const REPORT_KIND_KEYS: Record<ReportKind, string> = {
  bug: 'report.kindBug', player: 'report.kindPlayer', question: 'report.kindQuestion', other: 'report.kindOther',
};
export const REPORT_TEXT_MIN = 10, REPORT_TEXT_MAX = 2000;

export interface ReportInput {
  kind: ReportKind; text: string; email?: string; worldId?: number; screen?: string; build?: string; userAgent?: string;
}

/** The build the player runs, for the admin reading a bug report. */
export const BUILD_VERSION: string = import.meta.env.VITE_BUILD || 'dev';

/** Counts characters, not bytes, as the server does. */
export function reportTextOk(text: string): boolean {
  const n = [...text.trim()].length;
  return n >= REPORT_TEXT_MIN && n <= REPORT_TEXT_MAX;
}
