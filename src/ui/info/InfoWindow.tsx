import { useEffect, useRef, type ReactNode } from 'react';
import { buildingName, unitName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { effectiveBuildingLevels } from '../../engine/queues';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { BuildingEntry, UnitEntry, type VillageProgress } from '../../shared/encyclopedia/entries';
import { EntryLinkContext, type EntryRef } from '../../shared/encyclopedia/EntryLink';
import { BuildingSprite, UnitSprite } from '../../shared/encyclopedia/sections';
import { Overlay } from '../instant/Overlay';
import { InfoLink } from './InfoLink';
import { worldTiming } from './worldTiming';

const renderLink = (to: EntryRef, children: ReactNode) => <InfoLink to={to}>{children}</InfoLink>;

/** The details window of a building or unit, with this village's progress and this world's times; links inside it stack. */
export function InfoWindow() {
  const t = useT();
  const stack = useUiStore((s) => s.info);
  const back = useUiStore((s) => s.infoBack);
  const close = useUiStore((s) => s.closeInfo);
  const game = useGameStore((s) => s.game);
  const village = useGameStore((s) => (s.game ? selectActiveVillage(s) : null));
  const open = stack.length > 0;

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') close(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, close]);

  const heading = useRef<HTMLHeadingElement>(null);
  const top = stack[stack.length - 1];
  const topKey = top ? `${top.kind}:${top.id}` : '';

  useEffect(() => {
    if (!open) return;
    const opener = document.activeElement;
    return () => { if (opener instanceof HTMLElement && opener.isConnected) opener.focus(); };
  }, [open]);

  useEffect(() => { heading.current?.focus(); }, [topKey]);

  if (!top || !game || !village) return null;
  const timing = worldTiming(game, village);
  const progress: VillageProgress = {
    levels: top.kind === 'unit' ? village.buildings : effectiveBuildingLevels(village),
    research: village.research,
    unitCount: top.kind === 'unit' ? village.units[top.id] : undefined,
  };
  const name = top.kind === 'unit' ? unitName(top.id) : buildingName(top.id);
  return (
    <Overlay titleId="info-title" wide onClose={close}>
      <EntryLinkContext.Provider value={renderLink}>
        <div className="info-entry p-3">
          <div className="flex items-center gap-3 mb-2">
            {stack.length > 1 && <button type="button" className="btn" onClick={back}>{t('info.back')}</button>}
            {top.kind === 'unit' ? <UnitSprite id={top.id} size={128} /> : <BuildingSprite id={top.id} size={96} />}
            <h2 id="info-title" ref={heading} tabIndex={-1} className="text-lg flex-1 outline-none">{name}</h2>
            <button type="button" className="btn" aria-label={t('info.close')} onClick={close}>✕</button>
          </div>
          {top.kind === 'unit'
            ? <UnitEntry id={top.id} timing={timing} progress={progress} />
            : <BuildingEntry id={top.id} timing={timing} progress={progress} />}
        </div>
      </EntryLinkContext.Provider>
    </Overlay>
  );
}
