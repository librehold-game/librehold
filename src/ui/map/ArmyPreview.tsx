import { useEffect, useRef } from 'react';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { useDocumentHidden } from '../useDocumentHidden';
import { armyPosition, groundAt, type Ground, type MapArmy } from './armies';
import { armySprite, groundImage } from './armySprites';
import { crowdFor, drawBoat, drawFigure, image } from './CrowdCanvas';
import { crowdLayout } from './crowdLayout';
import { armyNow } from './gameArmyClock';

const W = 240, H = 120;
const FLAT: Record<Ground, string> = { meadow: '#7fae4e', forest: '#3d6b2a', mountain: '#8a8478', water: '#3f7fb8' };
const FRAME_MS = 180;
const PER_BOAT = 8;

/** The army in the village-view perspective, on the ground it stands on right now. */
export function ArmyPreview({ army, seed, isVillage }: { army: MapArmy; seed: number; isVillage: (x: number, y: number) => boolean }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const mode = useUiStore((s) => s.prefs.animations);
  const hidden = useDocumentHidden();
  const latest = useRef(army);
  latest.current = army;

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const draw = () => {
      const a = latest.current;
      const pos = armyPosition(a, armyNow());
      const ground = groundAt(seed, pos.x, pos.y, isVillage);
      const wall = typeof performance === 'undefined' ? Date.now() : performance.now();
      const frame = mode === 'full' ? Math.floor(wall / FRAME_MS) : 0;
      const crowd = crowdFor(a, 60);
      const n = crowd.reduce((s, c) => s + c.figures, 0);
      const boats = ground === 'water' ? Math.ceil(n / PER_BOAT) : 0;
      canvas.setAttribute('data-ground', ground);
      canvas.setAttribute('data-figures', String(ground === 'water' ? 0 : n));
      canvas.setAttribute('data-boats', String(boats));
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      ctx.imageSmoothingEnabled = false;
      const backdrop = groundImage(ground);
      const img = backdrop ? image(backdrop) : null;
      if (img) ctx.drawImage(img, 0, 0, W, H);
      else {
        ctx.fillStyle = FLAT[ground];
        ctx.fillRect(0, 0, W, H);
      }
      const headX = W * 0.72, headY = H * 0.62;
      if (boats > 0) {
        for (const s of crowdLayout([{ unit: 'spear', figures: boats }], { dx: 1, dy: 0 }, 34)) drawBoat(ctx, a, 'east', frame, headX + s.dx, headY + s.dy, 0.9);
        return;
      }
      const spots = crowdLayout(crowd, { dx: 1, dy: 0 }, 11).sort((p, q) => p.dy - q.dy);
      for (const s of spots) drawFigure(ctx, armySprite(s.unit), 'east', frame + (Math.round(s.dx) % 2 === 0 ? 0 : 1), headX + s.dx, headY + s.dy, 0.75);
    };
    draw();
    const unsubscribe = useGameStore.subscribe((s, prev) => { if (s.game?.now !== prev.game?.now) draw(); });
    let raf = 0;
    if (mode !== 'off' && !hidden && typeof requestAnimationFrame === 'function') {
      const loop = () => { draw(); raf = requestAnimationFrame(loop); };
      raf = requestAnimationFrame(loop);
    }
    return () => { unsubscribe(); if (raf) cancelAnimationFrame(raf); };
  }, [seed, isVillage, mode, hidden]);

  return (
    <canvas
      ref={ref}
      width={W}
      height={H}
      className="block w-full rounded border border-wood-border"
      style={{ imageRendering: 'pixelated', aspectRatio: `${W} / ${H}` }}
    />
  );
}
