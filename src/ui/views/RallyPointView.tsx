import { buildingName, unitName } from '../../i18n/names';
import { useEffect, useMemo, useState } from 'react';
import { useShallow } from 'zustand/react/shallow';
import { BUILDINGS, BUILDING_ORDER } from '../../data/buildings.data';
import { UNITS } from '../../data/units.data';
import { canCancelCommand, commandHome, playerCommands, type SendBlocker, type SendCheck, type UnitCounts } from '../../engine/commands';
import { checkSendClient } from './rallyCheck';
import { HUMAN_ID, UNIT_IDS, type BuildingId, type Command, type UnitId } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { useUiStore } from '../../state/uiStore';
import { Countdown } from '../components/Countdown';
import { UnitCountList } from '../components/UnitCountList';
import { UnitIcon } from '../components/UnitIcon';
import { IncomingList } from '../components/IncomingList';
import { formatCoords, formatDuration, formatDurationHuman, formatNumber, localDateInput, parseLocalTime } from '../format';
import { useClock } from '../useClock';
import { isHumanVillage } from '../../engine/players';
import { t, useT } from '../../i18n/i18n';
import { RichText } from '../components/RichText';
import { COMMAND_ICON, commandEnds, villageLabelOf } from '../villageName';

type Tab = 'send' | 'commands' | 'templates';

function blockerText(b: SendBlocker, now: number): string {
  switch (b.kind) {
    case 'notEnough': return t('rally.b.notEnough', { unit: unitName(b.unit), have: b.have });
    case 'nobleTooFar': return t('rally.b.nobleTooFar', { max: b.max });
    case 'targetProtected': return t('rally.b.targetProtected', { time: formatDurationHuman(Math.max(0, b.until - now)) });
    default: return t(`rally.b.${b.kind}`);
  }
}

type SendMode = 'now' | 'sendAt' | 'arriveAt';

/** The player's scheduled sends — with cancelling. */
function ScheduledList() {
  const clock = useClock();
  const json = useGameStore((s) => {
    const g = s.game!;
    return JSON.stringify(g.scheduled.filter((x) => x.ownerId === HUMAN_ID).map((x) => ({
      id: x.id, from: g.villages[x.originId] ? villageLabelOf(g, g.villages[x.originId]!) : x.originId, x: x.x, y: x.y, type: x.type, mode: x.mode, at: x.at, sendAt: x.sendAt, units: x.units,
    })));
  });
  const cancel = useGameStore((s) => s.cancelScheduled);
  useT();
  const rows = JSON.parse(json) as { id: string; from: string; x: number; y: number; type: 'attack' | 'support'; mode: 'sendAt' | 'arriveAt'; at: number; sendAt: number; units: UnitCounts }[];
  if (rows.length === 0) return null;
  return (
    <div className="mt-4">
      <h3 className="text-[15px] mb-1">{t('rally.scheduled')}</h3>
      <div className="overflow-x-auto"><table className="table-game w-full text-[12px]">
        <thead><tr>{(['from', 'target', 'kind', 'leaves', 'arrives', 'troops'] as const).map((c) => <th key={c}>{t(`rally.col.${c}`)}</th>)}<th /></tr></thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td data-label={t('rally.col.from')}>{r.from}</td>
              <td data-label={t('rally.col.target')}>{formatCoords(r.x, r.y)}</td>
              <td data-label={t('rally.col.kind')}>{t(r.type === 'attack' ? 'rally.attack' : 'rally.support')}</td>
              <td data-label={t('rally.col.leaves')} className="tabular-nums">{clock.at(r.sendAt, true)}</td>
              <td data-label={t('rally.col.arrives')} className="tabular-nums">{r.mode === 'arriveAt' ? clock.at(r.at, true) : '—'}</td>
              <td data-label={t('rally.col.troops')}><UnitCountList units={r.units} /></td>
              <td data-label=""><button className="btn" onClick={() => cancel(r.id)}>{t('rally.recall')}</button></td>
            </tr>
          ))}
        </tbody>
      </table></div>
    </div>
  );
}

function SendTab() {
  const clock = useClock();
  const origin = useGameStore((s) => s.game!.clock.originLocalMs);
  const village = useGameStore(useShallow((s) => { const v = selectActiveVillage(s); return { id: v.id, x: v.x, y: v.y }; }));
  const units = useGameStore((s) => JSON.stringify(selectActiveVillage(s).units));
  const have = JSON.parse(units) as Record<UnitId, number>;
  const consume = useUiStore((s) => s.consumeRallyPrefill);
  const send = useGameStore((s) => s.send);
  const schedule = useGameStore((s) => s.scheduleSend);
  const [mode, setMode] = useState<SendMode>('now');
  const [whenDate, setWhenDate] = useState('');
  const [whenTime, setWhenTime] = useState('');
  const [tx, setTx] = useState('');
  const [ty, setTy] = useState('');
  const [counts, setCounts] = useState<UnitCounts>({});
  const [catTarget, setCatTarget] = useState<BuildingId | ''>('');
  const [msg, setMsg] = useState<string | null>(null);
  useT();

  useEffect(() => {
    const p = consume();
    if (p) {
      setTx(String(p.x));
      setTy(String(p.y));
      if (p.units) setCounts(p.units);
    }
  }, [consume]);

  const x = Number(tx), y = Number(ty);
  const coordsOk = tx !== '' && ty !== '' && Number.isInteger(x) && Number.isInteger(y);
  const check = useGameStore((s) => {
    if (!s.game || !coordsOk) return null;
    return JSON.stringify({
      attack: checkSendClient(s.game, village.id, x, y, counts, 'attack'),
      support: checkSendClient(s.game, village.id, x, y, counts, 'support'),
      now: Math.floor(s.game.now / 1000) * 1000,
    });
  });
  const parsed = check ? (JSON.parse(check) as { attack: SendCheck; support: SendCheck; now: number }) : null;

  const available = UNIT_IDS.filter((u) => have[u] > 0 || (counts[u] ?? 0) > 0);
  const setCount = (u: UnitId, n: number) => setCounts((c) => ({ ...c, [u]: Math.max(0, Math.min(have[u], Math.floor(n) || 0)) }));

  const at = mode === 'now' || !parsed ? null : parseLocalTime(origin, parsed.now, whenDate, whenTime);
  const doSend = async (type: 'attack' | 'support') => {
    if (mode !== 'now') {
      if (at === null) { setMsg(t('rally.needTime')); return; }
      const err = await schedule(village.id, x, y, counts, type, mode, at, catTarget || undefined);
      setMsg(err ?? t('rally.planned', { kind: t(type === 'attack' ? 'rally.attack' : 'rally.support'), coords: formatCoords(x, y) }));
      if (!err) setCounts({});
      return;
    }
    const err = await send(village.id, x, y, counts, type, catTarget || undefined);
    if (err) setMsg(err);
    else {
      setMsg(t('rally.sent', { kind: t(type === 'attack' ? 'rally.attack' : 'rally.support'), coords: formatCoords(x, y) }));
      setCounts({});
    }
  };

  return (
    <div className="grid md:grid-cols-2 gap-4 [&>*]:min-w-0">
      <div>
        <h3 className="text-[15px] mb-1">{t('rally.target')}</h3>
        <div className="flex items-center gap-1 mb-2">
          <input className="w-16 px-1 border border-wood-border rounded bg-white/50 tabular-nums" placeholder="x" value={tx} onChange={(e) => setTx(e.target.value)} />
          <span>|</span>
          <input className="w-16 px-1 border border-wood-border rounded bg-white/50 tabular-nums" placeholder="y" value={ty} onChange={(e) => setTy(e.target.value)} />
          <span className="text-muted text-[11px] ml-2">{t('rally.from', { coords: formatCoords(village.x, village.y) })}</span>
        </div>
        <h3 className="text-[15px] mb-1">{t('rally.units')}</h3>
        {available.length === 0 ? (
          <p className="text-muted italic text-[12px]">{t('rally.noTroops')}</p>
        ) : (
          <div className="overflow-x-auto"><table className="table-game">
            <tbody>
              {available.map((u) => (
                <tr key={u}>
                  <td className="flex items-center gap-1"><UnitIcon unit={u} />{unitName(u)}</td>
                  <td>
                    <input type="number" min={0} max={have[u]} className="w-20 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
                      value={counts[u] ?? 0} onChange={(e) => setCount(u, Number(e.target.value))} />
                  </td>
                  <td><button className="btn !px-1.5" onClick={() => setCount(u, have[u])}>({formatNumber(have[u])})</button></td>
                </tr>
              ))}
            </tbody>
          </table></div>
        )}
        <div className="flex gap-1 mt-1">
          <button className="btn" onClick={() => { const all: UnitCounts = {}; for (const u of UNIT_IDS) if (have[u] > 0) all[u] = have[u]; setCounts(all); }}>{t('rally.all')}</button>
          <button className="btn" onClick={() => setCounts({})}>{t('rally.none')}</button>
        </div>
        {(counts.catapult ?? 0) > 0 && (
          <label className="block mt-2 text-[12px]">
            {t('rally.catTarget')}{' '}
            <select className="px-1 border border-wood-border rounded bg-white/50" value={catTarget} onChange={(e) => setCatTarget(e.target.value as BuildingId | '')}>
              <option value="">{t('rally.randomBuilding')}</option>
              {BUILDING_ORDER.map((b) => <option key={b} value={b}>{buildingName(b)}</option>)}
            </select>
          </label>
        )}
      </div>
      <div>
        <h3 className="text-[15px] mb-1">{t('rally.preview')}</h3>
        {parsed ? (
          <div className="text-[12px] space-y-1">
            <div><RichText text={t('rally.distance', { n: parsed.attack.distance.toFixed(1) })} /></div>
            <div>{t('rally.travel')} <strong className="tabular-nums">{formatDuration(parsed.attack.travelMs)}</strong></div>
            <div>{t('rally.arrival')} <span className="tabular-nums">{clock.at(parsed.now + parsed.attack.travelMs)}</span></div>
            {parsed.attack.isScoutMission && <div className="text-muted">{t('rally.scoutMission')}</div>}
            <div className="flex items-center gap-2 flex-wrap pt-1">
              <label htmlFor="rally-mode">{t('rally.when')}</label>
              <select id="rally-mode" className="px-1 border border-wood-border rounded bg-white/50" value={mode} onChange={(e) => setMode(e.target.value as SendMode)}>
                {(['now', 'sendAt', 'arriveAt'] as const).map((k) => <option key={k} value={k}>{t(`rally.mode.${k}`)}</option>)}
              </select>
              {mode !== 'now' && (
                <>
                  <label htmlFor="rally-date">{t('rally.on')}</label>
                  <input id="rally-date" type="date" className="px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={whenDate || localDateInput(origin, parsed.now)} onChange={(e) => setWhenDate(e.target.value)} />
                  <label htmlFor="rally-time">{t('rally.at')}</label>
                  <input id="rally-time" className="w-28 px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={whenTime} placeholder="14:05:30.250" onChange={(e) => setWhenTime(e.target.value)} />
                </>
              )}
            </div>
            {mode !== 'now' && at !== null && (
              <div className="tabular-nums">
                {mode === 'sendAt'
                  ? t('rally.leavesArrives', { leave: clock.at(at, true), arrive: clock.at(at + parsed.attack.travelMs, true) })
                  : t('rally.leavesArrives', { leave: clock.at(at - parsed.attack.travelMs, true), arrive: clock.at(at, true) })}
              </div>
            )}
            {parsed.attack.warnings.map((w) => <div key={w.kind} className="text-bad font-semibold">⚠ {blockerText(w, parsed.now)}</div>)}
            <div className="flex gap-2 pt-2">
              <button className="btn" disabled={parsed.attack.blockers.length > 0} title={parsed.attack.blockers.map((b) => blockerText(b, parsed.now)).join('\n')} onClick={() => doSend('attack')}>{t('rally.attackBtn')}</button>
              <button className="btn" disabled={parsed.support.blockers.length > 0} title={parsed.support.blockers.map((b) => blockerText(b, parsed.now)).join('\n')} onClick={() => doSend('support')}>{t('rally.supportBtn')}</button>
            </div>
            {parsed.attack.blockers.length > 0 && (
              <ul className="text-muted list-disc pl-4 pt-1">{parsed.attack.blockers.map((b, i) => <li key={i}>{blockerText(b, parsed.now)}</li>)}</ul>
            )}
          </div>
        ) : (
          <p className="text-muted italic text-[12px]">{t('rally.enterTarget')}</p>
        )}
        {msg && <p className="mt-2 text-[12px] font-semibold">{msg}</p>}
        <ScheduledList />
      </div>
    </div>
  );
}

function CommandRow({ cmd }: { cmd: Command }) {
  const clock = useClock();
  const names = useGameStore((s) => {
    const g = s.game!;
    const o = g.villages[cmd.originId], to = g.villages[cmd.targetId];
    const ends = commandEnds(g, cmd);
    return JSON.stringify({
      o: ends.from,
      t: ends.to,
      canCancel: canCancelCommand(g, cmd) && o !== undefined && isHumanVillage(o),
      incoming: o !== undefined && !isHumanVillage(o) && to !== undefined && isHumanVillage(to),
    });
  });
  const { o, t: to, canCancel, incoming } = JSON.parse(names) as { o: string; t: string; canCancel: boolean; incoming: boolean };
  useT();
  const cancel = useGameStore((s) => s.cancelCmd);
  const icon = COMMAND_ICON[cmd.type];
  const label = t(`rally.cmd.${cmd.type}`);
  return (
    <tr className={incoming ? 'text-bad font-semibold' : ''}>
      <td className="whitespace-nowrap">{icon} {incoming ? t('rally.incoming') : ''}{label}</td>
      <td className="text-[11px]" data-label={t('rally.col.route')}>{o}<br />→ {to}</td>
      <td data-label={t('rally.col.units')}><UnitCountList units={cmd.units} /></td>
      <td className="tabular-nums" data-label={t('rally.col.arrives')}><Countdown until={cmd.arrivesAt} /><div className="text-[10px] text-muted">{clock.at(cmd.arrivesAt)}</div></td>
      <td className="stack-inline">
        {cmd.type !== 'return' && cmd.type !== 'intercept' && !incoming && (
          <button className="btn" disabled={!canCancel} title={canCancel ? t('rally.turnBack') : t('rally.cancelWindow')} onClick={() => cancel(cmd.id)}>{t('rally.recall')}</button>
        )}
      </td>
    </tr>
  );
}

function CommandsTab() {
  const [onlyActive, setOnlyActive] = useState(true);
  const activeId = useGameStore((s) => selectActiveVillage(s).id);
  const json = useGameStore((s) => {
    const g = s.game!;
    const list = playerCommands(g).filter((c) => !onlyActive || c.originId === activeId || c.targetId === activeId || commandHome(c) === activeId);
    return JSON.stringify([...list].sort((a, b) => a.arrivesAt - b.arrivesAt));
  });
  const cmds = JSON.parse(json) as Command[];
  const supports = useGameStore((s) => JSON.stringify(selectActiveVillage(s).supportFrom));
  const supportFrom = JSON.parse(supports) as Record<string, UnitCounts>;
  const withdraw = useGameStore((s) => s.withdraw);
  const game = useGameStore((s) => s.game!);
  useT();
  return (
    <div>
      <label className="text-[12px] inline-flex items-center gap-1 mb-2">
        <input type="checkbox" checked={onlyActive} onChange={(e) => setOnlyActive(e.target.checked)} /> {t('rally.onlyActive')}
      </label>
      {cmds.length === 0 ? (
        <p className="text-muted italic text-[12px]">{t('rally.noneMoving')}</p>
      ) : (
        <table className="table-game table-stack">
          <thead><tr><th>{t('rally.col.type')}</th><th>{t('rally.col.route')}</th><th>{t('rally.col.units')}</th><th>{t('rally.col.arrives')}</th><th>{t('rally.col.action')}</th></tr></thead>
          <tbody>{cmds.map((c) => <CommandRow key={c.id} cmd={c} />)}</tbody>
        </table>
      )}
      {Object.keys(supportFrom).length > 0 && (
        <div className="mt-3">
          <h3 className="text-[15px] mb-1">{t('rally.stationed')}</h3>
          <div className="overflow-x-auto"><table className="table-game">
            <tbody>
              {Object.entries(supportFrom).map(([homeId, u]) => (
                <tr key={homeId}>
                  <td>{game.villages[homeId] ? `${villageLabelOf(game, game.villages[homeId]!)} ${formatCoords(game.villages[homeId]!.x, game.villages[homeId]!.y)}` : homeId}</td>
                  <td><UnitCountList units={u} /></td>
                  <td><button className="btn" onClick={() => withdraw(activeId, homeId)}>{t('rally.sendBack')}</button></td>
                </tr>
              ))}
            </tbody>
          </table></div>
        </div>
      )}
    </div>
  );
}

function TemplatesTab() {
  const templates = useGameStore((s) => JSON.stringify(s.game!.farmAssistant.templates));
  const setTemplate = useGameStore((s) => s.setTemplate);
  const parsed = JSON.parse(templates) as Record<'A' | 'B' | 'C' | 'D', { units: UnitCounts }>;
  const [drafts, setDrafts] = useState<Record<'A' | 'B' | 'C' | 'D', UnitCounts>>(() => ({
    A: parsed.A.units, B: parsed.B.units, C: parsed.C.units, D: parsed.D.units,
  }));
  const units = useMemo(() => UNIT_IDS.filter((u) => u !== 'noble'), []);
  useT();
  return (
    <div>
      <p className="text-[12px] text-muted mb-2">{t('rally.templatesHint')}</p>
      <div className="overflow-x-auto"><table className="table-game">
        <thead>
          <tr><th>{t('rally.col.unit')}</th>{(['A', 'B', 'C', 'D'] as const).map((k) => <th key={k}>{t('rally.template', { k })}</th>)}</tr>
        </thead>
        <tbody>
          {units.map((u) => (
            <tr key={u}>
              <td className="flex items-center gap-1"><UnitIcon unit={u} />{unitName(u)}</td>
              {(['A', 'B', 'C', 'D'] as const).map((k) => (
                <td key={k}>
                  <input type="number" min={0} className="w-20 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
                    value={drafts[k][u] ?? 0}
                    onChange={(e) => setDrafts((d) => ({ ...d, [k]: { ...d[k], [u]: Math.max(0, Math.floor(Number(e.target.value) || 0)) } }))} />
                </td>
              ))}
            </tr>
          ))}
          <tr>
            <td></td>
            {(['A', 'B', 'C', 'D'] as const).map((k) => (
              <td key={k}><button className="btn" onClick={() => setTemplate(k, drafts[k])}>{t('rally.save', { k })}</button></td>
            ))}
          </tr>
        </tbody>
      </table></div>
    </div>
  );
}

export function RallyPointView() {
  const prefill = useUiStore((s) => s.rallyPrefill);
  const [tab, setTab] = useState<Tab>(prefill ? 'send' : 'send');
  const commandCount = useGameStore((s) => playerCommands(s.game!).length);
  const activeId = useGameStore((s) => selectActiveVillage(s).id);
  useEffect(() => { if (prefill) setTab('send'); }, [prefill]);
  useT();
  return (
    <div>
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <h2 className="text-lg mr-2">{t('rally.title')}</h2>
        {([['send', t('rally.tab.send')], ['commands', t('rally.tab.commands', { n: commandCount })], ['templates', t('rally.tab.templates')]] as const).map(([id, label]) => (
          <button key={id} className={`btn ${tab === id ? '!bg-panel-dark' : ''}`} onClick={() => setTab(id)}>{label}</button>
        ))}
      </div>
      <IncomingList villageId={activeId} />
      {tab === 'send' && <SendTab />}
      {tab === 'commands' && <CommandsTab />}
      {tab === 'templates' && <TemplatesTab />}
    </div>
  );
}
