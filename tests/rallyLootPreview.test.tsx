// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { UNITS } from "../src/data/units.data";
import { UNIT_IDS, type GameState, type Village } from "../src/engine/types";
import { setLocale } from "../src/i18n/i18n";
import { useGameStore } from "../src/state/gameStore";
import { RallyPointView } from "../src/ui/views/RallyPointView";
import { activeVillage, makeBotVillage, newTestGame } from "./helpers";

let g: GameState;
let home: Village;

function openWithTarget() {
  const bot = makeBotVillage(g);
  useGameStore.setState({ game: g, phase: "ready" });
  render(<RallyPointView />);
  fireEvent.change(screen.getByPlaceholderText("x"), {
    target: { value: String(bot.x) },
  });
  fireEvent.change(screen.getByPlaceholderText("y"), {
    target: { value: String(bot.y) },
  });
}

const page = () => document.body.textContent ?? "";

beforeEach(() => {
  setLocale("en");
  g = newTestGame();
  home = activeVillage(g);
  for (const u of UNIT_IDS) home.units[u] = 0;
});
afterEach(() => cleanup());

describe("the loot an attack could carry", () => {
  it("adds up what every chosen unit carries and follows the choice", () => {
    home.units.spear = 10;
    home.units.axe = 5;
    openWithTarget();
    expect(page()).not.toContain("Loot: up to");

    fireEvent.click(screen.getByRole("button", { name: "(10)" }));
    expect(page()).toContain(`Loot: up to ${10 * UNITS.spear.haul} resources`);

    fireEvent.click(screen.getByRole("button", { name: "(5)" }));
    expect(page()).toContain(
      `Loot: up to ${10 * UNITS.spear.haul + 5 * UNITS.axe.haul} resources`,
    );

    fireEvent.click(screen.getByRole("button", { name: "None" }));
    expect(page()).not.toContain("Loot: up to");
  });

  it("stays hidden for troops that carry nothing", () => {
    home.units.scout = 3;
    openWithTarget();

    fireEvent.click(screen.getByRole("button", { name: "(3)" }));

    expect(page()).toContain("A scouting mission");
    expect(page()).not.toContain("Loot: up to");
  });
});
