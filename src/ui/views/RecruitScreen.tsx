import { buildingName, unitName } from '../../i18n/names';
import { t, useT } from '../../i18n/i18n';
import { useState } from 'react';
import { BUILDINGS } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import {
  checkRecruit, maxRecruitable, unitsForBuilding, type RecruitBlocker,
} from '../../engine/units';
import type { RecruitBuilding, UnitId } from '../../engine/types';
import { selectActiveVillage, useGameStore, type GameStore } from '../../state/gameStore';
import { selectNow } from '../../state/selectors';
import { CostRow } from '../components/CostRow';
import { Countdown } from '../components/Countdown';
import { ProgressBar } from '../components/ProgressBar';
import { UnitIcon } from '../components/UnitIcon';
import { formatDuration, formatNumber } from '../format';

function blockerText(b: RecruitBlocker): string {
  switch (b.kind) {
    case 'requirements': return t('recruit.requires', { list: b.missing.map((m) => `${buildingName(m.building)} ${m.level}`).join(', ') });
    case 'research': return t('recruit.needsResearch');
    case 'resources': return t('recruit.noResources');
    case 'population': return t('recruit.noPopulation', { needed: b.needed, free: b.free });
    case 'limit': return t('recruit.limit', { max: b.max });
    case 'coins': return t('recruit.coins', { have: b.have, needed: b.needed });
    case 'count': return t('recruit.count');
  }
}

function UnitRow({ unit }: { unit: UnitId }) {
  const [count, setCount] = useState(1);
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const recruit = useGameStore((s) => s.recruit);
  useT();
  const json = useGameStore((s: GameStore) => {
    if (!s.game) return '';
    const v = selectActiveVillage(s);
    const check = checkRecruit(s.game, v, unit, Math.max(1, count));
    return JSON.stringify({
      perUnitMs: check.perUnitMs,
      blockers: check.blockers,
      max: maxRecruitable(v, unit, s.game),
      present: v.units[unit],
      have: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) },
    });
  });
  if (!json) return null;
  const m = JSON.parse(json) as {
    perUnitMs: number; blockers: RecruitBlocker[]; max: number; present: number;
    have: { wood: number; clay: number; iron: number };
  };
  const def = UNITS[unit];
  const locked = m.blockers.some((b) => b.kind === 'requirements' || b.kind === 'research');
  const disabled = m.blockers.length > 0;

  return (
    <tr className={locked ? 'text-muted' : ''}>
      <td>
        <div className="flex items-center gap-2 font-semibold">
          <UnitIcon unit={unit} />{unitName(unit)}
        </div>
        <div className="text-[11px] text-muted">
          {t('recruit.stats', { attack: def.attack, def: def.defense, defCav: def.defenseCavalry, defArch: def.defenseArcher, speed: def.speed, haul: def.haul })}
        </div>
      </td>
      <td data-label={t('recruit.colCost')}><CostRow cost={def.cost} have={m.have} /></td>
      <td className="tabular-nums" data-label={t('recruit.colPopulation')}>{def.pop}</td>
      <td className="tabular-nums" data-label={t('recruit.colTimeEach')}>{formatDuration(m.perUnitMs)}</td>
      <td className="tabular-nums" data-label={t('recruit.colInVillage')}>{formatNumber(m.present)}</td>
      <td className="whitespace-nowrap stack-inline">
        {locked ? (
          <span className="text-[11px]">{m.blockers.filter((b) => b.kind === 'requirements' || b.kind === 'research').map(blockerText).join('; ')}</span>
        ) : (
          <span className="inline-flex items-center gap-1">
            <input
              type="number" min={1} max={Math.max(1, m.max)} value={count}
              className="w-16 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
              onChange={(e) => setCount(Math.max(1, Math.floor(Number(e.target.value) || 1)))}
            />
            <button className="btn !px-1.5" title={t('recruit.max', { n: m.max })} onClick={() => setCount(Math.max(1, m.max))}>
              ({m.max})
            </button>
            <button
              className="btn"
              disabled={disabled}
              title={m.blockers.map(blockerText).join('\n') || t('recruit.recruitTitle', { count, unit: unitName(unit) })}
              onClick={() => recruit(villageId, unit, count)}
            >
              {t('recruit.recruit')}
            </button>
          </span>
        )}
      </td>
    </tr>
  );
}

function QueueRow({ jobId, building }: { jobId: string; building: RecruitBuilding }) {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const cancel = useGameStore((s) => s.cancelRecruitJob);
  useT();
  const json = useGameStore((s) => {
    const v = selectActiveVillage(s);
    const j = v.recruitQueues[building].find((x) => x.id === jobId);
    if (!j) return '';
    const now = selectNow(s);
    const elapsed = Math.max(0, now - j.startedAt);
    const nextAt = j.startedAt + (j.done + 1) * j.perUnitMs;
    const ratio = Math.min(1, elapsed / Math.max(1, j.total * j.perUnitMs));
    return JSON.stringify({ unit: j.unit, done: j.done, total: j.total, completesAt: j.completesAt, nextAt, ratio: Math.round(ratio * 1000) / 1000 });
  });
  if (!json) return null;
  const j = JSON.parse(json) as { unit: UnitId; done: number; total: number; completesAt: number; nextAt: number; ratio: number };
  return (
    <tr>
      <td className="flex items-center gap-2 font-semibold"><UnitIcon unit={j.unit} />{unitName(j.unit)}</td>
      <td className="tabular-nums" data-label={t('recruit.colDone')}>{j.done} / {j.total}</td>
      <td className="w-44 max-w-full" data-label={t('recruit.colProgress')}><ProgressBar ratio={j.ratio} label={`${Math.floor(j.ratio * 100)}%`} /></td>
      <td className="tabular-nums" data-label={t('recruit.colNext')}><Countdown until={j.nextAt} /></td>
      <td className="tabular-nums" data-label={t('recruit.colEnd')}><Countdown until={j.completesAt} /></td>
      <td className="stack-inline"><button className="btn" onClick={() => cancel(villageId, jobId)} title={t('recruit.refund')}>{t('recruit.cancel')}</button></td>
    </tr>
  );
}

export function RecruitScreen({ building }: { building: RecruitBuilding }) {
  const units = unitsForBuilding(building);
  const jobIds = useGameStore((s) => selectActiveVillage(s).recruitQueues[building].map((j) => j.id).join(','));
  const ids = jobIds ? jobIds.split(',') : [];
  useT();
  return (
    <div>
      <table className="table-game table-stack">
        <thead>
          <tr><th>{t('recruit.colUnit')}</th><th>{t('recruit.colCost')}</th><th>{t('recruit.colPop')}</th><th>{t('recruit.colTimeEach')}</th><th>{t('recruit.colInVillage')}</th><th>{t('recruit.colRecruitment')}</th></tr>
        </thead>
        <tbody>{units.map((u) => <UnitRow key={u} unit={u} />)}</tbody>
      </table>
      <h3 className="text-[15px] mt-4 mb-1">{t('recruit.queueTitle')}</h3>
      {ids.length === 0 ? (
        <p className="text-muted italic text-[12px]">{t('recruit.queueEmpty')}</p>
      ) : (
        <table className="table-game table-stack">
          <thead>
            <tr><th>{t('recruit.colUnit')}</th><th>{t('recruit.colDone')}</th><th>{t('recruit.colProgress')}</th><th>{t('recruit.colNext')}</th><th>{t('recruit.colEnd')}</th><th>{t('recruit.colAction')}</th></tr>
          </thead>
          <tbody>{ids.map((id) => <QueueRow key={id} jobId={id} building={building} />)}</tbody>
        </table>
      )}
    </div>
  );
}
