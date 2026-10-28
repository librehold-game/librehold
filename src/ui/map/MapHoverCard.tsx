import type { MapHover } from './viewport';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { RESOURCES } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { useUiStore, type RallyPrefill } from '../../state/uiStore';
import { ResourceIcon } from '../components/ResourceIcon';
import { UnitCountList } from '../components/UnitCountList';
import { formatNumber } from '../format';
import { useClock } from '../useClock';
import { bonusLabel, villageInfo, type VillageInfo } from './villageInfo';
import { useT } from '../../i18n/i18n';
import { BotTag } from '../components/BotTag';

/** The tile under the cursor and its rectangle on the map surface (px). */
export type { MapHover } from './viewport';

const CARD_W = 260;
const GAP = 6;
/**
 * How long the card waits for the cursor to come back to the tile or enter the card before it hides or switches to another
 * tile — on its way to the card the cursor passes over neighbouring tiles.
 */
const CLOSE_DELAY_MS = 200;

const sameTile = (a: MapHover | null, b: MapHover | null) => a !== null && b !== null && a.x === b.x && a.y === b.y;

/** Kontynent: K + setki y + setki x (107|87 → K01). */
export const continent = (x: number, y: number) => `K${Math.floor(y / 100)}${Math.floor(x / 100)}`;

/**
 * Card shown when hovering a village: name, coordinates and continent, points, owner,
 * then what the player knows — last attack with loot, troops from scouting, distance; for an own village resources, population
 * and incoming attacks. It sits next to the tile and stays open while the cursor is on the tile or the card, so its
 * buttons can be clicked: attack, support and scout straight from the map. The card holds its own state (`register`), so
 * mouse movement does not redraw the map tiles.
 */
export function MapHoverCard({ register, bounds }: { register: (set: (h: MapHover | null) => void) => void; bounds: { width: number; height: number } }) {
  const [hover, setHover] = useState<MapHover | null>(null);
  const shown = useRef<MapHover | null>(null);
  const visible = useRef(false);
  const overCard = useRef(false);
  const next = useRef<MapHover | null>(null);
  const closing = useRef<ReturnType<typeof setTimeout> | null>(null);

  const show = useCallback((h: MapHover | null) => {
    shown.current = h;
    setHover((prev) => (prev && h && sameTile(prev, h) && prev.left === h.left && prev.top === h.top ? prev : h));
  }, []);
  const cancelClose = () => {
    if (closing.current) clearTimeout(closing.current);
    closing.current = null;
  };
  /** The tile changes only CLOSE_DELAY_MS after leaving it; the timer does not restart on every tile passed. */
  const scheduleChange = useCallback((h: MapHover | null) => {
    next.current = h;
    if (closing.current) return;
    closing.current = setTimeout(() => {
      closing.current = null;
      if (!overCard.current) show(next.current);
    }, CLOSE_DELAY_MS);
  }, [show]);
  const fromMap = useCallback((h: MapHover | null) => {
    if (h && (!visible.current || sameTile(h, shown.current))) {
      if (closing.current) clearTimeout(closing.current);
      closing.current = null;
      show(h);
    } else {
      scheduleChange(h);
    }
  }, [show, scheduleChange]);
  useEffect(() => register(fromMap), [register, fromMap]);
  useEffect(() => () => { if (closing.current) clearTimeout(closing.current); }, []);

  const json = useGameStore((s) => (hover && s.game ? JSON.stringify(villageInfo(s.game, hover.x, hover.y, s.game.activeVillageId, s.intel)) : 'null'));
  const info = useMemo(() => JSON.parse(json) as VillageInfo | null, [json]);
  useEffect(() => { visible.current = hover !== null && info !== null; }, [hover, info]);
  const activeId = useGameStore((s) => s.game?.activeVillageId ?? '');
  const openRally = useUiStore((s) => s.openRally);
  const setView = useUiStore((s) => s.setView);
  const setActive = useGameStore((s) => s.setActiveVillage);
  const clock = useClock();
  const t = useT();
  if (!hover || !info) return null;

  const close = () => {
    overCard.current = false;
    cancelClose();
    show(null);
  };
  const rally = (p: RallyPrefill) => {
    close();
    openRally(p);
  };

  let left = hover.left + hover.tw + GAP;
  if (left + CARD_W > bounds.width) left = Math.max(0, hover.left - GAP - CARD_W);
  const below = hover.top < bounds.height / 2;
  const style = below ? { left, top: Math.max(0, hover.top) } : { left, bottom: Math.max(0, bounds.height - hover.top - hover.th) };
  const rows: [string, React.ReactNode][] = [[t('mapCard.points'), formatNumber(info.points)]];
  const actions: [string, () => void][] = [];

  if (info.kind === 'own') {
    rows.push([t('mapCard.player'), t('mapCard.you')]);
    rows.push([t('mapCard.resources'), <span className="inline-flex gap-2">{RESOURCES.map((r) => <span key={r} className="inline-flex items-center gap-0.5"><ResourceIcon r={r} />{formatNumber(info.resources[r])}</span>)}</span>]);
    rows.push([t('mapCard.farm'), `${formatNumber(info.popUsed)} / ${formatNumber(info.popMax)}`]);
    if (info.unitsTotal > 0) rows.push([t('mapCard.army'), <UnitCountList units={info.unitsHome} />]);
    if (info.incoming.count > 0) rows.push([t('mapCard.incoming'), <span className="text-bad font-semibold">{info.incoming.count}{info.incoming.firstAt !== null ? t('mapCard.firstAt', { at: clock.at(info.incoming.firstAt) }) : ''}</span>]);
    if (info.id !== activeId) actions.push([t('mapCard.support'), () => rally({ x: info.x, y: info.y, mode: 'support' })]);
    actions.push([t('mapCard.village'), () => { close(); setActive(info.id); setView('village'); }]);
  } else {
    rows.push([t('mapCard.player'), info.ownerName ? <>{info.ownerPoints !== null ? t('mapCard.ownerPoints', { name: info.ownerName, points: formatNumber(info.ownerPoints) }) : info.ownerName}{info.ownerIsBot && <BotTag />}</> : t('mapCard.barbarians')]);
    if (info.bonus) rows.push([t('mapCard.bonus'), bonusLabel(info.bonus.kind)]);
    const last = info.reports[0];
    if (last) {
      const dot = last.outcome === 'attackerWon' ? (last.ours ? 'bg-ok' : 'bg-bad') : (last.ours ? 'bg-bad' : 'bg-ok');
      const haul = last.haul !== null ? t('mapCard.loot', { haul: `${formatNumber(last.haul)}${last.haulCapacity ? ` / ${formatNumber(last.haulCapacity)}` : ''}` }) : '';
      rows.push([t('mapCard.lastAttack'), <span className="inline-flex items-center gap-1"><span className={`inline-block w-2 h-2 rounded-full ${dot}`} />{clock.at(last.time)}{haul}</span>]);
    }
    if (info.intel) {
      rows.push([t('mapCard.scouted', { day: clock.day(info.intel.at) }), Object.keys(info.intel.units).length ? <UnitCountList units={info.intel.units} /> : t('mapCard.none')]);
      if (info.intel.wall > 0) rows.push([t('mapCard.wall'), t('mapCard.wallLevel', { n: info.intel.wall })]);
    }
    rows.push([t('mapCard.distance'), t('mapCard.fields', { n: info.distance.toFixed(1) })]);
    actions.push([t('mapCard.attack'), () => rally({ x: info.x, y: info.y, mode: 'attack' })]);
    if (info.ownerId) actions.push([t('mapCard.support'), () => rally({ x: info.x, y: info.y, mode: 'support' })]);
    actions.push([t('mapCard.scout'), () => rally({ x: info.x, y: info.y, mode: 'attack', units: { scout: 1 } })]);
  }

  return (
    <div
      role="tooltip"
      className="map-card absolute z-30 text-[11px] shadow-lg"
      style={{ ...style, width: CARD_W }}
      onPointerEnter={() => { overCard.current = true; cancelClose(); }}
      onPointerLeave={() => { overCard.current = false; scheduleChange(null); }}
      onPointerMove={(e) => e.stopPropagation()}
      onPointerDown={(e) => e.stopPropagation()}
      onClick={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      <table className="table-game w-full">
        <thead><tr><th colSpan={2} className="text-left">{info.name} ({info.x}|{info.y}) {continent(info.x, info.y)}</th></tr></thead>
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k}><td className="text-muted pr-2 whitespace-nowrap align-top">{k}:</td><td className="tabular-nums">{v}</td></tr>
          ))}
          <tr>
            <td colSpan={2} data-label={t('mapCard.actions')}>
              <div className="flex flex-wrap gap-1">
                {actions.map(([label, act]) => <button key={label} className="btn !py-0" onClick={act}>{label}</button>)}
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
