import { useEffect, useMemo, useState } from 'react';
import { HostPanel } from '../../account/host/HostPanel';
import { useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { formatDuration } from '../format';
import { worldNowOf } from './geometry';
import { ArmyCardView, ArmyList, RankingTable, VillageCardView, lookupsOf } from './WatchCards';
import { WatchMap } from './WatchMap';
import { useWatchStore } from './watchStore';
import { WatchTileMap } from './WatchTileMap';

const TICK_MS = 1000;

/** The wall clock, once a second, for the game clock and the army list. */
function useWallClock(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);
  return now;
}

/**
 * The host's god view of their player world (`/game/?world={id}&watch=1`): the whole map or the normal one, every
 * army, the live ranking and the host panel. Nothing here sends an order: the host is no player of the world.
 */
export function GodView() {
  const t = useT();
  const worldId = Number(new URLSearchParams(window.location.search).get('world'));
  const { state, receivedAt, failed, selected, village, boot, refresh, select, stop } = useWatchStore();
  const [normal, setNormal] = useState(false);
  const wall = useWallClock();
  useEffect(() => {
    boot(worldId);
    return stop;
  }, [worldId, boot, stop]);
  const lookups = useMemo(() => (state ? lookupsOf(state) : null), [state]);

  if (!state || !lookups) {
    return <p className="panel m-3 p-3" role="status">{t(failed ? 'watch.failed' : 'watch.loading')}</p>;
  }
  const now = worldNowOf(state, receivedAt, wall);
  const waiting = state.hosted?.status === 'waiting';
  const army = selected?.kind === 'army' ? state.commands.find((c) => c.id === selected.id) : undefined;
  return (
    <div className="max-w-[1280px] mx-auto px-1.5 sm:px-3 py-2 sm:py-3 flex flex-col gap-2">
      <header className="panel px-3 py-2 flex flex-wrap items-center gap-x-3 gap-y-1">
        <h1 className="font-heading text-lg">{worldDisplayName({ name: state.worldName })}</h1>
        <span>{t('watch.clock')} <strong role="timer" className="tabular-nums">{formatDuration(now)}</strong></span>
        {waiting ? <strong className="text-bad">{t('watch.waiting')}</strong> : state.paused && <strong className="text-bad">{t('watch.paused')}</strong>}
        <span className="ml-auto flex gap-1">
          <button className="btn" aria-pressed={!normal} onClick={() => setNormal(false)}>{t('watch.showWhole')}</button>
          <button className="btn" aria-pressed={normal} onClick={() => setNormal(true)}>{t('watch.showNormal')}</button>
        </span>
      </header>
      <div className="flex flex-col lg:flex-row gap-2 items-start">
        <main className="panel p-2 flex-1 min-w-0">
          {normal
            ? <WatchTileMap state={state} receivedAt={receivedAt} selected={selected} onPick={select} />
            : <WatchMap state={state} receivedAt={receivedAt} onPick={select} />}
        </main>
        <aside className="flex flex-col gap-2 w-full lg:w-[340px] shrink-0">
          {army && <ArmyCardView command={army} lookups={lookups} onClose={() => select(null)} />}
          {selected?.kind === 'village' && <VillageCardView village={village} onClose={() => select(null)} />}
          {state.hosted && <div className="panel p-2"><HostPanel card={state.hosted} onChange={() => void refresh()} watching /></div>}
          <section className="panel p-2"><RankingTable rows={state.ranking} /></section>
          <section className="panel p-2">
            <h2 className="font-heading text-[14px] mb-1">{t('watch.armies')}</h2>
            <ArmyList commands={state.commands} lookups={lookups} onOpen={(id) => select({ kind: 'army', id })} />
          </section>
        </aside>
      </div>
    </div>
  );
}
