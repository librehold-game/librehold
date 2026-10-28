import { buildingName } from '../../i18n/names';
import { useT } from '../../i18n/i18n';
import { BUILDINGS } from '../../data/buildings.data';
import { humanPlayer } from '../../engine/players';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { selectNow } from '../../state/selectors';
import { Countdown } from '../components/Countdown';
import { ProgressBar } from '../components/ProgressBar';

function QueueRow({ jobId }: { jobId: string }) {
  const job = useGameStore((s) => selectActiveVillage(s).buildQueue.find((j) => j.id === jobId));
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const ratio = useGameStore((s) => {
    const j = selectActiveVillage(s).buildQueue.find((x) => x.id === jobId);
    if (!j) return 0;
    const now = selectNow(s);
    if (now < j.startedAt) return 0;
    return Math.min(1, (now - j.startedAt) / Math.max(1, j.completesAt - j.startedAt));
  });
  const cancel = useGameStore((s) => s.cancelBuildJob);
  const t = useT();
  if (!job) return null;
  const started = ratio > 0;
  return (
    <tr>
      <td className="font-semibold">
        {job.kind === 'demolish' ? t('queue.demolish') : ''}{buildingName(job.building)}
        <span className="text-muted font-normal">{t('queue.toLevel', { n: job.targetLevel })}</span>
      </td>
      <td className="w-48 max-w-full" data-label={t('queue.progress')}><ProgressBar ratio={ratio} label={`${Math.floor(ratio * 100)}%`} /></td>
      <td className="tabular-nums" data-label={t('queue.left')}><Countdown until={job.completesAt} /></td>
      <td className="stack-inline">
        <button
          className="btn"
          onClick={() => cancel(villageId, job.id)}
          title={job.kind === 'build' ? (started ? t('queue.refund90') : t('queue.refund100')) : t('queue.stopDemolish')}
        >
          {t('queue.cancel')}
        </button>
      </td>
    </tr>
  );
}

export function BuildQueuePanel() {
  const jobIds = useGameStore((s) => selectActiveVillage(s).buildQueue.map((j) => j.id).join(','));
  const slots = useGameStore((s) => (s.game ? humanPlayer(s.game).buildQueueSlots : 2));
  const ids = jobIds ? jobIds.split(',') : [];
  const t = useT();
  return (
    <div className="mt-1">
      <h3 className="text-[15px] mb-1">
        {t('queue.title')} <span className="text-muted font-body text-[12px]">{t('queue.slots', { used: ids.length, slots })}</span>
      </h3>
      {ids.length === 0 ? (
        <p className="text-muted italic text-[12px]">{t('queue.empty')}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="table-game table-stack">
            <thead>
              <tr><th>{t('queue.colBuild')}</th><th>{t('queue.progress')}</th><th>{t('queue.left')}</th><th>{t('queue.colAction')}</th></tr>
            </thead>
            <tbody>
              {ids.map((id) => <QueueRow key={id} jobId={id} />)}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
