// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { tileAt } from "../src/ui/map/tiles";
import type { GameState } from "../src/engine/types";
import { useGameStore } from "../src/state/gameStore";
import { useUiStore } from "../src/state/uiStore";
import {
  MapHoverCard,
  OPEN_DELAY_MS,
  type MapHover,
} from "../src/ui/map/MapHoverCard";
import { VillagePopup } from "../src/ui/views/MapView";
import { blockersText } from "../src/i18n/names";
import { labelIncoming } from "../src/engine/incoming";
import { checkSendClient } from "../src/ui/views/rallyCheck";
import {
  activeVillage,
  barbarianVillage,
  makeBotVillage,
  newTestGame,
} from "./helpers";

function world(): {
  g: GameState;
  bot: { x: number; y: number };
  barb: { x: number; y: number };
} {
  const g = newTestGame();
  const home = activeVillage(g);
  const bot = makeBotVillage(g, home.x + 3, home.y + 1);
  const barb = barbarianVillage(g);
  return { g, bot: { x: bot.x, y: bot.y }, barb: { x: barb.x, y: barb.y } };
}

let setHover: (h: MapHover | null) => void = () => {};
let w: ReturnType<typeof world>;
let restoreActions: (() => void) | null = null;

beforeEach(() => {
  w = world();
  useGameStore.setState({
    game: w.g,
    phase: "ready",
    intel: {},
    notice: null,
    lastError: null,
  });
  useUiStore.setState({ view: "map", rallyPrefill: null });
  vi.useFakeTimers();
  render(
    <MapHoverCard
      register={(set) => {
        setHover = set;
      }}
      bounds={{ width: 800, height: 600 }}
    />,
  );
});

afterEach(() => {
  cleanup();
  restoreActions?.();
  restoreActions = null;
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const pointAt = (p: { x: number; y: number }) =>
  act(() => setHover({ ...p, left: 200, top: 150, tw: 53, th: 38 }));
const wait = (ms: number) =>
  act(() => {
    vi.advanceTimersByTime(ms);
  });
/** The cursor rests on the village until its card opens. */
const hoverOn = (p: { x: number; y: number }) => {
  pointAt(p);
  wait(OPEN_DELAY_MS);
};

function emptyTile(): { x: number; y: number } {
  for (let dx = 1; dx < 20; dx++) {
    const t = tileAt(w.g, w.bot.x + dx, w.bot.y);
    if (t.kind === "empty") return { x: t.x, y: t.y };
  }
  throw new Error("no empty tile");
}

describe("the card shown on hovering a village", () => {
  it("another player's village: Attack, Support, Scout; Support opens the rally point set to support", () => {
    hoverOn(w.bot);
    for (const name of ["Atakuj", "Wesprzyj", "Zwiaduj"])
      expect(screen.getByRole("button", { name })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Wesprzyj" }));
    expect(useUiStore.getState().rallyPrefill).toEqual({
      x: w.bot.x,
      y: w.bot.y,
      mode: "support",
    });
    expect(useUiStore.getState().view).toBe("rally");
  });

  it("a barbarian village: Attack and Scout, no Support", () => {
    hoverOn(w.barb);
    expect(screen.getByRole("button", { name: "Atakuj" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Zwiaduj" }));
    expect(useUiStore.getState().rallyPrefill).toEqual({
      x: w.barb.x,
      y: w.barb.y,
      mode: "attack",
      units: { scout: 1 },
    });
    hoverOn(w.barb);
    expect(screen.queryByRole("button", { name: "Wesprzyj" })).toBeNull();
  });

  it("a barbarian village offers template A, disabled until template A is defined", () => {
    hoverOn(w.barb);
    expect(
      (screen.getByRole("button", { name: "Szablon A" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("hides at once when its village is clicked, so it does not cover the village popup", () => {
    cleanup();
    const ui = render(
      <MapHoverCard
        register={(set) => {
          setHover = set;
        }}
        bounds={{ width: 800, height: 600 }}
        selected={null}
      />,
    );
    hoverOn(w.barb);
    expect(screen.queryByRole("tooltip")).not.toBeNull();
    ui.rerender(
      <MapHoverCard
        register={(set) => {
          setHover = set;
        }}
        bounds={{ width: 800, height: 600 }}
        selected={w.barb}
      />,
    );
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("a click before the card opens keeps it closed, so it does not cover the village popup", () => {
    cleanup();
    const ui = render(
      <MapHoverCard
        register={(set) => {
          setHover = set;
        }}
        bounds={{ width: 800, height: 600 }}
        selected={null}
      />,
    );
    pointAt(w.barb);
    wait(100);
    ui.rerender(
      <MapHoverCard
        register={(set) => {
          setHover = set;
        }}
        bounds={{ width: 800, height: 600 }}
        selected={w.barb}
      />,
    );
    wait(OPEN_DELAY_MS);
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("stays open when moving from the tile onto the card and hides 200 ms after leaving both", () => {
    hoverOn(w.bot);
    const card = screen.getByRole("tooltip");
    act(() => setHover(null));
    act(() => {
      vi.advanceTimersByTime(100);
    });
    fireEvent.pointerEnter(card);
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(screen.queryByRole("tooltip")).not.toBeNull();
    fireEvent.pointerLeave(card);
    act(() => {
      vi.advanceTimersByTime(199);
    });
    expect(screen.queryByRole("tooltip")).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("stays on the village when the cursor crosses an empty tile on its way to the card", () => {
    hoverOn(w.bot);
    act(() =>
      setHover({ ...emptyTile(), left: 253, top: 150, tw: 53, th: 38 }),
    );
    act(() => {
      vi.advanceTimersByTime(150);
    });
    fireEvent.pointerEnter(screen.getByRole("tooltip"));
    act(() => {
      vi.advanceTimersByTime(500);
    });
    fireEvent.click(screen.getByRole("button", { name: "Atakuj" }));
    expect(useUiStore.getState().rallyPrefill).toEqual({
      x: w.bot.x,
      y: w.bot.y,
      mode: "attack",
    });
  });

  it("stays on the village when the cursor crosses another village on its way to the card", () => {
    hoverOn(w.bot);
    act(() => setHover({ ...w.barb, left: 253, top: 188, tw: 53, th: 38 }));
    act(() => {
      vi.advanceTimersByTime(150);
    });
    fireEvent.pointerEnter(screen.getByRole("tooltip"));
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(screen.getByRole("tooltip").textContent).toContain(
      `(${w.bot.x}|${w.bot.y})`,
    );
  });

  it("switches to another village 200 ms after moving onto it if the cursor did not enter the card", () => {
    hoverOn(w.bot);
    act(() => setHover({ ...w.barb, left: 253, top: 188, tw: 53, th: 38 }));
    act(() => {
      vi.advanceTimersByTime(199);
    });
    expect(screen.getByRole("tooltip").textContent).toContain(
      `(${w.bot.x}|${w.bot.y})`,
    );
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.getByRole("tooltip").textContent).toContain(
      `(${w.barb.x}|${w.barb.y})`,
    );
  });

  it("opens only after the cursor rests on a village for a moment", () => {
    act(() =>
      setHover({ ...emptyTile(), left: 253, top: 150, tw: 53, th: 38 }),
    );
    pointAt(w.bot);
    wait(OPEN_DELAY_MS - 1);
    expect(screen.queryByRole("tooltip")).toBeNull();
    pointAt(w.bot);
    wait(1);
    expect(screen.getByRole("tooltip").textContent).toContain(
      `(${w.bot.x}|${w.bot.y})`,
    );
  });

  it("villages the cursor only passes over do not open a card", () => {
    pointAt(w.bot);
    wait(OPEN_DELAY_MS - 50);
    act(() => setHover({ ...w.barb, left: 253, top: 188, tw: 53, th: 38 }));
    wait(OPEN_DELAY_MS - 1);
    expect(screen.queryByRole("tooltip")).toBeNull();
    wait(1);
    expect(screen.getByRole("tooltip").textContent).toContain(
      `(${w.barb.x}|${w.barb.y})`,
    );
  });

  it("leaving the village before the card opens keeps it closed", () => {
    pointAt(w.bot);
    wait(OPEN_DELAY_MS - 50);
    act(() => setHover(null));
    wait(OPEN_DELAY_MS);
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("cursor moves over the card do not reach the map underneath", () => {
    cleanup();
    const mapMove = vi.fn();
    render(
      <div onPointerMove={mapMove}>
        <MapHoverCard
          register={(set) => {
            setHover = set;
          }}
          bounds={{ width: 800, height: 600 }}
        />
      </div>,
    );
    hoverOn(w.bot);
    fireEvent.pointerMove(screen.getByRole("button", { name: "Atakuj" }));
    expect(mapMove).not.toHaveBeenCalled();
  });

  it("own village: Village, and Support only when it is not the active one", () => {
    const home = activeVillage(w.g);
    hoverOn(home);
    expect(screen.getByRole("button", { name: "Wioska" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Wesprzyj" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Atakuj" })).toBeNull();
  });
});

describe("rally point: support", () => {
  it("supporting another player is allowed, barbarians are not, with a Polish description", () => {
    const home = activeVillage(w.g);
    home.units.spear = 10;
    const toBot = checkSendClient(
      w.g,
      home.id,
      w.bot.x,
      w.bot.y,
      { spear: 5 },
      "support",
    );
    expect(toBot.blockers).toEqual([]);
    const toBarb = checkSendClient(
      w.g,
      home.id,
      w.barb.x,
      w.barb.y,
      { spear: 5 },
      "support",
    );
    expect(toBarb.blockers).toEqual([{ kind: "supportTargetBarbarian" }]);
    expect(blockersText(toBarb.blockers)).toBe(
      "nie można wspierać wioski barbarzyńskiej",
    );
  });
});

describe("incoming attacks", () => {
  it("the travel-time label works without the origin village in the client state", () => {
    const home = activeVillage(w.g);
    const from = { x: home.x + 10, y: home.y };
    const minutes = (35 * 10) / w.g.config.unitSpeed;
    const c = {
      id: "c_x",
      type: "attack" as const,
      originId: `v:${from.x}:${from.y}`,
      targetId: home.id,
      units: {},
      departedAt: w.g.now,
      arrivesAt: w.g.now + minutes * 60_000,
    };
    expect(w.g.villages[c.originId]).toBeUndefined();
    expect(labelIncoming(w.g, c)).toBe("noble");
  });
});

describe("the village menu on tap", () => {
  it("shows Support on another player's village", () => {
    cleanup();
    render(
      <VillagePopup
        sel={w.bot}
        anchor={{ left: 0, top: 0, tw: 53, th: 38 }}
        bounds={{ width: 360, height: 640 }}
        sheet
        onClose={() => {}}
        onDetails={() => {}}
      />,
    );
    expect(screen.queryByText("Szybka budowa")).toBeNull();
    expect(screen.queryByText("Szybka rekrutacja")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Wesprzyj" }));
    expect(useUiStore.getState().rallyPrefill).toEqual({
      x: w.bot.x,
      y: w.bot.y,
      mode: "support",
    });
  });

  it("queues a building upgrade and recruits units from the own-village popup", () => {
    cleanup();
    const home = activeVillage(w.g);
    home.buildings.main = 1;
    home.buildings.wood = 1;
    home.buildings.barracks = 1;
    home.buildings.farm = 20;
    home.resources = { wood: 100_000, clay: 100_000, iron: 100_000 };
    const actions = useGameStore.getState();
    const build = vi.fn(actions.build).mockResolvedValue(null);
    const recruit = vi.fn(actions.recruit).mockResolvedValue(null);
    useGameStore.setState({ build, recruit });
    restoreActions = () =>
      useGameStore.setState({ build: actions.build, recruit: actions.recruit });

    render(
      <VillagePopup
        sel={{ x: home.x, y: home.y }}
        anchor={{ left: 0, top: 0, tw: 53, th: 38 }}
        bounds={{ width: 360, height: 640 }}
        sheet
        onClose={() => {}}
        onDetails={() => {}}
      />,
    );

    const villageButtons = screen.getByRole("button", { name: "Wioska" });
    const quickActions = screen
      .getByText("Szybka budowa")
      .closest("details")?.parentElement;
    expect(quickActions?.previousElementSibling).toBe(
      villageButtons.parentElement,
    );

    fireEvent.click(screen.getByText("Szybka budowa"));
    fireEvent.change(screen.getByLabelText("Budynek"), {
      target: { value: "wood" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Rozbuduj do 2" }));
    expect(build).toHaveBeenCalledWith(home.id, "wood");

    fireEvent.click(screen.getByText("Szybka rekrutacja"));
    fireEvent.change(screen.getByLabelText("Jednostka"), {
      target: { value: "spear" },
    });
    fireEvent.change(screen.getByLabelText("Liczba"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Rekrutuj" }));
    expect(recruit).toHaveBeenCalledWith(home.id, "spear", 2);
  });
});
