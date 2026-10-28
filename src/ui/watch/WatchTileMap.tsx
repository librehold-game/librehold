import { useCallback, useMemo, useRef, useState } from 'react';
import { MAP } from '../../data/world.config';
import { coordsOfVillageId, type CommandType } from '../../engine/types';
import type { TileInfo } from '../../engine/world';
import { useT } from '../../i18n/i18n';
import type { SpectatorState } from '../../net/types';
import { useUiStore } from '../../state/uiStore';
import { mapArmy, type ArmyKind, type MapArmy } from '../map/armies';
import { armyClock } from '../map/armyClock';
import { TileMap, ZOOM_MAX } from '../map/TileMap';
import { ONLINE_TERRAIN_SEED } from '../../shared/terrain/seed';
import { worldNowOf } from './geometry';
import type { WatchSelection } from './watchStore';
import { useWatchStore } from './watchStore';

/** Every army is the host's to see in full: drawn with the own-army styles, units shown. */
const KIND: Partial<Record<CommandType, ArmyKind>> = { attack: 'ownAttack', support: 'ownSupport', return: 'ownReturn', intercept: 'ownIntercept' };

const EMPTY = (x: number, y: number): TileInfo => ({ kind: 'empty', x, y, terrain: 0 });

/** The god view's normal map: the player's tile map, fed from the spectator state; a click opens a read-only card. */
export function WatchTileMap({ state, receivedAt, selected, onPick }: {
  state: SpectatorState; receivedAt: number; selected: WatchSelection | null; onPick: (p: WatchSelection | null) => void;
}) {
  const t = useT();
  const mapSize = state.config.mapSize;
  const animations = useUiStore((s) => s.prefs.animations);
  const [center, setCenter] = useState({ x: mapSize / 2, y: mapSize / 2 });
  const [zoomIdx, setZoomIdx] = useState<number>(MAP.defaultZoomIdx);
  const zoomBy = (d: number) => setZoomIdx((z) => Math.max(0, Math.min(ZOOM_MAX, z + d)));

  const tiles = useMemo(() => {
    const out = new Map<string, TileInfo>();
    for (const v of state.villages) {
      const owner = v.ownerId ? state.players[v.ownerId] : undefined;
      out.set(`${v.x},${v.y}`, {
        kind: 'village', x: v.x, y: v.y, id: v.id, ownerId: owner ? owner.id : null, ownerName: owner?.name, ownerIsBot: owner?.isBot,
        ownerStatus: owner?.status ?? null, ownerColor: owner?.color, name: '', points: v.points, materialized: true,
      });
    }
    return out;
  }, [state]);
  const useTile = useCallback((x: number, y: number) => tiles.get(`${x},${y}`) ?? EMPTY(x, y), [tiles]);
  const isVillage = useCallback((x: number, y: number) => tiles.has(`${x},${y}`), [tiles]);
  const minimapDots = useMemo(() => state.villages.map((v) => ({ x: v.x, y: v.y, player: v.ownerId !== '' })), [state]);
  const armies = useMemo(() => state.commands.flatMap((c): MapArmy[] => {
    const kind = KIND[c.type];
    const a = kind && mapArmy(c, kind, mapSize);
    return a ? [a] : [];
  }), [state, mapSize]);

  const timing = useRef({ state, receivedAt });
  timing.current = { state, receivedAt };
  const clock = useMemo(() => armyClock(
    () => worldNowOf(timing.current.state, timing.current.receivedAt, Date.now()),
    (onTick) => useWatchStore.subscribe((s, prev) => { if (s.state !== prev.state) onTick(); }),
  ), []);

  return (
    <section aria-label={t('watch.normalMap')}>
      <div className="flex gap-1 mb-2">
        <button className="btn" onClick={() => zoomBy(-1)} disabled={zoomIdx === 0} aria-label={t('watch.zoomIn')}>＋</button>
        <button className="btn" onClick={() => zoomBy(1)} disabled={zoomIdx === ZOOM_MAX} aria-label={t('watch.zoomOut')}>－</button>
      </div>
      <TileMap
        mapSize={mapSize} seed={ONLINE_TERRAIN_SEED} revision={String(state.villages.length)} useTile={useTile} isVillage={isVillage}
        minimapDots={minimapDots} armies={armies} clock={clock} animations={animations}
        center={center} onCenter={setCenter} zoomIdx={zoomIdx} onZoom={zoomBy} colors maxPoints={Number.POSITIVE_INFINITY}
        selected={selected?.kind === 'village' ? coordsOfVillageId(selected.id) : null}
        onSelectTile={(x, y, village) => onPick(village ? { kind: 'village', id: `v:${x}:${y}` } : null)}
        onSelectArmy={(id) => onPick({ kind: 'army', id })}
      />
    </section>
  );
}
