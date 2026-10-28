import { useEffect } from 'react';
import { useGameStore } from '../state/gameStore';
import { useUiStore } from '../state/uiStore';
import { SCENE_BALANCE } from './scene/sceneModel';
import { playAlarm, playBuildDone, playFanfare, playQuest } from './sound';
import { humanVillageIds } from '../engine/players';

interface Snap { levels: number; attackReports: number; nobles: number; quests: number }

function snap(): Snap | null {
  const g = useGameStore.getState().game;
  if (!g) return null;
  let levels = 0, nobles = 0;
  for (const id of humanVillageIds(g)) {
    const v = g.villages[id];
    if (!v) continue;
    for (const l of Object.values(v.buildings)) levels += l;
    nobles += v.units.noble;
  }
  let attackReports = 0;
  for (const r of g.reports) if (r.type === 'attack' && humanVillageIds(g).includes(r.targetId)) attackReports++;
  return { levels, attackReports, nobles, quests: g.quests.completed.length };
}

/** Event sounds (15.4): snapshots compared after every sync; skipped after a skip of > 60 s. */
export function useGameSounds(): void {
  useEffect(() => {
    let prev = snap();
    return useGameStore.subscribe((s, before) => {
      if (s.syncCounter === before.syncCounter && s.game === before.game) return;
      const cur = snap();
      const old = prev;
      prev = cur;
      if (!cur || !old) return;
      if (!useUiStore.getState().prefs.sound) return;
      if (s.lastSyncSpanMs > SCENE_BALANCE.beatClearSpanMs) return;
      if (cur.nobles > old.nobles) playFanfare();
      else if (cur.attackReports > old.attackReports) playAlarm();
      else if (cur.levels > old.levels) playBuildDone();
      else if (cur.quests > old.quests) playQuest();
    });
  }, []);
}
