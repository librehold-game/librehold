import { questDescription, questTitle } from '../../i18n/names';
import { PixelIcon } from '../pixelIcons';
import { useT } from '../../i18n/i18n';
import { villageLabelOf } from '../villageName';
import { useState } from 'react';
import { QUESTS, QUEST_BY_ID, type QuestReward } from '../../data/quests.data';
import { activeQuests, claimableQuests } from '../../engine/quests';
import { RESOURCES } from '../../engine/types';
import { useGameStore } from '../../state/gameStore';
import { ProgressBar } from '../components/ProgressBar';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatNumber } from '../format';

function RewardLabel({ reward }: { reward: QuestReward }) {
  const t = useT();
  return (
    <span className="inline-flex items-center gap-2 flex-wrap">
      {reward.resources && RESOURCES.filter((r) => reward.resources![r] > 0).map((r) => (
        <span key={r} className="inline-flex items-center gap-0.5 tabular-nums"><ResourceIcon r={r} size={14} />{formatNumber(reward.resources![r])}</span>
      ))}
      {reward.buildQueueSlot && <span className="text-ok font-semibold">{t('quests.slot')}</span>}
      {reward.autoFarm && <span className="text-ok font-semibold">{t('quests.autoFarm')}</span>}
    </span>
  );
}

interface Row { id: string; progress: number | null }

export function QuestsView() {
  const [showDone, setShowDone] = useState(false);
  const t = useT();
  const json = useGameStore((s) => {
    const g = s.game!;
    const active: Row[] = activeQuests(g).map((q) => {
      const p = g.quests.progress?.[q.id];
      return { id: q.id, progress: p === undefined ? null : Math.round(p * 100) / 100 };
    });
    const claimable = claimableQuests(g).map((q) => q.id);
    return JSON.stringify({ active, claimable, claimed: g.quests.claimed, villageName: g.villages[g.activeVillageId] ? villageLabelOf(g, g.villages[g.activeVillageId]!) : '' });
  });
  const { active, claimable, claimed, villageName } = JSON.parse(json) as { active: Row[]; claimable: string[]; claimed: string[]; villageName: string };
  const claim = useGameStore((s) => s.claimQuestReward);
  const total = QUESTS.length;
  const done = claimed.length + claimable.length;

  return (
    <div className="space-y-4 text-[12px]">
      <div className="flex items-center gap-3 flex-wrap">
        <h2 className="text-lg">{t('quests.title')} <span className="text-muted font-body text-[12px]">({done}/{total})</span></h2>
        <div className="w-48"><ProgressBar ratio={done / total} /></div>
        <span className="text-muted">{t('quests.rewardsTo')} <strong>{villageName}</strong></span>
      </div>

      {claimable.length > 0 && (
        <section>
          <h3 className="text-[15px] mb-1 text-ok">{t('quests.claimable')}</h3>
          <div className="grid md:grid-cols-2 gap-2">
            {claimable.map((id) => {
              const q = QUEST_BY_ID[id]!;
              return (
                <div key={id} className="panel p-2 border-ok flex items-start gap-2">
                  <div className="flex-1">
                    <div className="font-semibold">✔ {questTitle(q.id)}</div>
                    <div className="text-muted">{questDescription(q.id)}</div>
                    <div className="mt-1">{t('quests.reward')} <RewardLabel reward={q.reward} /></div>
                  </div>
                  <button className="btn" onClick={() => claim(id)}>{t('quests.claim')}</button>
                </div>
              );
            })}
          </div>
        </section>
      )}

      <section>
        <h3 className="text-[15px] mb-1">{t('quests.active')}</h3>
        {active.length === 0 ? (
          <p className="text-muted italic">{t('quests.allDone')}</p>
        ) : (
          <div className="grid md:grid-cols-2 gap-2">
            {active.map((row) => {
              const q = QUEST_BY_ID[row.id]!;
              return (
                <div key={row.id} className="panel p-2">
                  <div className="font-semibold"><PixelIcon name="nav-quests" size={16} /> {questTitle(q.id)}</div>
                  <div className="text-muted">{questDescription(q.id)}</div>
                  {row.progress !== null && <div className="mt-1 w-40"><ProgressBar ratio={row.progress} label={`${Math.round(row.progress * 100)}%`} /></div>}
                  <div className="mt-1">{t('quests.reward')} <RewardLabel reward={q.reward} /></div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section>
        <button className="btn" onClick={() => setShowDone((v) => !v)}>{t(showDone ? 'quests.hideDone' : 'quests.showDone', { n: claimed.length })}</button>
        {showDone && (
          <ul className="mt-2 columns-2 text-muted">
            {QUESTS.filter((q) => claimed.includes(q.id)).map((q) => <li key={q.id}>✔ {questTitle(q.id)}</li>)}
          </ul>
        )}
      </section>
    </div>
  );
}
