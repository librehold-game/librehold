import { create } from 'zustand';
import type { BuildingId, UnitId } from '../engine/types';

export type ViewId = 'village' | 'map' | 'rally' | 'farm' | 'reports' | 'overview' | 'news' | 'quests' | 'ranking' | 'settings';
export type AnimationMode = 'full' | 'limited' | 'off';

export interface RallyPrefill {
  x: number;
  y: number;
  units?: Partial<Record<UnitId, number>>;
  mode?: 'attack' | 'support';
}

export interface UiPrefs {
  animations: AnimationMode;
  sound: boolean;
  /** Village illustration shown above the buildings table. */
  showScene: boolean;
}

const PREFS_KEY = 'lh-ui-prefs';

function defaultPrefs(): UiPrefs {
  const reduced = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
  return { animations: reduced ? 'limited' : 'full', sound: false, showScene: true };
}

function loadPrefs(): UiPrefs {
  const base = defaultPrefs();
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (!raw) return base;
    const parsed = JSON.parse(raw) as Partial<UiPrefs>;
    return { ...base, ...parsed };
  } catch {
    return base;
  }
}

function persistPrefs(p: UiPrefs): void {
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(p)); } catch { /* no localStorage */ }
}

/** The active village is a screen choice, not world state — remembered in the browser, separately per world. */
const activeVillageKey = (worldId: number) => `lh-active-village-${worldId}`;

export function rememberActiveVillage(worldId: number, villageId: string): void {
  try { localStorage.setItem(activeVillageKey(worldId), villageId); } catch { /* no localStorage */ }
}

export function recallActiveVillage(worldId: number): string | null {
  try { return localStorage.getItem(activeVillageKey(worldId)); } catch { return null; }
}

interface UiStore {
  view: ViewId;
  setView: (v: ViewId) => void;
  /** Menu click or keyboard shortcut: go to the view and back to its main screen (closes the building screen). */
  navigate: (v: ViewId) => void;
  /** The open building screen in the Village view — valid only for the village it was opened in. */
  buildingScreen: { building: BuildingId; villageId: string } | null;
  openBuilding: (building: BuildingId, villageId: string) => void;
  closeBuilding: () => void;
  rallyPrefill: RallyPrefill | null;
  openRally: (prefill: RallyPrefill) => void;
  consumeRallyPrefill: () => RallyPrefill | null;
  mapCenter: { x: number; y: number } | null;
  goToMap: (x: number, y: number) => void;
  /** Player profile (a window over the current view). */
  profilePlayerId: string | null;
  openProfile: (playerId: string) => void;
  closeProfile: () => void;
  /** Reports view filter: only reports about this village (from the map menu). */
  reportsVillageFilter: string | null;
  setReportsVillageFilter: (villageId: string | null) => void;
  openReportsFor: (villageId: string | null) => void;
  /** Report to open on entering the Reports view (one-off). */
  reportToOpen: string | null;
  openReport: (reportId: string) => void;
  consumeReportToOpen: () => string | null;
  /** Building to highlight in the table (illustration hotspot click). */
  focusBuilding: string | null;
  setFocusBuilding: (b: string | null) => void;
  prefs: UiPrefs;
  setPrefs: (patch: Partial<UiPrefs>) => void;
  /** The report-a-problem dialog is open. */
  reportOpen: boolean;
  setReportOpen: (open: boolean) => void;
}

export const useUiStore = create<UiStore>()((set, get) => ({
  view: 'village',
  setView: (view) => set({ view }),
  reportOpen: false,
  setReportOpen: (reportOpen) => set({ reportOpen }),
  navigate: (view) => set({ view, buildingScreen: null }),
  buildingScreen: null,
  openBuilding: (building, villageId) => set({ view: 'village', buildingScreen: { building, villageId } }),
  closeBuilding: () => set({ buildingScreen: null }),
  rallyPrefill: null,
  openRally: (rallyPrefill) => set({ rallyPrefill, view: 'rally' }),
  consumeRallyPrefill: () => {
    const p = get().rallyPrefill;
    if (p) set({ rallyPrefill: null });
    return p;
  },
  mapCenter: null,
  goToMap: (x, y) => set({ mapCenter: { x, y }, view: 'map' }),
  profilePlayerId: null,
  openProfile: (profilePlayerId) => set({ profilePlayerId }),
  closeProfile: () => set({ profilePlayerId: null }),
  reportsVillageFilter: null,
  setReportsVillageFilter: (reportsVillageFilter) => set({ reportsVillageFilter }),
  openReportsFor: (reportsVillageFilter) => set({ reportsVillageFilter, view: 'reports' }),
  reportToOpen: null,
  openReport: (reportToOpen) => set({ reportToOpen, view: 'reports' }),
  consumeReportToOpen: () => {
    const id = get().reportToOpen;
    if (id) set({ reportToOpen: null });
    return id;
  },
  focusBuilding: null,
  setFocusBuilding: (focusBuilding) => set({ focusBuilding }),
  prefs: loadPrefs(),
  setPrefs: (patch) => {
    const prefs = { ...get().prefs, ...patch };
    persistPrefs(prefs);
    set({ prefs });
  },
}));
