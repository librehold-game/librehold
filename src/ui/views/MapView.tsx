import { t as tr, useT } from '../../i18n/i18n';
import { villageName } from '../../i18n/names';
import { BotTag } from '../components/BotTag';
import { unitName } from '../../i18n/names';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useShallow } from 'zustand/react/shallow';
import { MAP } from '../../data/world.config';
import { UNITS } from '../../data/units.data';
import { travelTimeMs } from '../../engine/commands';
import { distance, type TileInfo } from '../../engine/world';
import { tileAt } from '../map/tiles';
import { HUMAN_ID, RESOURCES } from '../../engine/types';
import { mapCache, selectActiveVillage, useGameStore, type GameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { formatCoords, formatDuration, formatDurationHuman, formatNumber } from '../format';
import { useClock } from '../useClock';
import { loyaltyAt, noblesNeededFor } from '../../engine/conquest';
import { UnitCountList } from '../components/UnitCountList';
import { PopIcon, ResourceIcon } from '../components/ResourceIcon';
import { Countdown } from '../components/Countdown';
import type { VillageIntel, WorldConfig } from '../../engine/types';
import { playerCommands } from '../../engine/commands';
import { armiesOnMap, type MapArmy } from '../map/armies';
import { gameArmyClock } from '../map/gameArmyClock';
import { TileMap, ZOOM_MAX, type Dot, type TileMapView } from '../map/TileMap';
import { ArmyCard } from '../map/ArmyCard';
import { villageSizeLabel, villageSize } from '../map/MapArt';
import { bonusLabel } from '../map/villageInfo';
import { MapHoverCard, type MapHover } from '../map/MapHoverCard';
import { villageInfo, type VillageInfo } from '../map/villageInfo';
import { isHumanVillage, humanVillageIds } from '../../engine/players';


/** Selector: a compact summary of materialised villages — it changes rarely, so the map doesn't re-render every tick. */
const selectVillageSummary = (s: GameStore): string => {
  const g = s.game;
  if (!g) return '';
  const parts: string[] = [];
  for (const v of Object.values(g.villages)) parts.push(`${v.x},${v.y},${isHumanVillage(v) ? 'p' : 'b'},${v.points},${v.name}`);
  return parts.join(';');
};

/**
 * Tile cache — a Zustand selector must return a stable reference, otherwise React loops.
 * Tiles other than own villages: keyed by map version and coordinates. Own ones: keyed by the fields that affect their look.
 */
const dormantCache = new Map<string, TileInfo>();
const materializedCache = new Map<string, TileInfo>();

function useTile(x: number, y: number): TileInfo {
  return useGameStore((s) => cachedTile(s, x, y));
}

function cachedTile(s: GameStore, x: number, y: number): TileInfo {
  const g = s.game!;
  const id = `v:${x}:${y}`;
  const v = g.villages[id];
  if (v) {
    const key = `${id}|${v.ownerId ?? ''}|${v.points}|${v.name}|${v.bonus?.kind ?? ''}`;
    let t = materializedCache.get(key);
    if (!t) {
      t = tileAt(g, x, y);
      if (materializedCache.size > 20_000) materializedCache.clear();
      materializedCache.set(key, t);
    }
    return t;
  }
  const key = `${s.mapRev}:${x}:${y}`;
  let t = dormantCache.get(key);
  if (!t) {
    t = tileAt(g, x, y);
    if (dormantCache.size > 50_000) dormantCache.clear();
    dormantCache.set(key, t);
  }
  return t;
}

const DOT_PRIORITY: Record<Dot, number> = { t: 1, r: 2, s: 3, a: 4, i: 5 };

/** Command dots in the corner of the target tile: i = incoming AI attack, a = our attack, s = support, r = return, t = trade. */
const selectCommandDots = (s: GameStore): string => {
  const g = s.game;
  if (!g) return '{}';
  const out: Record<string, Dot> = {};
  for (const c of playerCommands(g)) {
    const kind: Dot = c.type === 'attack' ? (humanVillageIds(g).includes(c.originId) ? 'a' : 'i') : c.type === 'support' ? 's' : c.type === 'return' ? 'r' : 't';
    const prev = out[c.targetId];
    if (!prev || DOT_PRIORITY[kind] > DOT_PRIORITY[prev]) out[c.targetId] = kind;
  }
  return JSON.stringify(out);
};

const POPUP_W = 256;
const CMD_ICON = { attack: '⚔', support: '🛡', return: '↩', trade: '🛒' } as const;

/** Small menu next to the clicked village — different for an own one (economy, threats) and a foreign one (intel, reports, movements). */
export function VillagePopup({ sel, anchor, bounds, sheet, onClose, onDetails }: {
  sel: { x: number; y: number };
  anchor: { left: number; top: number; tw: number; th: number };
  bounds: { width: number; height: number };
  /** On a narrow screen the menu doesn't fit next to the tile — it slides up from the bottom at full width. */
  sheet: boolean;
  onClose: () => void;
  onDetails: () => void;
}) {
  const clock = useClock();
  const json = useGameStore((s) => JSON.stringify(villageInfo(s.game!, sel.x, sel.y, s.game!.activeVillageId, s.intel)));
  const openProfile = useUiStore((s) => s.openProfile);
  const info = useMemo(() => JSON.parse(json) as VillageInfo | null, [json]);
  const cfg = useGameStore(useShallow((s) => ({ worldSpeed: s.game!.config.worldSpeed, unitSpeed: s.game!.config.unitSpeed })));
  const nowMin = useGameStore((s) => Math.floor(s.game!.now / 60_000));
  const activeId = useGameStore((s) => s.game!.activeVillageId);
  const tplA = useGameStore((s) => Object.keys(s.game!.farmAssistant.templates.A.units).length > 0);
  const setActive = useGameStore((s) => s.setActiveVillage);
  const sendTemplate = useGameStore((s) => s.sendTemplate);
  const openRally = useUiStore((s) => s.openRally);
  const setView = useUiStore((s) => s.setView);
  const openReport = useUiStore((s) => s.openReport);
  const openReportsFor = useUiStore((s) => s.openReportsFor);
  const ref = useRef<HTMLDivElement>(null);
  const t = useT();
  const [size, setSize] = useState({ w: POPUP_W, h: 160 });
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    if (Math.abs(r.width - size.w) > 1 || Math.abs(r.height - size.h) > 1) setSize({ w: r.width, h: r.height });
  });
  if (!info) return null;

  let left = anchor.left + anchor.tw + 6;
  if (left + size.w > bounds.width - 4) left = anchor.left - size.w - 6;
  if (left < 4) left = 4;
  let top = anchor.top - 4;
  if (top + size.h > bounds.height - 4) top = bounds.height - size.h - 4;
  if (top < 4) top = 4;
  const ago = (t: number) => formatDurationHuman(Math.max(0, nowMin * 60_000 - t));
  const sizeLabel = villageSizeLabel(villageSize(info.points));

  return (
    <div
      ref={ref}
      className={`panel shadow-[0_4px_12px_rgba(0,0,0,0.35)] text-[11px] p-2 leading-snug ${
        sheet ? 'fixed inset-x-1 bottom-[calc(64px+env(safe-area-inset-bottom,0px))] z-40 max-h-[52vh] overflow-y-auto' : 'absolute z-20'
      }`}
      style={sheet ? undefined : { left, top, width: POPUP_W }}
      onPointerDown={(e) => e.stopPropagation()}
      onClick={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="min-w-0">
          <div className="font-heading text-[14px] leading-tight truncate">{info.name}</div>
          <div className="text-muted">
            {formatCoords(info.x, info.y)} · {t('map.pts', { n: formatNumber(info.points) })} · {sizeLabel}
            {info.kind === 'own' && info.active && <span className="text-ok font-semibold">{t('map.active')}</span>}
            {info.kind === 'foreign' && <span> · {info.ownerId ? <><button className="underline decoration-dotted" onClick={() => openProfile(info.ownerId!)}>{info.ownerName}</button>{info.ownerIsBot && <BotTag />}</> : t('map.barbarians')}</span>}
          </div>
        </div>
        <button className="btn !px-1.5 !py-0" onClick={onClose} title={t('map.close')}>✕</button>
      </div>

      {info.kind === 'own' ? (
        <>
          <div className="flex flex-wrap gap-x-2 tabular-nums mt-1">
            {RESOURCES.map((r) => <span key={r} className="inline-flex items-center gap-0.5"><ResourceIcon r={r} size={12} />{formatNumber(info.resources[r])}</span>)}
            <span className="text-muted">/ {formatNumber(info.storage)}</span>
            <span title={t('map.population')} className="inline-flex items-center gap-1"><PopIcon size={14} /> {formatNumber(info.popUsed)}/{formatNumber(info.popMax)}</span>
          </div>
          <div className="mt-0.5">
            🔨 {info.build
              ? <>{info.build.name} {info.build.level} — <Countdown until={info.build.completesAt} />{info.queueLength > 1 && <span className="text-muted">{t('map.queued', { n: info.queueLength - 1 })}</span>}</>
              : <span className="text-muted">{t('map.queueEmpty')}</span>}
          </div>
          <div className="mt-0.5 flex items-center gap-1 flex-wrap">
            <span className="text-muted">{t('map.troops')}</span>
            {info.unitsTotal > 0 ? <UnitCountList units={info.unitsHome} size={12} /> : <span className="text-muted">{t('map.noTroops')}</span>}
            {info.outgoing > 0 && <span className="text-muted">{t('map.commandsOnWay', { count: info.outgoing })}</span>}
          </div>
          {info.incoming.count > 0 && info.incoming.firstAt !== null && (
            <div className="mt-0.5 text-bad font-semibold">{t('map.incomingAttacks', { count: info.incoming.count })} <Countdown until={info.incoming.firstAt} /></div>
          )}
          <div className="flex flex-wrap gap-1 mt-1.5">
            <button className="btn" onClick={() => { setActive(info.id); setView('village'); }}>{t('map.village')}</button>
            {info.id !== activeId && <button className="btn" onClick={() => setActive(info.id)}>{t('map.switch')}</button>}
            {info.id !== activeId && <button className="btn" onClick={() => openRally({ x: info.x, y: info.y, mode: 'support' })}>{t('map.support')}</button>}
            <button className="btn" onClick={() => openReportsFor(info.id)}>{t('map.reports')}{info.reportCount > 0 ? ` (${info.reportCount})` : ''}</button>
            <button className="btn" onClick={onDetails}>{t('map.more')}</button>
          </div>
        </>
      ) : (
        <>
          {info.bonus && <div className="text-gold font-semibold">{tr('map.bonusVillage', { bonus: bonusLabel(info.bonus.kind) })}</div>}
          <div className="mt-1 tabular-nums">
            {t('map.travel', { d: info.distance.toFixed(1), lc: formatDuration(travelTimeMs(info.distance, UNITS.lightCav.speed, cfg.unitSpeed)), axe: formatDuration(travelTimeMs(info.distance, UNITS.axe.speed, cfg.unitSpeed)) })}
          </div>
          {info.intel ? (
            <div className="mt-0.5">
              <span className="text-muted">{t('map.garrison', { ago: ago(info.intel.at) })}</span> <UnitCountList units={info.intel.units} size={12} />
              <span className="text-muted">{t('map.wallLoyalty', { wall: info.intel.wall, loyalty: info.intel.loyalty })}</span>
            </div>
          ) : (
            <div className="mt-0.5 text-muted italic">{t('map.noIntel')}</div>
          )}
          {info.lastAttackAt !== null && (
            <div>
              {t('map.lastAttack', { ago: ago(info.lastAttackAt) })}
              {info.intel?.lastHaul !== null && info.intel?.lastHaul !== undefined && <>{t('map.loot', { n: `${formatNumber(info.intel.lastHaul)}${info.intel.lastHaulCapacity ? `/${formatNumber(info.intel.lastHaulCapacity)}` : ''}` })}</>}
              {info.intel && (info.intel.lastLosses ? <span className="text-bad">{t('map.losses', { n: info.intel.lastLosses })}</span> : <span className="text-ok">{t('map.noLosses')}</span>)}
            </div>
          )}
          {info.threat && <div className="text-bad font-semibold">{t('map.threat', { target: info.threat.targetName, units: info.threat.units })} <Countdown until={info.threat.arrivesAt} /></div>}
          {info.ours.map((c) => (
            <div key={c.id}>{CMD_ICON[c.kind]} {t(`map.cmd.${c.kind}`)}{c.units > 0 ? ` (${c.units})` : ''} {t('map.arrivesIn')} <Countdown until={c.arrivesAt} /></div>
          ))}
          <div className="mt-1 border-t border-wood-border/60 pt-1">
            <div className="flex items-center justify-between">
              <span className="font-semibold">{t('map.reportsCount', { n: info.reportCount })}</span>
              {info.reportCount > 0 && <button className="underline" onClick={() => openReportsFor(info.id)}>{t('map.allReports')}</button>}
            </div>
            {info.reports.length === 0 ? (
              <div className="text-muted italic">{t('map.noReports')}</div>
            ) : info.reports.map((r) => (
              <button key={r.id} className={`block w-full text-left px-1 rounded hover:bg-white/40 ${r.read ? '' : 'font-semibold'}`} onClick={() => openReport(r.id)}>
                {r.icon} {t(`map.r.${r.type === 'scout' || r.type === 'conquest' || r.type === 'support' ? r.type : r.ours ? 'ours' : 'theirs'}`)} · {clock.at(r.time)}{r.haul !== null ? t('map.loot', { n: formatNumber(r.haul) }) : ''}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap gap-1 mt-1.5">
            <button className="btn" onClick={() => openRally({ x: info.x, y: info.y, mode: 'attack' })}>{t('map.attack')}</button>
            {info.ownerId && <button className="btn" onClick={() => openRally({ x: info.x, y: info.y, mode: 'support' })}>{t('map.support')}</button>}
            <button className="btn" onClick={() => openRally({ x: info.x, y: info.y, mode: 'attack', units: { scout: 1 } })}>{t('map.scout')}</button>
            <button className="btn" disabled={!tplA} title={tplA ? t('map.sendA') : t('map.defineA')} onClick={() => sendTemplate('A', info.x, info.y)}>{t('map.templateA')}</button>
            <button className="btn" onClick={onDetails}>{t('map.more')}</button>
          </div>
        </>
      )}
    </div>
  );
}

function SidePanel({ sel, onClose }: { sel: { x: number; y: number }; onClose: () => void }) {
  const clock = useClock();
  const t = useTile(sel.x, sel.y);
  const openProfile = useUiStore((s) => s.openProfile);
  const active = useGameStore(useShallow((s) => { const v = selectActiveVillage(s); return { x: v.x, y: v.y, id: v.id }; }));
  const cfg = useGameStore(useShallow((s) => ({ worldSpeed: s.game!.config.worldSpeed, unitSpeed: s.game!.config.unitSpeed })));
  const now = useGameStore((s) => Math.floor(s.game!.now / 1000));
  const openRally = useUiStore((s) => s.openRally);
  const sendTemplate = useGameStore((s) => s.sendTemplate);
  const tplA = useGameStore((s) => Object.keys(s.game!.farmAssistant.templates.A.units).length > 0);
  const setActive = useGameStore((s) => s.setActiveVillage);
  const d = distance(active.x, active.y, sel.x, sel.y);
  const intelJson = useGameStore((s) => {
    const v = s.game!.villages[`v:${sel.x}:${sel.y}`];
    if (!v || isHumanVillage(v)) return '';
    const nowMs = s.game!.now;
    return JSON.stringify({
      intel: v.intel ?? null,
      lastFarmedAgo: v.lastFarmedAt !== undefined ? nowMs - v.lastFarmedAt : null,
      loyalty: v.intel ? loyaltyAt({ ...v, loyalty: v.intel.loyalty, loyaltyUpdatedAt: v.intel.at }, nowMs, s.game!.config.worldSpeed) : null,
      owned: v.ownerId !== null,
      config: s.game!.config,
    });
  });
  const intel = intelJson ? (JSON.parse(intelJson) as { intel: VillageIntel | null; lastFarmedAgo: number | null; loyalty: number | null; owned: boolean; config: WorldConfig }) : null;

  return (
    <div className="panel p-2 text-[12px] w-full sm:w-[260px] shrink-0">
      <div className="flex items-center justify-between mb-1">
        <h3 className="text-[15px]">{formatCoords(sel.x, sel.y)}</h3>
        <button className="btn !px-1.5" onClick={onClose}>✕</button>
      </div>
      {t.kind === 'empty' ? (
        <p className="text-muted italic">{tr('map.emptyField')}</p>
      ) : (
        <div className="space-y-1">
          <div className="font-semibold">{villageName(t, t.ownerName)}</div>
          <div>{tr('map.owner')} {t.ownerId === null ? tr('map.barbarians') : <><button className="underline decoration-dotted" onClick={() => openProfile(t.ownerId!)}>{t.ownerId === HUMAN_ID ? tr('map.you') : t.ownerName}</button>{t.ownerIsBot && <BotTag />}</>}</div>
          <div>{tr('map.points', { n: formatNumber(t.points) })} <span className="text-muted">({villageSizeLabel(villageSize(t.points))})</span></div>
          {t.bonus && <div className="text-gold font-semibold">{tr('map.bonusVillage', { bonus: bonusLabel(t.bonus.kind) })}</div>}
          <div>{tr('map.distance', { n: d.toFixed(1) })}</div>
          {intel?.intel && (
            <div className="panel p-1.5 text-[11px] space-y-0.5">
              <div className="font-semibold">{tr('map.intelFrom', { source: intel.lastFarmedAgo !== null ? tr('map.intelAttack', { ago: formatDurationHuman(intel.lastFarmedAgo) }) : tr('map.intelScout') })}</div>
              <div>{tr('map.garrisonShort')} <UnitCountList units={intel.intel.units} size={12} /></div>
              <div>{tr('map.wallLoyaltyNobles', { wall: intel.intel.wall, loyalty: intel.loyalty ?? '?' })} <span className="text-muted">{tr('map.noblesLeft', { n: noblesNeededFor(intel.config, intel.owned, intel.loyalty ?? 100) })}</span></div>
              {intel.intel.lastHaul && (
                <div>{tr('map.lastLoot', { n: formatNumber(intel.intel.lastHaul.wood + intel.intel.lastHaul.clay + intel.intel.lastHaul.iron), cap: formatNumber(intel.intel.lastHaulCapacity ?? 0) })}
                  {intel.intel.lastLosses ? <span className="text-bad">{tr('map.losses', { n: intel.intel.lastLosses })}</span> : <span className="text-ok">{tr('map.noLosses')}</span>}
                </div>
              )}
            </div>
          )}
          {d > 0 && (
            <table className="w-full text-[11px] mt-1">
              <tbody>
                {(['lightCav', 'spear', 'sword', 'ram', 'noble'] as const).map((u) => {
                  const ms = travelTimeMs(d, UNITS[u].speed, cfg.unitSpeed);
                  return (
                    <tr key={u}>
                      <td className="text-muted">{unitName(u)}</td>
                      <td className="tabular-nums text-right">{formatDuration(ms)}</td>
                      <td className="tabular-nums text-right text-muted">{clock.at(now * 1000 + ms)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
          <div className="flex flex-wrap gap-1 pt-1">
            {t.ownerId === HUMAN_ID ? (
              <>
                <button className="btn" onClick={() => setActive(t.id)}>{tr('map.switch')}</button>
                {t.id !== active.id && <button className="btn" onClick={() => openRally({ x: sel.x, y: sel.y, mode: 'support' })}>{tr('map.support')}</button>}
              </>
            ) : (
              <>
                <button className="btn" onClick={() => openRally({ x: sel.x, y: sel.y, mode: 'attack' })}>{tr('map.attack')}</button>
                {t.ownerId && <button className="btn" onClick={() => openRally({ x: sel.x, y: sel.y, mode: 'support' })}>{tr('map.support')}</button>}
                <button className="btn" onClick={() => openRally({ x: sel.x, y: sel.y, mode: 'attack', units: { scout: 1 } })}>{tr('map.scout')}</button>
                <button className="btn" disabled={!tplA} title={tplA ? tr('map.sendA') : tr('map.defineA')} onClick={() => sendTemplate('A', sel.x, sel.y)}>{tr('map.templateA')}</button>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}


/** The player's map: their world through the shared tile map, with the village menu, details and army card. */
export function MapView() {
  const active = useGameStore(useShallow((s) => { const v = selectActiveVillage(s); return { x: v.x, y: v.y, id: v.id }; }));
  const mapSize = useGameStore((s) => s.game!.config.mapSize);
  const seed = useGameStore((s) => s.game!.worldSeed);
  const mapRev = useGameStore((s) => s.mapRev);
  const villageSummary = useGameStore(selectVillageSummary);
  const minimapDots = useMemo(() => villageSummary.split(';').filter(Boolean).map((p) => {
    const [x, y, o] = p.split(',');
    return { x: Number(x), y: Number(y), player: o === 'p' };
  }), [villageSummary]);
  const isVillage = useCallback((x: number, y: number) => {
    const g = useGameStore.getState().game;
    return !!g && tileAt(g, x, y).kind === 'village';
  }, []);
  const dotsJson = useGameStore(selectCommandDots);
  const dots = useMemo(() => JSON.parse(dotsJson) as Record<string, Dot>, [dotsJson]);
  const armiesJson = useGameStore((s) => JSON.stringify(armiesOnMap(s.game!)));
  const armies = useMemo(() => JSON.parse(armiesJson) as MapArmy[], [armiesJson]);
  const animations = useUiStore((s) => s.prefs.animations);
  const mapCenter = useUiStore((s) => s.mapCenter);
  const [center, setCenter] = useState<{ x: number; y: number }>(() => mapCenter ?? { x: active.x + 0.5, y: active.y + 0.5 });
  const [zoomIdx, setZoomIdx] = useState<number>(MAP.defaultZoomIdx);
  const [sel, setSel] = useState<{ x: number; y: number } | null>(null);
  const [selArmy, setSelArmy] = useState<string | null>(null);
  const [colors, setColors] = useState(true);
  const [details, setDetails] = useState(false);
  const [maxPoints, setMaxPoints] = useState(99999);
  const [goto, setGoto] = useState('');

  useEffect(() => {
    if (mapCenter) setCenter({ x: mapCenter.x + 0.5, y: mapCenter.y + 0.5 });
  }, [mapCenter]);

  useEffect(() => {
    if (!sel && !selArmy) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') { setSel(null); setDetails(false); setSelArmy(null); } };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sel, selArmy]);
  const closeArmy = useCallback(() => setSelArmy(null), []);

  const loadRegion = useCallback((x0: number, y0: number, x1: number, y1: number) => {
    const cache = mapCache();
    if (!cache) return;
    let live = true;
    void cache.ensure(x0, y0, x1, y1)
      .then((fetched) => { if (live && fetched) useGameStore.getState().mapLoaded(); })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);

  const clampCenter = (c: { x: number; y: number }) => ({
    x: Math.max(0.5, Math.min(mapSize - 0.5, c.x)),
    y: Math.max(0.5, Math.min(mapSize - 0.5, c.y)),
  });
  const closeSel = () => { setSel(null); setDetails(false); };
  const zoomBy = (d: number) => setZoomIdx((z) => Math.max(0, Math.min(ZOOM_MAX, z + d)));

  const setHover = useRef<(h: MapHover | null) => void>(() => {});
  const registerHover = useCallback((set: (h: MapHover | null) => void) => { setHover.current = set; }, []);

  const popup = (view: TileMapView, sheet: boolean) => {
    if (!sel) return null;
    const anchor = { left: Math.round((sel.x - view.x0) * view.tw - view.offX), top: Math.round((sel.y - view.y0) * view.th - view.offY), tw: view.tw, th: view.th };
    const visible = anchor.left > -view.tw && anchor.left < view.width && anchor.top > -view.th && anchor.top < view.height;
    if (!sheet && !visible) return null;
    return <VillagePopup sel={sel} anchor={anchor} bounds={{ width: view.width, height: view.height }} sheet={sheet} onClose={closeSel} onDetails={() => setDetails(true)} />;
  };

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2 mb-2 text-[12px]">
        <h2 className="text-lg mr-2">{tr('map.title')}</h2>
        <button className="btn" onClick={() => setCenter({ x: active.x + 0.5, y: active.y + 0.5 })}>{tr('map.myVillage')}</button>
        <button className="btn" onClick={() => zoomBy(-1)} disabled={zoomIdx === 0}>＋</button>
        <button className="btn" onClick={() => zoomBy(1)} disabled={zoomIdx === ZOOM_MAX}>－</button>
        <form
          className="inline-flex items-center gap-1"
          onSubmit={(e) => {
            e.preventDefault();
            const m = goto.match(/^\s*(\d+)\s*[|,;: ]\s*(\d+)\s*$/);
            if (m) { setCenter(clampCenter({ x: Number(m[1]) + 0.5, y: Number(m[2]) + 0.5 })); setSel({ x: Number(m[1]), y: Number(m[2]) }); setDetails(false); }
          }}
        >
          <input className="w-20 px-1 border border-wood-border rounded bg-white/50" placeholder="x|y" value={goto} onChange={(e) => setGoto(e.target.value)} />
          <button className="btn" type="submit">{tr('map.go')}</button>
        </form>
        <label className="inline-flex items-center gap-1 sm:ml-auto">
          <input type="checkbox" checked={colors} onChange={(e) => setColors(e.target.checked)} /> {tr('map.playerColors')}
        </label>
        <label className="inline-flex items-center gap-1">
          {tr('map.hideOver')}
          <input type="number" className="w-20 px-1 border border-wood-border rounded bg-white/50" value={maxPoints} onChange={(e) => setMaxPoints(Number(e.target.value) || 0)} />
          {tr('map.ptsShort')}
        </label>
      </div>
      <TileMap
        mapSize={mapSize} seed={seed} revision={`${mapRev}|${villageSummary}`} useTile={useTile} isVillage={isVillage}
        dots={dots} activeId={active.id} minimapDots={minimapDots} armies={armies} clock={gameArmyClock} animations={animations}
        center={center} onCenter={setCenter} zoomIdx={zoomIdx} onZoom={zoomBy} colors={colors} maxPoints={maxPoints}
        selected={sel} onRegion={loadRegion} onHover={(h) => setHover.current(h)}
        onSelectTile={(x, y, village) => {
          setSelArmy(null);
          if (!village) { closeSel(); return; }
          setSel({ x, y });
          setDetails(false);
        }}
        onSelectArmy={(id) => { closeSel(); setSelArmy(id); }}
        surface={(view) => (
          <>
            {selArmy && view.wide && <ArmyCard armyId={selArmy} sheet={false} isVillage={isVillage} onClose={closeArmy} />}
            <MapHoverCard register={registerHover} bounds={{ width: view.width, height: view.height }} />
            {view.wide && popup(view, false)}
          </>
        )}
        outside={(view) => (
          <>
            {selArmy && !view.wide && <ArmyCard armyId={selArmy} sheet isVillage={isVillage} onClose={closeArmy} />}
            {!view.wide && popup(view, true)}
          </>
        )}
        side={sel && details ? <SidePanel sel={sel} onClose={() => setDetails(false)} /> : undefined}
      />
      <p className="text-muted text-[11px] mt-4">
        {tr('map.help1')} {tr('map.help2')} {tr('map.help3')}
      </p>
    </div>
  );
}
