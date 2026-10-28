import { useState } from 'react';
import { useT } from '../../i18n/i18n';
import { useGameStore } from '../../state/gameStore';
import { Overlay } from './Overlay';

const STEPS = ['army', 'barbarian', 'noble', 'yours'] as const;

/** Four short steps to a first conquest, once per account; skipping counts as done. */
export function Tutorial() {
  const t = useT();
  const show = useGameStore((s) => !!s.instant && !s.instant.ended && !!s.account && !s.account.tutorialDone);
  const finish = useGameStore((s) => s.finishTutorial);
  const [step, setStep] = useState(0);
  if (!show) return null;
  const last = step === STEPS.length - 1;
  const id = STEPS[step]!;
  return (
    <Overlay label={t('tutorial.label')}>
      <h2 className="panel-header">{t(`tutorial.${id}.title`)}</h2>
      <div className="p-3 flex flex-col gap-3">
        <p>{t(`tutorial.${id}.text`)}</p>
        <div className="flex items-center gap-2">
          <span className="text-muted">{t('tutorial.step', { n: step + 1, total: STEPS.length })}</span>
          <button className="btn ml-auto" onClick={finish}>{t('tutorial.skip')}</button>
          <button className="btn" onClick={() => (last ? finish() : setStep(step + 1))}>{t(last ? 'tutorial.done' : 'tutorial.next')}</button>
        </div>
      </div>
    </Overlay>
  );
}
