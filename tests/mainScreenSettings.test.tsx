// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { BUILDING_IDS, type GameState } from "../src/engine/types";
import { setLocale, t } from "../src/i18n/i18n";
import { useGameStore } from "../src/state/gameStore";
import { useUiStore } from "../src/state/uiStore";
import { BuildingScreen } from "../src/ui/views/BuildingScreen";
import { SettingsView } from "../src/ui/views/SettingsView";
import { VillageView } from "../src/ui/views/VillageView";
import { activeVillage, newTestGame } from "./helpers";

let g: GameState;

const hasPlan = () =>
  (document.body.textContent ?? "").includes(t("plan.title"));
const hasLocked = () => screen.queryByText(/Not yet available/) !== null;
const prefs = () => useUiStore.getState().prefs;

beforeEach(() => {
  setLocale("en");
  g = newTestGame();
  const v = activeVillage(g);
  for (const id of BUILDING_IDS) v.buildings[id] = 0;
  v.buildings.main = 1;
  v.buildQueue = [];
  useGameStore.setState({ game: g, phase: "ready" });
  useUiStore.setState({ buildingScreen: null });
  useUiStore
    .getState()
    .setPrefs({ showBuildPlan: true, showLockedBuildings: true });
});
afterEach(() => cleanup());

describe("what the Village tab shows", () => {
  it("shows the build plan and locked buildings to new players", () => {
    expect(useUiStore.getInitialState().prefs.showBuildPlan).toBe(true);
    expect(useUiStore.getInitialState().prefs.showLockedBuildings).toBe(true);
    render(<VillageView />);
    expect(hasPlan()).toBe(true);
    expect(hasLocked()).toBe(true);
  });

  it("hides each section on its own", () => {
    useUiStore.getState().setPrefs({ showBuildPlan: false });
    const { unmount } = render(<VillageView />);
    expect(hasPlan()).toBe(false);
    expect(hasLocked()).toBe(true);
    unmount();

    useUiStore
      .getState()
      .setPrefs({ showBuildPlan: true, showLockedBuildings: false });
    render(<VillageView />);
    expect(hasPlan()).toBe(true);
    expect(hasLocked()).toBe(false);
  });

  it("says a hidden build plan is still running and links to the Town Hall", () => {
    activeVillage(g).buildPlan = ["wood", "clay"];
    useGameStore.setState({ game: structuredClone(g) });
    useUiStore.getState().setPrefs({ showBuildPlan: false });
    render(<VillageView />);
    const note = screen.getByText(/Build plan active \(2\)/);
    fireEvent.click(within(note).getByRole("button", { name: "Town Hall" }));
    expect(useUiStore.getState().buildingScreen?.building).toBe("main");
  });

  it("has no plan line when nothing is planned or the plan panel is shown", () => {
    useUiStore.getState().setPrefs({ showBuildPlan: false });
    const { unmount } = render(<VillageView />);
    expect(screen.queryByText(/Build plan active/)).toBeNull();
    unmount();

    activeVillage(g).buildPlan = ["wood"];
    useGameStore.setState({ game: structuredClone(g) });
    useUiStore.getState().setPrefs({ showBuildPlan: true });
    render(<VillageView />);
    expect(screen.queryByText(/Build plan active/)).toBeNull();
  });

  it("keeps both in the Town Hall when they are hidden from the Village tab", () => {
    useUiStore
      .getState()
      .setPrefs({ showBuildPlan: false, showLockedBuildings: false });
    render(<BuildingScreen building="main" onBack={() => {}} />);
    expect(hasPlan()).toBe(true);
    expect(hasLocked()).toBe(true);
  });
});

describe("interface settings", () => {
  it("turns each section off with its own checkbox", () => {
    render(<SettingsView />);
    fireEvent.click(
      screen.getByRole("button", { name: t("settings.interface") }),
    );

    fireEvent.click(
      screen.getByRole("checkbox", { name: /Build plan on the Village tab/ }),
    );
    expect(prefs().showBuildPlan).toBe(false);
    expect(prefs().showLockedBuildings).toBe(true);

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Locked buildings on the Village tab/,
      }),
    );
    expect(prefs().showLockedBuildings).toBe(false);
  });
});
