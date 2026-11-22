import { create } from "zustand";
import type { BuildingId, UnitId } from "../engine/types";
import type { EntryRef } from "../shared/encyclopedia/EntryLink";

export type ViewId =
  | "village"
  | "map"
  | "rally"
  | "farm"
  | "reports"
  | "overview"
  | "news"
  | "quests"
  | "ranking"
  | "settings";
export type AnimationMode = "full" | "limited" | "off";

export interface RallyPrefill {
  x: number;
  y: number;
  units?: Partial<Record<UnitId, number>>;
  mode?: "attack" | "support";
}

export interface UiPrefs {
  animations: AnimationMode;
  sound: boolean;
  /** Village illustration shown above the buildings table. */
  showScene: boolean;
  /** The list of buildings that are not yet available is unfolded. */
  lockedOpen: boolean;
  /**
   * The Movements card is folded into its heading. Remembered because the card unmounts whenever the last movement
   * lands — without this, folding it would last only until the next march ends.
   */
  movementsCollapsed: boolean;
  /**
   * The Movements card leaves out the auto-farm's marches. Remembered for the same reason as the fold: the card
   * unmounts whenever the last movement lands.
   */
  movementsHideAutoFarm: boolean;
  /** Villages, armies and command dots on the map; the route lines and their arrows stay either way. */
  mapIcons: boolean;
  /** Other players' armies passing within sight of the player's villages (attacks on the player always show). */
  mapNearbyAttacks: boolean;
  /** The player's own attacks and the armies coming home from them. */
  mapOwnAttacks: boolean;
  /** Support on its way: the player's, other players' to the player and other players' passing by. */
  mapSupport: boolean;
  /** Merchants on the road. */
  mapMerchants: boolean;
  /** The map's toolbar over the map is unfolded. On a phone it can take a third of the screen, so it can be put away. */
  mapToolbarOpen: boolean;
  /** The build planner is shown on the Village tab; the Town Hall always has it. */
  showBuildPlan: boolean;
  /** Buildings that are not yet available are listed on the Village tab; the Town Hall always lists them. */
  showLockedBuildings: boolean;
  /**
   * Battle reports: show only the unit types the battle used, instead of all twelve. Remembered like the other
   * preferences, so it survives a reload; a phone starts with it on, where twelve columns do not fit.
   */
  reportsCompactUnits: boolean;
}

/**
 * A narrow window is a phone, where a report's twelve unit columns do not fit: the reader starts with only the
 * units the battle used, so nothing has to be scrolled sideways.
 */
function narrowScreen(): boolean {
  return (
    typeof matchMedia === "function" && matchMedia("(max-width: 600px)").matches
  );
}

const PREFS_KEY = "lh-ui-prefs";

function defaultPrefs(): UiPrefs {
  const reduced =
    typeof matchMedia === "function" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches;
  return {
    animations: reduced ? "limited" : "full",
    sound: false,
    showScene: true,
    lockedOpen: true,
    movementsCollapsed: false,
    movementsHideAutoFarm: false,
    mapIcons: true,
    mapNearbyAttacks: true,
    mapOwnAttacks: true,
    mapSupport: true,
    mapMerchants: true,
    mapToolbarOpen: true,
    showBuildPlan: true,
    showLockedBuildings: true,
    reportsCompactUnits: narrowScreen(),
  };
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
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(p));
  } catch {
    /* no localStorage */
  }
}

/** The active village is a screen choice, not world state — remembered in the browser, separately per world. */
const activeVillageKey = (worldId: number) => `lh-active-village-${worldId}`;

export function rememberActiveVillage(
  worldId: number,
  villageId: string,
): void {
  try {
    localStorage.setItem(activeVillageKey(worldId), villageId);
  } catch {
    /* no localStorage */
  }
}

export function recallActiveVillage(worldId: number): string | null {
  try {
    return localStorage.getItem(activeVillageKey(worldId));
  } catch {
    return null;
  }
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
  /** Reports view: leave out the auto-farm's reports; unlike the village filter it holds while the game is open. */
  reportsHideAutoFarm: boolean;
  setReportsHideAutoFarm: (hide: boolean) => void;
  /**
   * Battle reports: show only the unit types the battle used, instead of all twelve. Set from the report itself and
   * remembered, so a phone can read a report without scrolling sideways.
   */
  reportsCompactUnits: boolean;
  setReportsCompactUnits: (compact: boolean) => void;
  /** Report to open on entering the Reports view (one-off). */
  reportToOpen: string | null;
  openReport: (reportId: string) => void;
  consumeReportToOpen: () => string | null;
  /** Building to highlight in the table (illustration hotspot click). */
  focusBuilding: string | null;
  setFocusBuilding: (b: string | null) => void;
  prefs: UiPrefs;
  setPrefs: (patch: Partial<UiPrefs>) => void;
  /** Details windows, newest last; empty means closed. */
  info: EntryRef[];
  openInfo: (ref: EntryRef) => void;
  infoBack: () => void;
  closeInfo: () => void;
  /** The report-a-problem dialog is open. */
  reportOpen: boolean;
  setReportOpen: (open: boolean) => void;
}

export const useUiStore = create<UiStore>()((set, get) => ({
  view: "village",
  setView: (view) => set({ view }),
  info: [],
  openInfo: (ref) =>
    set((s) => {
      const top = s.info[s.info.length - 1];
      return top && top.kind === ref.kind && top.id === ref.id
        ? s
        : { info: [...s.info, ref] };
    }),
  infoBack: () => set((s) => ({ info: s.info.slice(0, -1) })),
  closeInfo: () => set({ info: [] }),
  reportOpen: false,
  setReportOpen: (reportOpen) => set({ reportOpen }),
  navigate: (view) => set({ view, buildingScreen: null }),
  buildingScreen: null,
  openBuilding: (building, villageId) =>
    set({ view: "village", buildingScreen: { building, villageId } }),
  closeBuilding: () => set({ buildingScreen: null }),
  rallyPrefill: null,
  openRally: (rallyPrefill) => set({ rallyPrefill, view: "rally" }),
  consumeRallyPrefill: () => {
    const p = get().rallyPrefill;
    if (p) set({ rallyPrefill: null });
    return p;
  },
  mapCenter: null,
  goToMap: (x, y) => set({ mapCenter: { x, y }, view: "map" }),
  profilePlayerId: null,
  openProfile: (profilePlayerId) => set({ profilePlayerId }),
  closeProfile: () => set({ profilePlayerId: null }),
  reportsVillageFilter: null,
  setReportsVillageFilter: (reportsVillageFilter) =>
    set({ reportsVillageFilter }),
  openReportsFor: (reportsVillageFilter) =>
    set({ reportsVillageFilter, view: "reports" }),
  reportsHideAutoFarm: false,
  setReportsHideAutoFarm: (reportsHideAutoFarm) => set({ reportsHideAutoFarm }),
  reportsCompactUnits: narrowScreen(),
  setReportsCompactUnits: (reportsCompactUnits) => set({ reportsCompactUnits }),
  reportToOpen: null,
  openReport: (reportToOpen) => set({ reportToOpen, view: "reports" }),
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
