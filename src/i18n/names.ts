import { hasKey, t } from './i18n';
import { formatDurationHuman, formatNumber } from './format';

/** Names of game things, worded from the catalogues by their ids. */

export const buildingName = (id: string): string => t(`building.${id}`);

/** An instant game reads "Game #41" in the reader's language; other worlds keep their own name. */
export const worldDisplayName = (w: { name: string; gameNumber?: number | null }): string =>
  (w.gameNumber ? t('world.gameName', { n: w.gameNumber }) : w.name);
export const resourceName = (id: string): string => t(`resource.${id}`);
export const questTitle = (id: string): string => t(`quest.${id}.title`);
export const questDescription = (id: string): string => t(`quest.${id}.description`);

/** A unit's name; with `count`, the form that goes after that number ("5 Spearmen"). */
export function unitName(id: string, count?: number): string {
  return count === undefined ? t(`unit.${id}`) : t(`unitPlural.${id}`, { count });
}

/** A build template's name: the player's own, or the default template's from the catalogue. */
export function templateName(tpl: { id: string; name: string }): string {
  return tpl.name || (hasKey(`template.${tpl.id}`) ? t(`template.${tpl.id}`) : tpl.id);
}

/** A village's name; an empty stored name is the default one, worded from the owner. */
export function villageName(v: { name: string }, ownerName?: string | null): string {
  if (v.name) return v.name;
  return ownerName ? t('village.of', { owner: ownerName }) : t('village.barbarian');
}

export interface ReportVillage { id: string; name: string; x: number; y: number; ownerId?: string; ownerName?: string; ownerIsBot?: boolean }

/** "Name (x|y)" as a report shows a village. */
export function reportVillageLabel(v: ReportVillage): string {
  const coords = `(${v.x}|${v.y})`;
  if (!v.name && !v.ownerName && !v.ownerId) return v.id.startsWith('v:') ? `${t('village.barbarian')} ${coords}` : coords;
  return `${villageName(v, v.ownerName)} ${coords}`;
}

export type Blocker = { kind: string } & Record<string, unknown>;

/** What stands in the way, in the player's words. */
export function blockerText(b: Blocker): string {
  const key = `blocker.${b.kind}`;
  return hasKey(key) ? t(key) : b.kind;
}

export function blockersText(blockers: readonly Blocker[]): string {
  return blockers.map(blockerText).join(', ');
}

/** Values in a message that are ids or raw numbers, turned into words before they are put in. */
function wordParams(params: Record<string, unknown> | undefined): Record<string, string | number> {
  const out: Record<string, string | number> = {};
  for (const [k, v] of Object.entries(params ?? {})) {
    if (k === 'building' && typeof v === 'string') out[k] = buildingName(v);
    else if (k === 'ms' && typeof v === 'number') out.time = formatDurationHuman(v);
    else if (k === 'n' && typeof v === 'number') out[k] = formatNumber(v);
    else if (k === 'blockers' && Array.isArray(v)) out.reasons = blockersText(v as Blocker[]);
    else if (typeof v === 'string' || typeof v === 'number') out[k] = v;
  }
  return out;
}

/** The text of a server error code with its params and blockers. */
export function errorMessage(code: string, params?: Record<string, unknown>, blockers?: readonly Blocker[]): string {
  const key = `errors.${code}`;
  if (!hasKey(key)) return t('errors.unknown');
  return t(key, wordParams(blockers ? { ...params, blockers } : params));
}

export interface NewsLike { key: string; params?: Record<string, unknown> }

/** A news item or rumour in the reader's language. */
export function newsText(item: NewsLike): string {
  const p = { ...(item.params ?? {}) };
  if (item.key.startsWith('gossip.')) {
    for (const k of ['a', 'b']) {
      if (k in p && !p[k]) p[k] = t('gossip.barbarians');
      if (p[k] === '?') p[k] = t('gossip.unknown');
    }
    if ('x' in p && 'y' in p) p.place = p.village ? `${String(p.village)} (${p.x}|${p.y})` : `${p.x}|${p.y}`;
  }
  return t(item.key, wordParams(p));
}
