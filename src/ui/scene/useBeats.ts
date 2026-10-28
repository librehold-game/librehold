import { useEffect, useRef, useState } from 'react';
import { useGameStore } from '../../state/gameStore';
import { RECRUIT_BUILDINGS, UNIT_IDS, type RecruitBuilding, type UnitId } from '../../engine/types';
import { mergeBeat, SCENE_BALANCE, type Beat } from './sceneModel';
import { humanCoins } from '../../engine/players';

export interface LiveBeat extends Beat { key: string; unit: UnitId | null; kind: 'unit' | 'research' | 'coin' | 'build' }

interface Snapshot {
  jobs: Record<string, { building: RecruitBuilding; unit: UnitId; done: number; total: number }>;
  research: number;
  coins: number;
  levels: number;
}

function snapshotOf(villageId: string): string {
  const g = useGameStore.getState().game;
  const v = g?.villages[villageId];
  if (!g || !v) return '';
  const jobs: Snapshot['jobs'] = {};
  for (const b of RECRUIT_BUILDINGS) for (const j of v.recruitQueues[b]) jobs[j.id] = { building: b, unit: j.unit, done: j.done, total: j.total };
  let research = 0;
  for (const u of UNIT_IDS) research += v.research[u];
  let levels = 0;
  for (const l of Object.values(v.buildings)) levels += l;
  return JSON.stringify({ jobs, research, coins: humanCoins(g), levels } satisfies Snapshot);
}

/**
 * Beats (16.6.4): detects a finished unit/research/coin by comparing queue snapshots.
 * Purely presentational: never writes to the store. After a skip of > 60 s beats are dropped (current state, not history).
 */
export function useBeats(villageId: string, enabled: boolean): LiveBeat[] {
  const [beats, setBeats] = useState<LiveBeat[]>([]);
  const prev = useRef<string>(snapshotOf(villageId));
  const last = useRef<Map<string, Beat>>(new Map());
  const seq = useRef(0);

  useEffect(() => {
    prev.current = snapshotOf(villageId);
    last.current.clear();
    setBeats([]);
    if (!enabled) return;
    const unsub = useGameStore.subscribe((s, before) => {
      if (s.syncCounter === before.syncCounter && s.game === before.game) return;
      const nowJson = snapshotOf(villageId);
      const prevJson = prev.current;
      prev.current = nowJson;
      if (!nowJson || !prevJson || nowJson === prevJson) return;
      if (s.lastSyncSpanMs > SCENE_BALANCE.beatClearSpanMs) { setBeats([]); return; }
      const a = JSON.parse(prevJson) as Snapshot;
      const b = JSON.parse(nowJson) as Snapshot;
      const fresh: LiveBeat[] = [];
      const nowMs = performance.now();
      const push = (building: string, kind: LiveBeat['kind'], unit: UnitId | null, units: number) => {
        const merged = mergeBeat(last.current.get(building), building, nowMs, units);
        last.current.set(building, merged);
        if (merged.at !== nowMs) {
          setBeats((cur) => cur.map((x) => (x.building === building && x.at === merged.at ? { ...x, count: merged.count } : x)));
          return;
        }
        fresh.push({ ...merged, key: `${building}-${seq.current++}`, unit, kind });
      };
      for (const [id, job] of Object.entries(a.jobs)) {
        const cur = b.jobs[id];
        const produced = cur ? cur.done - job.done : job.total - job.done;
        if (produced > 0) push(job.building, 'unit', job.unit, produced);
      }
      if (b.research > a.research) push('smithy', 'research', null, 1);
      if (b.coins > a.coins) push('academy', 'coin', null, 1);
      if (b.levels > a.levels) push('main', 'build', null, 1);
      if (fresh.length) {
        setBeats((cur) => [...cur, ...fresh]);
        window.setTimeout(() => setBeats((cur) => cur.filter((x) => !fresh.includes(x))), 3000);
      }
    });
    return unsub;
  }, [villageId, enabled]);

  return beats;
}

export { useDocumentHidden } from '../useDocumentHidden';
