import { useEffect, useId, useMemo, useState } from 'react';
import { useT } from '../../i18n/i18n';
import { canCancelCommand } from '../../engine/commands';
import { labelIncoming } from '../../engine/incoming';
import { RESOURCES } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { Countdown } from '../components/Countdown';
import { ResourceIcon } from '../components/ResourceIcon';
import { UnitCountList } from '../components/UnitCountList';
import { formatCoords, formatNumber } from '../format';
import { useClock } from '../useClock';
import { villageLabelById } from '../villageName';
import { FOREIGN_KINDS, INTERCEPTABLE_KINDS, armiesOnMap, type MapArmy } from './armies';
import { InterceptForm } from './InterceptForm';
import { ArmyPreview } from './ArmyPreview';

interface CardData {
  army: MapArmy;
  from: { label: string; owner: string | null };
  to: { label: string; owner: string | null };
  canCancel: boolean;
  fieldHit: boolean;
  label: string | null;
  haul: Partial<Record<(typeof RESOURCES)[number], number>> | null;
}

/** What the player knows about an army on the map and what they can do about it. */
export function ArmyCard({ armyId, sheet, isVillage, onClose }: {
  armyId: string; sheet: boolean; isVillage: (x: number, y: number) => boolean; onClose: () => void;
}) {
  const t = useT();
  const clock = useClock();
  const titleId = useId();
  const json = useGameStore((s) => {
    const g = s.game!;
    const army = armiesOnMap(g).find((a) => a.id === armyId);
    const cmd = g.commands.find((c) => c.id === armyId);
    if (!army || !cmd) return 'null';
    const data: CardData = {
      army,
      from: army.originId ? villageLabelById(g, army.originId) : { label: formatCoords(Math.floor(army.from.x), Math.floor(army.from.y)), owner: null },
      to: army.targetId ? villageLabelById(g, army.targetId) : { label: formatCoords(Math.floor(army.to.x), Math.floor(army.to.y)), owner: null },
      canCancel: !FOREIGN_KINDS.has(army.kind) && canCancelCommand(g, cmd),
      fieldHit: !!cmd.fieldHit,
      label: army.kind === 'incomingAttack' ? labelIncoming(g, cmd) : null,
      haul: army.kind === 'ownReturn' && cmd.haul ? { ...cmd.haul } : null,
    };
    return JSON.stringify(data);
  });
  const data = useMemo(() => JSON.parse(json) as CardData | null, [json]);
  const cancel = useGameStore((s) => s.cancelCmd);
  const dodge = useGameStore((s) => s.dodge);
  const setActive = useGameStore((s) => s.setActiveVillage);
  const navigate = useUiStore((s) => s.navigate);
  const seed = useGameStore((s) => s.game!.worldSeed);
  const [meet, setMeet] = useState<'scout' | 'attack' | null>(null);

  useEffect(() => { if (!data) onClose(); }, [data, onClose]);
  if (!data) return null;
  const { army, from, to } = data;
  const home = army.kind === 'ownReturn' ? army.targetId : army.originId;
  const open = (village: string, view: 'rally' | 'village') => { setActive(village); navigate(view); onClose(); };

  return (
    <div
      role="dialog"
      aria-labelledby={titleId}
      data-army-card={army.id}
      className={`panel shadow-[0_4px_12px_rgba(0,0,0,0.35)] text-[11px] p-2 leading-snug ${
        sheet ? 'fixed inset-x-1 bottom-[calc(64px+env(safe-area-inset-bottom,0px))] z-40 max-h-[60vh] overflow-y-auto' : 'absolute z-20 right-2 top-2 w-[272px]'
      }`}
      onPointerDown={(e) => e.stopPropagation()}
      onClick={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      <div className="flex items-start justify-between gap-1">
        <div id={titleId} className="font-heading text-[14px] leading-tight">{t(`map.army.${army.kind}`)}</div>
        <button className="btn !px-1.5 !py-0 leading-none" onClick={onClose} aria-label={t('map.close')}>{'✕'}</button>
      </div>
      <div data-preview className="my-1"><ArmyPreview army={army} seed={seed} isVillage={isVillage} /></div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
        <dt className="text-muted">{t('armyCard.from')}</dt>
        <dd>{from.label}{from.owner && army.kind.startsWith('incoming') ? ` (${from.owner})` : ''}</dd>
        <dt className="text-muted">{t('armyCard.to')}</dt>
        <dd>{to.label}</dd>
        <dt className="text-muted">{t('armyCard.arrival')}</dt>
        <dd className="tabular-nums">{clock.at(army.arrivesAt)} · <Countdown until={army.arrivesAt} /></dd>
        {data.label && (<><dt className="text-muted">{t('armyCard.threat')}</dt><dd>{t(`incomingLabel.${data.label}`)}</dd></>)}
      </dl>
      {army.units && Object.keys(army.units).length > 0 && <div className="mt-1"><UnitCountList units={army.units} /></div>}
      {data.haul && (
        <div className="mt-1 flex gap-2 tabular-nums">
          {RESOURCES.map((r) => <span key={r} className="inline-flex items-center gap-0.5"><ResourceIcon r={r} size={14} />{formatNumber(data.haul![r] ?? 0)}</span>)}
        </div>
      )}
      <div className="mt-2 flex flex-wrap gap-1">
        {data.canCancel && (
          <button className="btn" onClick={() => { void cancel(army.id).then((err) => { if (!err) onClose(); }); }}>
            {data.fieldHit ? t('armyCard.turnBack') : t('armyCard.cancel')}
          </button>
        )}
        {(army.kind === 'ownAttack' || army.kind === 'ownSupport' || army.kind === 'ownReturn' || army.kind === 'ownIntercept') && (
          <button className="btn" onClick={() => open(home, 'rally')}>{t('armyCard.openRally')}</button>
        )}
        {army.kind === 'incomingAttack' && (
          <>
            <button className="btn" onClick={() => { void dodge(army.id); }} title={t('incoming.dodgeTitle')}>{t('incoming.dodge')}</button>
            <button className="btn" onClick={() => open(army.targetId, 'village')}>{t('armyCard.openTarget')}</button>
          </>
        )}
        {INTERCEPTABLE_KINDS.has(army.kind) && (
          <>
            <button className="btn" aria-pressed={meet === 'scout'} onClick={() => setMeet('scout')}>{t('armyCard.scout')}</button>
            <button className="btn" aria-pressed={meet === 'attack'} onClick={() => setMeet('attack')}>{t('armyCard.attackField')}</button>
          </>
        )}
      </div>
      {meet && INTERCEPTABLE_KINDS.has(army.kind) && <InterceptForm key={meet} army={army} scout={meet === 'scout'} onDone={onClose} />}
    </div>
  );
}
