import { useMemo } from 'react';
import { MAP } from '../../data/world.config';
import { useT } from '../../i18n/i18n';
import type { AnimationMode } from '../../state/uiStore';
import { pixelIcon } from '../pixelIcons';
import { FOREIGN_KINDS, TORCH_KINDS, leadUnit, type ArmyKind, type MapArmy } from './armies';
import type { ArmyClock } from './armyClock';
import { UNIT_IDS, type UnitId } from '../../engine/types';
import { formatNumber } from '../format';
import { markerState, useArmyLoop, type ToPx } from './useArmyLoop';
import { CrowdCanvas } from './CrowdCanvas';

const LINE: Record<ArmyKind, { color: string; width: number; dash?: string }> = {
  ownAttack: { color: '#2f5fa8', width: 1.5, dash: '4 3' },
  ownSupport: { color: '#4c8f2f', width: 1.5, dash: '4 3' },
  ownReturn: { color: '#6f9a3a', width: 1.5, dash: '2 3' },
  incomingAttack: { color: '#ff2d2d', width: 2.5 },
  incomingSupport: { color: '#4c8f2f', width: 2 },
  ownIntercept: { color: '#7b4bb5', width: 1.5, dash: '4 3' },
  threat: { color: '#e07a1f', width: 2.5 },
  passingAttack: { color: '#a33a2a', width: 1.5, dash: '6 3' },
  passingSupport: { color: '#3a8f8a', width: 1.5, dash: '6 3' },
  passingReturn: { color: '#7a7a6a', width: 1.5, dash: '6 3' },
  trade: { color: '#c8a54b', width: 1.5, dash: '4 3' },
};

function Torch() {
  return (
    <g>
      <rect x={-1.5} y={-2} width={3} height={9} fill="#6b4421" />
      <path d="M0 -9 C4 -6 3 -3 0 -2 C-3 -3 -4 -6 0 -9 Z" fill="#ff8a1e" stroke="#c0390f" strokeWidth={0.8} />
    </g>
  );
}

function Shield() {
  return <path d="M0 -8 L7 -5 L6 2 C5 5 2 7 0 8 C-2 7 -5 5 -6 2 L-7 -5 Z" fill="#3f6fb5" stroke="#1f3a63" strokeWidth={1} />;
}

/** An army on its way home: a faded shield with a return arrow, so it does not read as support. */
function Homeward() {
  return (
    <g>
      <g opacity={0.5}><Shield /></g>
      <image href={pixelIcon('return')} x={1} y={-10} width={12} height={12} style={{ imageRendering: 'pixelated' }} />
    </g>
  );
}

function ArmyIcon({ army }: { army: MapArmy }) {
  if (TORCH_KINDS.has(army.kind)) return <Torch />;
  if (army.kind === 'passingReturn') return <Homeward />;
  if (FOREIGN_KINDS.has(army.kind)) return <Shield />;
  if (army.kind === 'trade' || !army.units) return <image href={pixelIcon('cart')} x={-8} y={-8} width={16} height={16} style={{ imageRendering: 'pixelated' }} />;
  return <image href={pixelIcon(`unit-${leadUnit(army.units)}`)} x={-8} y={-8} width={16} height={16} style={{ imageRendering: 'pixelated' }} />;
}

function Boat({ army }: { army: MapArmy }) {
  return (
    <g>
      <path d="M-12 1 L12 1 L8 8 L-8 8 Z" fill="#7a4a22" stroke="#3b230f" strokeWidth={1} />
      <g transform="translate(0, -3) scale(0.8)"><ArmyIcon army={army} /></g>
    </g>
  );
}

/** The exact count of each unit type, shown beside an own army while it marches as a crowd. */
function Counts({ units }: { units: Partial<Record<UnitId, number>> }) {
  let x = 0;
  const items = UNIT_IDS.filter((u) => (units[u] ?? 0) > 0).map((u) => {
    const text = formatNumber(units[u]!);
    const at = x;
    x += 15 + text.length * 6;
    return (
      <g key={u} transform={`translate(${at}, 0)`}>
        <image href={pixelIcon(`unit-${u}`)} x={0} y={-6} width={12} height={12} />
        <text x={14} y={4} fontSize={10} fill="#2e1a05">{text}</text>
      </g>
    );
  });
  return (
    <g data-counts display="none" transform="translate(16, -30)">
      <rect x={-3} y={-9} width={x + 2} height={18} rx={3} fill="#f4e4bc" stroke="#7a5a2a" strokeWidth={1} opacity={0.92} />
      {items}
    </g>
  );
}

function segmentInView(ax: number, ay: number, bx: number, by: number, width: number, height: number, pad: number): boolean {
  return Math.max(ax, bx) >= -pad && Math.min(ax, bx) <= width + pad && Math.max(ay, by) >= -pad && Math.min(ay, by) <= height + pad;
}

export interface ArmyViewport {
  x0: number; y0: number; offX: number; offY: number; tw: number; th: number; width: number; height: number;
  onSelect: (armyId: string) => void; isDragging: () => boolean;
}

/** Armies on the tile map: route lines, markers and the marching crowds, from whatever source draws them. */
export function ArmyLayerView({ armies, seed, isVillage, mode, clock, x0, y0, offX, offY, tw, th, width, height, onSelect, isDragging }: ArmyViewport & {
  armies: MapArmy[]; seed: number; isVillage: (x: number, y: number) => boolean; mode: AnimationMode; clock: ArmyClock;
}) {
  const t = useT();
  const toPx = useMemo<ToPx>(() => (x, y) => [(x - x0) * tw - offX, (y - y0) * th - offY], [x0, y0, offX, offY, tw, th]);
  const bind = useArmyLoop(armies, toPx, mode, seed, clock, isVillage);
  if (armies.length === 0) return null;
  const now = clock.now();
  return (
    <>
    {mode !== 'off' && tw >= MAP.crowdMinTileW && (
      <CrowdCanvas armies={armies} toPx={toPx} tw={tw} th={th} width={width} height={height} seed={seed} isVillage={isVillage} mode={mode} clock={clock} />
    )}
    <svg className="absolute inset-0 pointer-events-none" width={width} height={height} data-army-layer>
      <defs>
        {(Object.keys(LINE) as ArmyKind[]).map((k) => (
          <marker key={k} id={`army-arrow-${k}`} markerWidth="8" markerHeight="8" refX="6" refY="4" orient="auto">
            <path d="M0 0 L8 4 L0 8 Z" fill={LINE[k].color} />
          </marker>
        ))}
      </defs>
      {armies.map((a) => {
        const [ax, ay] = toPx(a.from.x, a.from.y), [bx, by] = toPx(a.to.x, a.to.y);
        if (!segmentInView(ax, ay, bx, by, width, height, Math.max(tw, th))) return null;
        const s = markerState(a, now, toPx, seed, isVillage);
        const style = LINE[a.kind];
        return (
          <g key={a.id} data-army={a.id} data-kind={a.kind}>
            <line
              x1={Math.round(ax)} y1={Math.round(ay)} x2={Math.round(bx)} y2={Math.round(by)}
              stroke={style.color} strokeWidth={style.width} strokeDasharray={style.dash} opacity="0.85"
              markerEnd={`url(#army-arrow-${a.kind})`}
            />
            <g
              ref={bind(a.id)}
              data-marker
              data-boat={s.boat ? '' : undefined}
              transform={`translate(${s.px}, ${s.py})`}
              style={{ pointerEvents: 'auto', cursor: 'pointer' }}
              onClick={(e) => { if (isDragging()) return; e.stopPropagation(); onSelect(a.id); }}
            >
              <title>{t(`map.army.${a.kind}`)}</title>
              <circle r={12} fill="transparent" />
              <g data-icon>
                <circle r={10} fill="#f4e4bc" stroke={style.color} strokeWidth={1.5} />
                <g data-land display={s.boat ? 'none' : 'inline'}><ArmyIcon army={a} /></g>
                <g data-water display={s.boat ? 'inline' : 'none'}><Boat army={a} /></g>
              </g>
              {a.units && !FOREIGN_KINDS.has(a.kind) && <Counts units={a.units} />}
              <text data-eta textAnchor="middle" y={21} fontSize={9} fill="#2e1a05" stroke="#f4e4bc" strokeWidth={2} paintOrder="stroke">{s.eta}</text>
            </g>
          </g>
        );
      })}
    </svg>
    </>
  );
}
