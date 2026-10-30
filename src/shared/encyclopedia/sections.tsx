import type { CSSProperties } from 'react';
import { formatDuration, formatNumber } from '../../i18n/format';
import { useT } from '../../i18n/i18n';
import type { BuildingId, ResourceSet, UnitId } from '../../engine/types';
import { EntryLink } from './EntryLink';
import { buildingEffect, buildingLevels, researchRows, unitFacts, unlocksOf, type ReqNode, type Timing } from './facts';
import { BUILDING_SPRITES, UNIT_SPRITES } from './sprites';
import { CONQUEST_BALANCE } from '../../data/conquest.data';

const cost3 = (c: ResourceSet) => `${formatNumber(c.wood)} / ${formatNumber(c.clay)} / ${formatNumber(c.iron)}`;

/** A unit's first walking frame; the sheet's frames are 48 px wide, drawn at `size` (default twice that). */
export function UnitSprite({ id, size = 96 }: { id: UnitId; size?: number }) {
  const s = UNIT_SPRITES[id];
  const style: CSSProperties = {
    width: size, height: size, backgroundImage: `url(${s.src})`, backgroundSize: `${s.frames * size}px ${size}px`,
    backgroundRepeat: 'no-repeat', backgroundPosition: '0 0', imageRendering: 'pixelated',
  };
  return <div className="unit-sprite" style={style} aria-hidden="true" />;
}

export function BuildingSprite({ id, size = 96 }: { id: BuildingId; size?: number }) {
  const pixelated: CSSProperties = { imageRendering: 'pixelated' };
  if (id === 'wall') {
    const style: CSSProperties = {
      ...pixelated, display: 'block', height: size, width: Math.round((size * 102) / 107), backgroundImage: `url(${BUILDING_SPRITES.wall})`,
      backgroundSize: '700% 100%', backgroundPosition: '0 0', backgroundRepeat: 'no-repeat',
    };
    return <span className="building-sprite wall-sprite" style={style} />;
  }
  return <img className="building-sprite" src={BUILDING_SPRITES[id]} alt="" loading="lazy" style={{ ...pixelated, display: 'block', height: size, objectFit: 'contain' }} />;
}

function ChainNodes({ nodes, progress }: { nodes: ReqNode[]; progress: boolean }) {
  const t = useT();
  return (
    <ul className="req-chain">
      {nodes.map((n) => (
        <li key={n.building} className={progress ? (n.met ? 'req-met' : 'req-missing') : undefined}>
          <EntryLink to={{ kind: 'building', id: n.building }}>{t(`building.${n.building}`)} {n.level}</EntryLink>
          {progress && (n.met ? ' ✓' : `: ${n.have ? t('encyclopedia.label.have', { n: n.have }) : t('encyclopedia.label.notBuilt')}`)}
          {n.children.length > 0 && <ChainNodes nodes={n.children} progress={progress} />}
        </li>
      ))}
    </ul>
  );
}

/** What a building or unit needs, down the whole chain; with the village's levels each line says whether it is met. */
export function RequirementChain({ nodes, research }: { nodes: ReqNode[]; research?: { needed: boolean; done?: boolean } }) {
  const t = useT();
  const progress = nodes.some((n) => n.have !== null) || research?.done !== undefined;
  const step = research?.needed ? research : null;
  if (nodes.length === 0 && !step) return <p>{t('encyclopedia.label.none')}</p>;
  return (
    <>
      <ChainNodes nodes={nodes} progress={progress} />
      {step && (
        <ul className="req-chain">
          <li className={step.done === undefined ? undefined : step.done ? 'req-met' : 'req-missing'}>
            {t('encyclopedia.label.researchStep')}
            {step.done !== undefined && `: ${step.done ? t('encyclopedia.label.researched') : t('encyclopedia.label.notResearched')}`}
          </li>
        </ul>
      )}
    </>
  );
}

/** Every unit and building that needs this building, with the level each needs. */
export function UnlocksList({ building }: { building: BuildingId }) {
  const t = useT();
  const unlocks = unlocksOf(building);
  if (unlocks.length === 0) return null;
  return (
    <>
      <h2>{t('encyclopedia.label.unlocks')}</h2>
      <ul>
        {unlocks.map((u) => (
          <li key={`${u.kind}-${u.id}`}>
            <EntryLink to={u.kind === 'unit' ? { kind: 'unit', id: u.id as UnitId } : { kind: 'building', id: u.id as BuildingId }}>
              {u.kind === 'unit' ? t(`unit.${u.id}`) : t(`building.${u.id}`)}
            </EntryLink>
            {`, ${t(`building.${building}`)} ${u.level}`}
            {u.recruitedHere && ` (${t('encyclopedia.label.recruitedHere')})`}
          </li>
        ))}
      </ul>
    </>
  );
}

/** Attack and defence at each research level, with what each level costs; `current` is the village's level. */
export function ResearchTable({ unit, timing, current }: { unit: UnitId; timing: Timing; current?: number }) {
  const t = useT();
  const rows = researchRows(unit, timing);
  if (rows.length === 0) return null;
  const heads = ['researchLevel', 'researchBonus', 'attack', 'defense', 'defenseCavalry', 'defenseArcher', 'researchCost', 'time'];
  return (
    <>
      <h2>{t('encyclopedia.label.researchTitle')}</h2>
      <div className="table-scroll">
        <table className="levels">
          <thead><tr>{heads.map((k) => <th key={k}>{t(`encyclopedia.label.${k}`)}</th>)}</tr></thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.level} aria-current={current === r.level ? 'true' : undefined} className={current === r.level ? 'row-current' : undefined}>
                <td>
                  {r.level}
                  {r.free && ` (${t('encyclopedia.label.free')})`}
                </td>
                <td>{r.bonus > 0 ? t('encyclopedia.label.percent', { n: Math.round(r.bonus * 100) }) : '—'}</td>
                <td>{formatNumber(r.attack)}</td><td>{formatNumber(r.defense)}</td>
                <td>{formatNumber(r.defenseCavalry)}</td><td>{formatNumber(r.defenseArcher)}</td>
                <td>{r.free ? '—' : cost3(r.cost)}</td>
                <td>{r.free ? '—' : formatDuration(r.timeMs)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="muted">{t('encyclopedia.label.researchCaption')}</p>
    </>
  );
}

export function UnitStatsTable({ unit, timing, count }: { unit: UnitId; timing: Timing; count?: number }) {
  const t = useT();
  const f = unitFacts(unit);
  const cost = timing.unitCost(unit);
  const n = formatNumber;
  const rows: [string, React.ReactNode][] = [
    ['wood', n(cost.wood)], ['clay', n(cost.clay)], ['iron', n(cost.iron)], ['pop', n(f.pop)],
    ['attack', n(f.attack)], ['defense', n(f.defense)], ['defenseCavalry', n(f.defenseCavalry)],
    ['defenseArcher', n(f.defenseArcher)], ['speed', n(f.minutesPerField)], ['carry', n(f.carry)],
    ['recruitedIn', <EntryLink to={{ kind: 'building', id: f.building }}>{t(`building.${f.building}`)}</EntryLink>],
    ['recruitTime', formatDuration(timing.recruitMs(unit))],
  ];
  if (count !== undefined) rows.push(['inVillage', n(count)]);
  if (unit === 'noble') {
    if (timing.coinCost) rows.push(['coinCost', cost3(timing.coinCost)]);
    rows.push(['maxDistance', t('encyclopedia.label.fields', { n: CONQUEST_BALANCE.nobleMaxDistance })]);
  }
  return (
    <>
      <h2>{t('encyclopedia.label.stats')}</h2>
      <table className="stats">
        <tbody>
          {rows.map(([k, v]) => <tr key={k}><th scope="row">{t(`encyclopedia.label.${k}`)}</th><td>{v}</td></tr>)}
        </tbody>
      </table>
    </>
  );
}

export function LevelsTable({ id, timing }: { id: BuildingId; timing: Timing }) {
  const t = useT();
  const levels = buildingLevels(id, timing);
  const effect = buildingEffect(id);
  const n = formatNumber;
  return (
    <div className="table-scroll">
      <table className="levels">
        <thead>
          <tr>
            {['level', 'wood', 'clay', 'iron', 'time', 'popTotal', 'pointsTotal'].map((k) => <th key={k}>{t(`encyclopedia.label.${k}`)}</th>)}
            {effect && <th>{t(`encyclopedia.label.effect.${effect}`)}</th>}
          </tr>
        </thead>
        <tbody>
          {levels.map((r) => (
            <tr key={r.level}>
              <td>{r.level}</td><td>{n(r.cost.wood)}</td><td>{n(r.cost.clay)}</td><td>{n(r.cost.iron)}</td>
              <td>{formatDuration(r.timeMs)}</td><td>{n(r.pop)}</td><td>{n(r.points)}</td>
              {effect && <td>{effect === 'wall' ? t('encyclopedia.label.percent', { n: r.effect! }) : n(r.effect!)}</td>}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
