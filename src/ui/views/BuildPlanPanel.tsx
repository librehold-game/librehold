import { buildingName, templateName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { useState } from 'react';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import type { BuildingId } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';

/** Build templates: loading a plan into a village + a simple editor. */
export function BuildPlanPanel() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const plan = useGameStore((s) => (selectActiveVillage(s).buildPlan ?? []).join(','));
  const templates = useGameStore((s) => JSON.stringify(s.game!.buildTemplates));
  const apply = useGameStore((s) => s.applyBuildTemplate);
  const clear = useGameStore((s) => s.clearPlan);
  const save = useGameStore((s) => s.saveBuildTemplate);
  const del = useGameStore((s) => s.deleteBuildTemplate);
  const list = JSON.parse(templates) as { id: string; name: string; steps: BuildingId[] }[];
  const [tpl, setTpl] = useState(list[0]?.id ?? '');
  const [editing, setEditing] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftSteps, setDraftSteps] = useState<BuildingId[]>([]);
  const steps = plan ? (plan.split(',') as BuildingId[]) : [];
  const t = useT();

  return (
    <div className="mt-4 text-[12px]">
      <h3 className="text-[15px] mb-1">{t('plan.title')} <span className="text-muted font-body text-[12px]">{t('plan.hint')}</span></h3>
      <div className="flex items-center gap-2 flex-wrap">
        <select className="px-1 border border-wood-border rounded bg-white/50" value={tpl} onChange={(e) => setTpl(e.target.value)}>{list.map((x) => <option key={x.id} value={x.id}>{templateName(x)} ({x.steps.length})</option>)}</select>
        <button className="btn" disabled={!tpl} onClick={() => apply(villageId, tpl)}>{t('plan.apply')}</button>
        <button className="btn" onClick={() => { const cur = list.find((x) => x.id === tpl); setDraftName(cur ? t('plan.copy', { name: templateName(cur) }) : t('plan.newTemplate')); setDraftSteps(cur ? [...cur.steps] : []); setEditing(true); }}>{t('plan.edit')}</button>
        {list.length > 0 && tpl && <button className="btn" onClick={() => { del(tpl); setTpl(list.find((x) => x.id !== tpl)?.id ?? ''); }}>{t('plan.delete')}</button>}
        {steps.length > 0 && <span className="ml-auto">{t('plan.inPlan', { n: steps.length, next: steps.slice(0, 5).map((b) => buildingName(b)).join(', ') + (steps.length > 5 ? '…' : '') })} <button className="btn !px-1.5" onClick={() => clear(villageId)}>{t('plan.clear')}</button></span>}
      </div>
      {editing && (
        <div className="panel p-2 mt-2 space-y-2">
          <input className="px-1 border border-wood-border rounded bg-white/50 w-64" value={draftName} onChange={(e) => setDraftName(e.target.value)} />
          <div className="flex flex-wrap gap-1">
            {BUILDING_ORDER.map((b) => <button key={b} className="btn !px-1.5" onClick={() => setDraftSteps((d) => [...d, b])}>+{buildingName(b)}</button>)}
          </div>
          <div className="flex flex-wrap gap-1 min-h-[24px]">
            {draftSteps.map((b, i) => <span key={i} className="px-1 border border-wood-border rounded bg-white/50 cursor-pointer" title={t('plan.clickToRemove')} onClick={() => setDraftSteps((d) => d.filter((_, j) => j !== i))}>{i + 1}. {buildingName(b)}</span>)}
            {draftSteps.length === 0 && <span className="text-muted italic">{t('plan.addSteps')}</span>}
          </div>
          <div className="flex gap-2">
            <button className="btn" disabled={!draftName.trim() || draftSteps.length === 0} onClick={() => { const id = `tpl_${Date.now().toString(36)}`; save({ id, name: draftName.trim(), steps: draftSteps }); setTpl(id); setEditing(false); }}>{t('plan.save')}</button>
            <button className="btn" onClick={() => setEditing(false)}>{t('plan.cancel')}</button>
          </div>
        </div>
      )}
    </div>
  );
}
