// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { setLocale } from "../src/i18n/i18n";
import { useGameStore } from "../src/state/gameStore";
import { BuildPlanPanel } from "../src/ui/views/BuildPlanPanel";
import { newTestGame } from "./helpers";

afterEach(() => {
  cleanup();
  setLocale("pl");
});

describe("build plan panel", () => {
  it("shows the plan controls while build plans are allowed for the world", () => {
    setLocale("en");
    const g = newTestGame();
    useGameStore.setState({ game: g, phase: "ready" });
    render(<BuildPlanPanel />);
    expect(screen.getByText("Build plan")).toBeTruthy();
  });

  it("hides the whole panel when the host has switched build plans off", () => {
    setLocale("en");
    const g = newTestGame();
    g.config.noBuildPlans = true;
    useGameStore.setState({ game: g, phase: "ready" });
    render(<BuildPlanPanel />);
    expect(screen.queryByText("Build plan")).toBeNull();
  });
});
