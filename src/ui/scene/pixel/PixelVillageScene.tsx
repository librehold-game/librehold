import { t } from '../../../i18n/i18n';
import { buildingName } from '../../../i18n/names';
import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { BUILDINGS } from '../../../data/buildings.data';
import { BUILDING_IDS, type BuildingId } from '../../../engine/types';
import { useGameStore } from '../../../state/gameStore';
import { useUiStore, type AnimationMode } from '../../../state/uiStore';
import { formatDuration } from '../../format';
import { animationBudget, sceneModel, type SceneModel } from '../sceneModel';
import { useDocumentHidden } from '../useBeats';
import { LevelBadge } from './LevelBadge';
import { Strip } from './Strip';
import { LAYOUT, pixelUrl, roadKeys, stageFile, wallKind, type Layer } from './layout';
import { NightOverlay, WallHotspot, wallLayers } from './WallRing';
import { buildRoutes } from './routes';
import { troopLayers, useWalkerLoop, walkerLayers, workerLayers, type Worker } from './Walkers';

const GOLD = '#d4af37';
const BLUE = '#2f5fa8';
const RED = '#a33a2a';
const GATE_FRAMES = LAYOUT.walls.stone.gate.frames;
const GATE_STEP_MS = 90;
const CHOPPER: [number, number] = [687, 571];

type SlotId = Exclude<BuildingId, 'wall'>;

const BONUS_AT: Partial<Record<NonNullable<SceneModel['bonus']>, SlotId>> = {
  wood: 'wood', clay: 'clay', iron: 'iron', farm: 'farm', storage: 'storage', infantry: 'barracks', cavalry: 'stable', allResources: 'market',
};

/** Top-left of a sprite anchored at its bottom-centre, rounded the same way as the export script. */
export function spriteOrigin(file: string, x: number, y: number): { left: number; top: number; w: number; h: number } {
  const [w, h] = LAYOUT.files[file]!;
  return { left: Math.floor(x - w / 2), top: Math.floor(y - h), w, h };
}

export function Img({ file, x, y, flip }: { file: string; x: number; y: number; flip?: boolean }) {
  const { left, top, w, h } = spriteOrigin(file, x, y);
  return (
    <image
      href={pixelUrl(file)}
      x={left}
      y={top}
      width={w}
      height={h}
      transform={flip ? `translate(${2 * left + w}, 0) scale(-1, 1)` : undefined}
    />
  );
}

function HitImg({ file, x, y }: { file: string; x: number; y: number }) {
  const { left, top } = spriteOrigin(file, x, y);
  return (
    <>
      <path className="hit" d={LAYOUT.hits[file]} transform={`translate(${left}, ${top})`} />
      <Img file={file} x={x} y={y} />
    </>
  );
}

function buildingTitle(m: SceneModel, id: BuildingId): string {
  const level = m.levels[id];
  const job = m.construction.find((c) => c.building === id && c.started);
  const queued = !job && m.construction.some((c) => c.building === id);
  return `${buildingName(id)} ${level > 0 ? t('scene.level', { n: level }) : t('scene.notBuilt')}${job ? t('scene.building', { pct: Math.round(job.ratio * 100) }) : queued ? t('scene.queued') : ''}`;
}

function sortLayers(layers: Layer[]): Layer[] {
  return layers.map((l, i) => [l, i] as const).sort((a, b) => a[0].z - b[0].z || a[1] - b[1]).map(([l]) => l);
}

export function PixelVillageScene({ villageId, mode }: { villageId: string; mode: AnimationMode }) {
  const json = useGameStore((s) => JSON.stringify(sceneModel(s.game!, villageId)));
  const m = useMemo(() => JSON.parse(json) as SceneModel | null, [json]);
  const hidden = useDocumentHidden();
  const openBuilding = useUiStore((s) => s.openBuilding);
  const setView = useUiStore((s) => s.setView);
  const full = mode === 'full';

  const life = useMemo(() => {
    if (!m || !full) return { routes: [], workers: [] as Worker[], troops: 0 };
    const budget = animationBudget(m, mode);
    const workers: Worker[] = [];
    if (m.levels.wood > 0) workers.push({ key: 'chop', sprite: 'chop', x: CHOPPER[0], y: CHOPPER[1] });
    const site = m.construction.find((c) => c.started)?.building;
    if (site && site !== 'wall') {
      const [x, y] = LAYOUT.slots[site];
      workers.push({ key: 'hammer', sprite: 'hammer', x: x - 46, y: y + 22 });
    }
    return { routes: buildRoutes(m, budget), workers, troops: budget.troops };
  }, [m, full, mode]);

  const [gateFrame, setGateFrame] = useState(0);
  const gateWanted = useRef(false);
  const onGate = useCallback((open: boolean) => { gateWanted.current = open; }, []);
  useEffect(() => {
    if (!full || hidden) return;
    const id = setInterval(() => setGateFrame((f) => (gateWanted.current ? Math.min(GATE_FRAMES - 1, f + 1) : Math.max(0, f - 1))), GATE_STEP_MS);
    return () => clearInterval(id);
  }, [full, hidden]);
  const bind = useWalkerLoop(life.routes, life.workers, hidden || !full, onGate);

  if (!m) return null;
  const limited = mode !== 'full';
  const layers: Layer[] = [];
  // Badges and progress sit above every layer, so a wall tower in front of a building cannot hide them.
  const marks: ReactNode[] = [];

  for (const key of roadKeys(m.levels, m.construction.map((c) => c.building))) {
    const road = LAYOUT.roads[key];
    if (!road) continue;
    const [w, h] = LAYOUT.files[road.file]!;
    layers.push({ z: -1e6, key: `road-${key}`, node: <image href={pixelUrl(road.file)} x={road.x} y={road.y} width={w} height={h} /> });
  }

  LAYOUT.scenery.forEach((s, i) => layers.push({ z: s.z, key: `sc-${i}`, node: <Img file={s.file} x={s.x} y={s.y} flip={s.flip} /> }));

  const wall = wallKind(m.variants.wall);
  if (m.levels.wall > 0 && wall) layers.push(...wallLayers(wall, full ? gateFrame : 0));
  layers.push(...walkerLayers(life.routes, bind), ...workerLayers(life.workers, bind), ...troopLayers(life.troops));
  layers.push({ z: -1e5, key: 'wall-hotspot', node: <WallHotspot level={m.levels.wall} onClick={() => openBuilding('wall', villageId)} /> });

  for (const id of BUILDING_IDS) {
    if (id === 'wall') continue;
    const [sx, sy] = LAYOUT.slots[id];
    const ay = sy + 12;
    const level = m.levels[id];
    const building = m.construction.some((c) => c.building === id);
    const job = m.construction.find((c) => c.building === id && c.started);
    const variant = m.variants[id];
    const file = level > 0 && variant !== 0 ? stageFile(id, variant) : building ? LAYOUT.construction : null;
    const top = file ? spriteOrigin(file, sx, ay) : null;
    const bonus = m.bonus !== null && BONUS_AT[m.bonus] === id;
    const animated = !limited && level > 0 && variant === 2;
    const loop = animated && (id === 'iron' || id === 'farm') ? LAYOUT.anim[id] : null;
    const wheel = animated && id === 'wood' ? LAYOUT.anim.wood : null;
    layers.push({
      z: ay,
      key: `b-${id}`,
      node: (
        <g className="hotspot" data-building={id} onClick={() => openBuilding(id, villageId)}>
          <title>{buildingTitle(m, id)}</title>
          <ellipse className="hl" cx={sx} cy={sy + 4} rx="46" ry="17" fill="#fff" />
          {file && top && <path className="hit" d={LAYOUT.hits[file]} transform={`translate(${top.left}, ${top.top})`} />}
          {loop
            ? <Strip name={id} {...loop} x={Math.floor(sx - loop.w / 2)} y={Math.floor(ay - loop.h)} className="deco" />
            : file && <Img file={file} x={sx} y={ay} />}
          {wheel && <Strip name="wood" {...wheel} className="deco" />}
        </g>
      ),
    });
    if (top && (level > 0 || bonus)) {
      const bx = sx + Math.min(40, top.w / 2 - 8);
      marks.push(
        <g key={id} data-marks={id} className="marks" onClick={() => openBuilding(id, villageId)}>
          {level > 0 && <LevelBadge x={bx} y={top.top + 12} level={level} />}
          {level > 0 && job && (
            <g data-progress={job.ratio} transform={`translate(${bx - 13}, ${top.top + 23})`}>
              <rect width="26" height="7" rx="2" fill="#f4efe0" stroke="#2e1a05" strokeWidth="1" />
              <rect x="1.5" y="1.5" width={Math.max(2, 23 * job.ratio)} height="4" rx="1" fill="#2f7d32" />
            </g>
          )}
          {level > 0 && building && !job && <text x={bx - 20} y={top.top + 17} fontSize="12" textAnchor="middle">⏳</text>}
          {bonus && <text x={sx - Math.min(40, top.w / 2 - 8)} y={top.top + 16} fontSize="14" fill={GOLD} stroke="#2e1a05" strokeWidth="0.6">★</text>}
        </g>,
      );
    }
  }

  const [rx, ry] = LAYOUT.rally;
  layers.push({
    z: ry + 22,
    key: 'rally',
    node: (
      <g className="hotspot" onClick={() => setView('rally')}>
        <title>{t('scene.rallyPoint')}</title>
        <ellipse className="hl" cx={rx + 4} cy={ry + 14} rx="40" ry="15" fill="#fff" />
        <HitImg file={LAYOUT.place} x={rx + 4} y={ry + 22} />
        {m.conqueredFlag && (
          <g transform={`translate(${rx - 24}, ${ry + 16})`}>
            <line x1="0" y1="0" x2="0" y2="-40" stroke="#2e1a05" strokeWidth="1.6" />
            <path d="M0 -40 L18 -34 L0 -28 Z" fill={BLUE} style={{ transformOrigin: '0 -34px', animation: 'sc-flag 1.6s infinite' }} />
          </g>
        )}
        {m.storageFull && <Img file="scenery/woodpile.png" x={rx + 70} y={ry + 34} />}
      </g>
    ),
  });

  if (m.caravan && !limited) {
    layers.push({
      z: LAYOUT.h - 1,
      key: 'caravan',
      node: (
        <g className="deco" style={{ '--dx': '-420px', animation: 'sc-wagon 11s linear infinite' } as CSSProperties}>
          <Img file="scenery/cart.png" x={LAYOUT.ring.cx + 120} y={LAYOUT.h - 18} />
        </g>
      ),
    });
  }

  const [mx, my] = LAYOUT.slots.main;
  const mainTop = m.levels.main > 0 && m.variants.main !== 0 ? spriteOrigin(stageFile('main', m.variants.main), mx, my + 12).top : my - 40;
  return (
    <svg viewBox={`0 0 ${LAYOUT.w} ${LAYOUT.h}`} className={`scene pixel scene-${mode} ${hidden ? 'scene-paused' : ''}`} role="img" aria-label={t('scene.label')}>
      <image href={pixelUrl('ground.png')} x="0" y="0" width={LAYOUT.w} height={LAYOUT.h} />
      {sortLayers(layers).map((l) => <Fragment key={l.key}>{l.node}</Fragment>)}
      {marks}
      <NightOverlay phase={m.dayPhase} levels={m.levels} />
      {m.defeated && (
        <g transform={`translate(${mx - 20}, ${mainTop + 10})`}>
          {[0, 1, 2].map((i) => <circle key={i} cx={i * 14} cy="0" r={5 + i} fill="#4a4540" style={{ pointerEvents: 'none', '--sx': `${(i - 1) * 10}px`, animation: `sc-smoke ${3 + i * 0.5}s linear ${i * 0.9}s infinite` } as CSSProperties} />)}
        </g>
      )}
      {m.alarm && (
        <g style={{ pointerEvents: 'none' }}>
          <rect width={LAYOUT.w} height={LAYOUT.h} fill={RED} style={{ animation: 'sc-alarm 0.8s infinite alternate' }} />
          <g transform={`translate(${mx}, ${mainTop - 4})`}>
            <path d="M-7 0 Q-7 -12 0 -12 Q7 -12 7 0 Z" fill={GOLD} stroke="#2e1a05" strokeWidth="1" style={{ transformOrigin: '0 -12px', animation: 'sc-bell 0.4s ease-in-out infinite alternate' }} />
            <text y="-18" textAnchor="middle" fontSize="13" fontWeight="700" fill={RED} stroke="#fff" strokeWidth="0.4" style={{ animation: 'sc-pulse 0.6s infinite alternate' }}>ATAK za {formatDuration(m.alarmEtaMs ?? 0)}</text>
          </g>
        </g>
      )}
    </svg>
  );
}
