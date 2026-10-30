import { buildingName, unitName } from '../../i18n/names';
import { t, useT } from '../../i18n/i18n';
import { BUILDINGS } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import { researchBonus, unitNeedsResearch } from '../../engine/research';
import { checkResearch, type ResearchBlocker } from '../../engine/units';
import { UNIT_IDS, type UnitId } from '../../engine/types';
import { selectActiveVillage, useGameStore, type GameStore } from '../../state/gameStore';
import { selectNow } from '../../state/selectors';
import { CostRow } from '../components/CostRow';
import { Countdown } from '../components/Countdown';
import { ProgressBar } from '../components/ProgressBar';
import { UnitIcon } from '../components/UnitIcon';
import { InfoLink } from '../info/InfoLink';
import { formatDuration } from '../format';

function blockerText(b: ResearchBlocker): string {
  switch (b.kind) {
    case 'maxLevel': return t('research.maxLevel');
    case 'busy': return t('research.busy');
    case 'requirements': return t('recruit.requires', { list: b.missing.map((m) => `${buildingName(m.building)} ${m.level}`).join(', ') });
    case 'resources': return t('research.noResources');
  }
}

function ResearchRow({ unit }: { unit: UnitId }) {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const research = useGameStore((s) => s.research);
  useT();
  const json = useGameStore((s: GameStore) => {
    if (!s.game) return '';
    const v = selectActiveVillage(s);
    const c = checkResearch(s.game, v, unit);
    return JSON.stringify({
      ...c,
      have: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) },
    });
  });
  if (!json) return null;
  const m = JSON.parse(json) as ReturnType<typeof checkResearch> & { have: { wood: number; clay: number; iron: number } };
  const maxed = m.blockers.some((b) => b.kind === 'maxLevel');
  const def = UNITS[unit];
  const free = !unitNeedsResearch(unit);
  return (
    <tr className={m.currentLevel === 0 ? 'text-muted' : ''}>
      <td>
        <InfoLink to={{ kind: 'unit', id: unit }} className="flex items-center gap-2 font-semibold cursor-pointer"><UnitIcon unit={unit} />{unitName(unit)}</InfoLink>
        {free && <div className="text-[11px] text-muted">{t('research.free')}</div>}
      </td>
      <td className="tabular-nums" data-label={t('research.colLevel')}>
        {m.currentLevel} / 3
        <span className="text-muted"> (+{Math.round(researchBonus(m.currentLevel) * 100)} %)</span>
      </td>
      <td data-label={t('research.colCost')}>{maxed ? '—' : <CostRow cost={m.cost} have={m.have} />}</td>
      <td className="tabular-nums" data-label={t('research.colTime')}>{maxed ? '—' : formatDuration(m.durationMs)}</td>
      <td className="stack-inline">
        <button
          className="btn"
          disabled={m.blockers.length > 0}
          title={m.blockers.map(blockerText).join('\n') || t('research.researchTitle', { n: m.targetLevel, pct: Math.round(researchBonus(m.targetLevel) * 100) })}
          onClick={() => research(villageId, unit)}
        >
          {maxed ? t('research.max') : m.currentLevel === 0 ? t('research.unlock') : t('research.upgradeTo', { n: m.targetLevel })}
        </button>
      </td>
    </tr>
  );
}

function ActiveResearch() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const cancel = useGameStore((s) => s.cancelResearchJob);
  useT();
  const json = useGameStore((s) => {
    const j = selectActiveVillage(s).researchQueue[0];
    if (!j) return '';
    const now = selectNow(s);
    const ratio = Math.min(1, Math.max(0, (now - j.startedAt) / Math.max(1, j.completesAt - j.startedAt)));
    return JSON.stringify({ id: j.id, unit: j.unit, targetLevel: j.targetLevel, completesAt: j.completesAt, ratio: Math.round(ratio * 1000) / 1000 });
  });
  if (!json) return <p className="text-muted italic text-[12px]">{t('research.idle')}</p>;
  const j = JSON.parse(json) as { id: string; unit: UnitId; targetLevel: number; completesAt: number; ratio: number };
  return (
    <table className="table-game table-stack">
      <thead><tr><th>{t('research.colResearch')}</th><th>{t('research.colProgress')}</th><th>{t('research.colLeft')}</th><th>{t('research.colAction')}</th></tr></thead>
      <tbody>
        <tr>
          <td className="font-semibold">{t('research.toLevel', { unit: unitName(j.unit), n: j.targetLevel })}</td>
          <td className="w-48 max-w-full" data-label={t('research.colProgress')}><ProgressBar ratio={j.ratio} label={`${Math.floor(j.ratio * 100)}%`} /></td>
          <td data-label={t('research.colLeft')}><Countdown until={j.completesAt} /></td>
          <td className="stack-inline"><button className="btn" onClick={() => cancel(villageId, j.id)} title={t('research.refund')}>{t('research.cancel')}</button></td>
        </tr>
      </tbody>
    </table>
  );
}

export function ResearchScreen() {
  const units = UNIT_IDS.filter((u) => u !== 'noble');
  useT();
  return (
    <div>
      <table className="table-game table-stack">
        <thead><tr><th>{t('research.colUnit')}</th><th>{t('research.colLevel')}</th><th>{t('research.colCost')}</th><th>{t('research.colTime')}</th><th>{t('research.colAction')}</th></tr></thead>
        <tbody>{units.map((u) => <ResearchRow key={u} unit={u} />)}</tbody>
      </table>
      <h3 className="text-[15px] mt-4 mb-1">{t('research.running')}</h3>
      <ActiveResearch />
    </div>
  );
}
