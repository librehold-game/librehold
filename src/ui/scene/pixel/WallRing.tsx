import { t } from '../../../i18n/i18n';
import type { ReactNode } from 'react';
import type { BuildingId } from '../../../engine/types';
import type { DayPhase } from '../sceneModel';
import { LevelBadge } from './LevelBadge';
import { LAYOUT, pixelUrl, type Layer, type WallKind } from './layout';
import { Strip } from './Strip';

/** Wall ring as 8 px columns: back halves sort behind the buildings inside, front halves in front of them. */
export function wallLayers(kind: WallKind, gateFrame: number, gate: (node: ReactNode) => ReactNode = (n) => n): Layer[] {
  const wall = LAYOUT.walls[kind];
  const back = pixelUrl(wall.back);
  const front = pixelUrl(wall.front);
  const layers: Layer[] = [];
  for (const { x0, zBack, zFront } of wall.strips) {
    const viewBox = `${x0 - wall.x} 0 8 ${wall.h}`;
    for (const [z, href, half] of [[zBack, back, 'b'], [zFront, front, 'f']] as const) {
      layers.push({
        z,
        key: `wall-${half}${x0}`,
        node: (
          <svg className="wall-strip" x={x0} y={wall.y} width="8" height={wall.h} viewBox={viewBox}>
            <image href={href} width={wall.w} height={wall.h} />
          </svg>
        ),
      });
    }
  }
  const g = wall.gate;
  layers.push({
    z: LAYOUT.ring.cy + LAYOUT.ring.ry + 3,
    key: 'wall-gate',
    node: gate(<Strip name={`${kind}-gate`} file={g.file} w={g.w} h={g.h} frames={g.frames} frame={gateFrame} x={Math.floor(g.x - g.w / 2)} y={Math.floor(g.y - g.foot)} />),
  });
  return layers;
}

function ringBand(): string {
  const { cx, cy, rx, ry } = LAYOUT.ring;
  const y = cy - 18;
  const ellipse = (a: number, b: number) => `M${cx - a} ${y} A${a} ${b} 0 1 0 ${cx + a} ${y} A${a} ${b} 0 1 0 ${cx - a} ${y} Z`;
  return `${ellipse(rx + 14, ry + 14)} ${ellipse(rx - 14, ry - 14)}`;
}

export function WallHotspot({ level, onClick }: { level: number; onClick: () => void }) {
  const [gx, gy] = LAYOUT.gates.s;
  return (
    <g className="hotspot" onClick={onClick}>
      <title>{t('scene.wall', { level: level > 0 ? t('scene.level', { n: level }) : t('scene.notBuilt') })}</title>
      <path className="hl" d={ringBand()} fillRule="evenodd" fill="#fff" style={{ pointerEvents: 'fill' }} />
      {level > 0 && <LevelBadge x={gx + 58} y={gy - 74} level={level} />}
    </g>
  );
}

const NIGHT: Record<DayPhase, number> = { dawn: 0.25, day: 0, dusk: 0.3, night: 0.55 };

export function NightOverlay({ phase, levels }: { phase: DayPhase; levels: Record<BuildingId, number> }) {
  const opacity = NIGHT[phase];
  if (opacity === 0) return null;
  return (
    <g style={{ pointerEvents: 'none' }}>
      <rect data-night={phase} width={LAYOUT.w} height={LAYOUT.h} fill="#0b1030" opacity={opacity} style={{ mixBlendMode: 'multiply' }} />
      {phase === 'night' && (
        <>
          <defs>
            <radialGradient id="px-glow">
              <stop offset="0" stopColor="#ffb347" stopOpacity="0.55" />
              <stop offset="1" stopColor="#ffb347" stopOpacity="0" />
            </radialGradient>
          </defs>
          {LAYOUT.lights.filter((l) => l.at === 'rally' || levels[l.at] > 0).map((l, i) => <circle key={i} className="deco" cx={l.x} cy={l.y} r={l.r} fill="url(#px-glow)" style={{ mixBlendMode: 'screen' }} />)}
        </>
      )}
    </g>
  );
}
