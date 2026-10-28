import { useCallback, useEffect, useMemo, useRef } from 'react';
import { drawMap, terrainCanvas, type MapLayout } from '../../account/timelapse/render';
import { useT } from '../../i18n/i18n';
import type { SpectatorState } from '../../net/types';
import { armyAt, hitTest, worldNowOf, type MapPick } from './geometry';
import { armyOwnerId } from './WatchCards';

const SIZE = 800;
const LAYOUT: MapLayout = { x: 0, y: 0, size: SIZE };
const ROUTE_ALPHA = 0.35;

/**
 * The whole world on one canvas: terrain, a square per village in its owner's colour and every army as a marker on its
 * route, moving with the clock. A click picks an army marker, else the village under it.
 */
export function WatchMap({ state, receivedAt, onPick }: { state: SpectatorState; receivedAt: number; onPick: (p: MapPick | null) => void }) {
  const t = useT();
  const canvas = useRef<HTMLCanvasElement>(null);
  const mapSize = state.config.mapSize;
  const model = useMemo(() => {
    const index = new Map(Object.keys(state.players).map((id, i) => [id, i + 1]));
    const ids = [...index.keys()];
    const colorOf = (owner: number) => state.players[ids[owner - 1]!]?.color ?? '#8c8c8c';
    const villages = state.villages.map((v) => ({ x: v.x, y: v.y, owner: v.ownerId ? index.get(v.ownerId) ?? 0 : 0 }));
    const owners = new Map(state.villages.map((v) => [v.id, v.ownerId]));
    const armyColor = state.commands.map((c) => state.players[armyOwnerId(c, (id) => owners.get(id) ?? '')]?.color ?? '#8c8c8c');
    return { colorOf, villages, armyColor };
  }, [state]);

  const armiesAt = useCallback((now: number) => state.commands.flatMap((c, i) => {
    const at = armyAt(c, now);
    return at ? [{ id: c.id, x: at.x, y: at.y, from: armyAt(c, c.departedAt)!, to: armyAt(c, c.arrivesAt)!, color: model.armyColor[i]! }] : [];
  }), [state, model]);

  useEffect(() => {
    const ctx = canvas.current?.getContext('2d');
    if (!ctx) return;
    const terrain = terrainCanvas(mapSize);
    const cell = SIZE / mapSize;
    let raf = 0;
    const draw = () => {
      drawMap(ctx, mapSize, terrain, LAYOUT, model.villages, model.colorOf);
      for (const a of armiesAt(worldNowOf(state, receivedAt, Date.now()))) {
        ctx.globalAlpha = ROUTE_ALPHA;
        ctx.strokeStyle = a.color;
        ctx.lineWidth = Math.max(1, cell / 6);
        ctx.beginPath();
        ctx.moveTo(a.from.x * cell, a.from.y * cell);
        ctx.lineTo(a.to.x * cell, a.to.y * cell);
        ctx.stroke();
        ctx.globalAlpha = 1;
        ctx.fillStyle = a.color;
        ctx.strokeStyle = '#2e1a05';
        ctx.beginPath();
        ctx.arc(a.x * cell, a.y * cell, Math.max(3, cell / 3), 0, 2 * Math.PI);
        ctx.fill();
        ctx.stroke();
      }
      raf = requestAnimationFrame(draw);
    };
    draw();
    return () => cancelAnimationFrame(raf);
  }, [state, receivedAt, mapSize, model, armiesAt]);

  const click = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const el = e.currentTarget;
    const r = el.getBoundingClientRect();
    const scale = r.width ? el.width / r.width : 1;
    const now = worldNowOf(state, receivedAt, Date.now());
    onPick(hitTest((e.clientX - r.left) * scale, (e.clientY - r.top) * scale, LAYOUT, mapSize, state.villages, armiesAt(now)));
  };

  return (
    <canvas
      ref={canvas} width={SIZE} height={SIZE} role="img" aria-label={t('watch.wholeMap')} onClick={click}
      className="frame-wood block w-full max-w-[800px] mx-auto cursor-pointer" style={{ aspectRatio: '1', imageRendering: 'pixelated' }}
    />
  );
}
