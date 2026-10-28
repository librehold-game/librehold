/** Blocker kinds joined for developer-facing error messages; players read them through the i18n catalogues. */
export function blockerKinds(blockers: readonly { kind: string }[]): string {
  return blockers.map((b) => b.kind).join(', ');
}
