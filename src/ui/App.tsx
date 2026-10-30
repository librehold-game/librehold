import { lazy, Suspense, useEffect } from 'react';
import { useGameStore } from '../state/gameStore';
import { useUiStore } from '../state/uiStore';
import { Header } from './layout/Header';
import { Nav, TABS } from './layout/Nav';
import { VillageView } from './views/VillageView';
import { MapView } from './views/MapView';
import { RallyPointView } from './views/RallyPointView';
import { ReportsView } from './views/ReportsView';
import { FarmAssistantView } from './views/FarmAssistantView';
import { OverviewView } from './views/OverviewView';
import { NewsView } from './views/NewsView';
import { QuestsView } from './views/QuestsView';
import { SettingsView } from './views/SettingsView';
import { unreadReports } from '../engine/reports';
import { claimableQuests } from '../engine/quests';
import { ErrorToast } from './components/ErrorToast';
import { useGameSounds } from './useGameSounds';
import { PlayerProfile } from './components/PlayerProfile';
import { RankingView } from './views/RankingView';
import { humanVillageIds } from '../engine/players';
import { t, useT } from '../i18n/i18n';
import { ClaimDialog } from './instant/ClaimDialog';
import { ReportDialog } from './components/ReportDialog';
import { Results } from './instant/Results';
import { HostPause, Waiting } from './instant/Waiting';
import { Tutorial } from './instant/Tutorial';

const InfoWindow = lazy(() => import('./info/InfoWindow').then((m) => ({ default: m.InfoWindow })));

function InfoMount() {
  const open = useUiStore((s) => s.info.length > 0);
  return open ? <Suspense fallback={null}><InfoWindow /></Suspense> : null;
}

function Shell() {
  useT();
  const view = useUiStore((s) => s.view);
  const profileId = useUiStore((s) => s.profilePlayerId);
  const closeProfile = useUiStore((s) => s.closeProfile);
  const navigate = useUiStore((s) => s.navigate);
  const unread = useGameStore((s) => (s.game ? unreadReports(s.game) : 0));
  const claimable = useGameStore((s) => (s.game ? claimableQuests(s.game).length : 0));
  const paused = useGameStore((s) => !!s.game?.config.paused);
  const hosted = useGameStore((s) => s.hosted);
  const incoming = useGameStore((s) => (s.game ? s.game.commands.filter((c) => c.type === 'attack' && humanVillageIds(s.game!).includes(c.targetId) && !humanVillageIds(s.game!).includes(c.originId)).length : 0));
  useGameSounds();

  useEffect(() => {
    document.title = (unread > 0 ? `(${unread}) ` : '') + t('app.name');
  }, [unread]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLSelectElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const tab = TABS.find((t) => t.key === e.key && t.enabled);
      if (tab) navigate(tab.id);
      if (e.key === ',') useGameStore.getState().switchVillage(-1);
      if (e.key === '.') useGameStore.getState().switchVillage(1);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [navigate]);

  return (
    <div className={`max-w-[1280px] mx-auto px-1.5 sm:px-3 py-2 sm:py-3 pb-[calc(62px+env(safe-area-inset-bottom,0px))] md:pb-3`}>
      <Header />
      <Nav view={view} onChange={navigate} badges={{ reports: unread, rally: incoming, quests: claimable }} />
      {hosted?.pausedByHost ? <HostPause hosted={hosted} />
        : paused && !hosted?.waiting && <div role="status" className="panel mt-2 px-3 py-2 text-center font-semibold text-bad">{t('app.paused')}</div>}
      <main className="panel md:rounded-t-none mt-2 md:mt-0 p-2 sm:p-3 min-h-[400px]">
        {view === 'village' && <VillageView />}
        {view === 'map' && <MapView />}
        {view === 'rally' && <RallyPointView />}
        {view === 'reports' && <ReportsView />}
        {view === 'farm' && <FarmAssistantView />}
        {view === 'overview' && <OverviewView />}
        {view === 'news' && <NewsView />}
        {view === 'quests' && <QuestsView />}
        {view === 'ranking' && <RankingView />}
        {view === 'settings' && <SettingsView />}
      </main>
      {profileId && <PlayerProfile playerId={profileId} onClose={closeProfile} />}
      <Tutorial />
      <Results />
      <Waiting />
      <ClaimDialog />
      <ReportDialog />
      <InfoMount />
      <ErrorToast />
    </div>
  );
}

export function App() {
  const t = useT();
  const phase = useGameStore((s) => s.phase);
  const boot = useGameStore((s) => s.boot);
  const error = useGameStore((s) => s.lastError);

  useEffect(() => {
    void boot();
  }, [boot]);

  if (phase === 'booting') {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="panel px-8 py-6 text-center">
          <h1 className="text-2xl mb-2">{t('app.name')}</h1>
          <p className="text-muted">{error ?? t('app.loadingWorld')}</p>
        </div>
      </div>
    );
  }

  return <Shell />;
}
