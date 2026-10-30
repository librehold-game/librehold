import { BUILDING_ORDER } from '../../data/buildings.data';
import { UNIT_IDS, type BuildingId, type UnitId } from '../../engine/types';
import { pathFor, slugFor } from '../routes';
import { t, type Locale } from '../../i18n/i18n';
import '../../shared/encyclopedia/catalog';

export const UNIT_ORDER: readonly UnitId[] = UNIT_IDS;
export { BUILDING_ORDER };

export const unitPath = (id: UnitId, l: Locale) => pathFor('unit', l, { unit: slugFor('unit', id, l) });
export const buildingPath = (id: BuildingId, l: Locale) => pathFor('building', l, { building: slugFor('building', id, l) });

/** Home › list › entry, for the visible trail and the BreadcrumbList. */
export function trail(locale: Locale, list: 'units' | 'buildings', entry?: { name: string; path: string }) {
  const items = [
    { name: t('encyclopedia.label.home'), path: pathFor('home', locale) },
    { name: t(`encyclopedia.${list}.heading`), path: pathFor(list, locale) },
  ];
  return entry ? [...items, entry] : items;
}

/** The entries either side of one, wrapping round. */
export function neighbours<T>(order: readonly T[], id: T): [T, T] {
  const i = order.indexOf(id);
  return [order[(i - 1 + order.length) % order.length]!, order[(i + 1) % order.length]!];
}
