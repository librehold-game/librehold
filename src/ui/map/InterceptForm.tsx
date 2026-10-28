import { useMemo, useState } from 'react';
import { useT } from '../../i18n/i18n';
import { unitName } from '../../i18n/names';
import { slowestSpeed } from '../../engine/commands';
import { meetingTime } from '../../engine/fieldGeometry';
import { humanVillageIds } from '../../engine/players';
import { UNIT_IDS, type UnitId } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { UnitIcon } from '../components/UnitIcon';
import { useClock } from '../useClock';
import { villageLabelOf } from '../villageName';
import type { MapArmy } from './armies';

type Counts = Partial<Record<UnitId, number>>;

interface OwnVillage { id: string; label: string; x: number; y: number; units: Counts }

/** When an army leaving (x, y) with these units would meet the target, or null when it cannot in time. */
function meeting(army: MapArmy, v: { x: number; y: number }, units: Counts, unitSpeed: number, now: number): number | null {
  const speed = slowestSpeed(units);
  if (speed <= 0) return null;
  // Map armies are drawn between village centres; the rules measure between village corners.
  const from = { x: army.from.x - 0.5, y: army.from.y - 0.5 }, to = { x: army.to.x - 0.5, y: army.to.y - 0.5 };
  return meetingTime(from, to, army.departedAt, army.arrivesAt, v, speed, unitSpeed, now)?.t ?? null;
}

/** Sends troops from one of the player's villages to scout or attack a moving army. */
export function InterceptForm({ army, scout, onDone }: { army: MapArmy; scout: boolean; onDone: () => void }) {
  const t = useT();
  const clock = useClock();
  const json = useGameStore((s) => {
    const g = s.game!;
    const own: OwnVillage[] = humanVillageIds(g).map((id) => g.villages[id]).filter((v) => !!v)
      .map((v) => ({ id: v!.id, label: villageLabelOf(g, v!), x: v!.x, y: v!.y, units: { ...v!.units } }));
    return JSON.stringify(own);
  });
  const villages = useMemo(() => JSON.parse(json) as OwnVillage[], [json]);
  const now = useGameStore((s) => s.game!.now);
  const unitSpeed = useGameStore((s) => s.game!.config.unitSpeed);
  const intercept = useGameStore((s) => s.intercept);
  const allowed = (u: UnitId) => (scout ? u === 'scout' : u !== 'noble');
  const firstChoice = useMemo(() => {
    let best: string | null = null, bestAt = Infinity;
    for (const v of villages) {
      const all: Counts = {};
      for (const u of UNIT_IDS) if (allowed(u) && (v.units[u] ?? 0) > 0) all[u] = v.units[u];
      const at = meeting(army, v, all, unitSpeed, now);
      if (at !== null && at < bestAt) { best = v.id; bestAt = at; }
    }
    return best ?? villages[0]?.id ?? '';
  }, [villages]);
  const [originId, setOriginId] = useState(firstChoice);
  const [units, setUnits] = useState<Counts>({});
  const [error, setError] = useState<string | null>(null);
  const origin = villages.find((v) => v.id === originId);
  const chosen: Counts = Object.fromEntries(Object.entries(units).filter(([, n]) => (n ?? 0) > 0));
  const meetsAt = origin && Object.keys(chosen).length > 0 ? meeting(army, origin, chosen, unitSpeed, now) : null;
  const tooLate = Object.keys(chosen).length > 0 && meetsAt === null;

  return (
    <form
      className="mt-2 border-t border-wood-border/60 pt-1.5"
      onSubmit={(e) => {
        e.preventDefault();
        if (!origin || meetsAt === null) return;
        void intercept(origin.id, army.id, chosen, scout).then((err) => { if (err) setError(err); else onDone(); });
      }}
    >
      <div className="font-semibold">{scout ? t('armyCard.scout') : t('armyCard.attackField')}</div>
      {villages.length > 1 && (
        <label className="block my-1">
          {t('armyCard.from')}{' '}
          <select value={originId} onChange={(e) => { setOriginId(e.target.value); setUnits({}); }}>
            {villages.map((v) => <option key={v.id} value={v.id}>{`${v.label} (${v.x}|${v.y})`}</option>)}
          </select>
        </label>
      )}
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        {origin && UNIT_IDS.filter((u) => allowed(u) && (origin.units[u] ?? 0) > 0).map((u) => (
          <label key={u} className="inline-flex items-center gap-1" title={unitName(u)}>
            <UnitIcon unit={u} size={14} />
            <input
              type="number" min={0} max={origin.units[u]} value={units[u] ?? ''} aria-label={unitName(u)}
              className="w-14 px-1 border border-wood-border rounded bg-white/50"
              onChange={(e) => setUnits({ ...units, [u]: Math.max(0, Math.min(origin.units[u] ?? 0, Number(e.target.value) || 0)) })}
            />
            <span className="text-muted text-[10px]">/{origin.units[u]}</span>
          </label>
        ))}
      </div>
      <p className="my-1 tabular-nums">
        {tooLate ? <span className="text-bad">{t('armyCard.tooLate')}</span>
          : meetsAt !== null ? t('armyCard.meetsAt', { at: clock.at(meetsAt) }) : null}
      </p>
      {error && <p className="text-bad">{error}</p>}
      <button className="btn" type="submit" disabled={meetsAt === null}>{t('armyCard.send')}</button>
    </form>
  );
}
