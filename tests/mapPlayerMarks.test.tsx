// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { setLocale } from "../src/i18n/i18n";
import { MapView } from "../src/ui/views/MapView";
import { useGameStore } from "../src/state/gameStore";
import { cleanMarks, markKey } from "../src/engine/marks";
import { useUiStore } from "../src/state/uiStore";
import {
  MARK_COLORS,
  OWN_VILLAGE_COLOR,
  nextMarkColor,
} from "../src/ui/map/playerMarks";
import {
  HUMAN_ID,
  type GameState,
  type PlayerMark,
  type Village,
} from "../src/engine/types";
import {
  activeVillage,
  addOwnVillage,
  barbarianVillage,
  makeBotVillage,
  newTestGame,
  stateFixture,
} from "./helpers";
import { stubServer } from "./serverStub";

let g: GameState;
let bot: Village;
/** Every body the client sent to `set-marks`, in order. */
let sent: PlayerMark[][];

/**
 * Answers `set-marks` the way the server does: it replaces the marks in the world's state and hands the state back,
 * which is what the map then draws.
 */
function stubMarks() {
  stubServer(g, {
    routes: {
      "POST /api/v1/worlds/1/orders/set-marks": (body) => {
        const { marks } = body as { marks: PlayerMark[] };
        sent.push(marks);
        g.marks = marks;
        return { status: 200, body: stateFixture(g) };
      },
    },
  });
}

/** The map loads its chunks after the first paint, and the minimap draws what is loaded. */
async function showMap() {
  await useGameStore.getState().boot();
  const r = render(<MapView />);
  await act(async () => {
    await new Promise((res) => setTimeout(res, 0));
  });
  return r;
}

const panel = () => screen.getByRole("group", { name: "Marked players" });
// The field offers the world's players as suggestions, so it is a combobox rather than a plain textbox.
const nameField = () => screen.getByRole("combobox", { name: "Player name" });
/** The dialog's submit button; it stays off while the field names no player. */
const markButton = () =>
  screen.getByRole("button", { name: "Mark" }) as HTMLButtonElement;
/** The `+` chip at the end of the row opens the dialog that asks for a name and a colour. */
const openAdd = () =>
  fireEvent.click(
    within(panel()).getByRole("button", { name: "Mark a player" }),
  );

/** Lets the order's answer arrive and the map redraw, as it does a moment after a click. */
async function settle() {
  await act(async () => {
    await new Promise((res) => setTimeout(res, 0));
  });
}

/** The test world holds one other player; the suggestion list needs a few to filter between. */
function addPlayers(...names: string[]) {
  for (const name of names) {
    const id = `bot_${name.toLowerCase()}`;
    g.players[id] = {
      ...g.players.bot_test!,
      id,
      name,
      villageIds: [],
    };
  }
}

/** The marks the map holds, as the server last answered them. */
const marksNow = () => useGameStore.getState().game?.marks ?? [];

async function mark(name: string) {
  openAdd();
  fireEvent.change(nameField(), { target: { value: name } });
  fireEvent.click(screen.getByRole("button", { name: "Mark" }));
  await settle();
}

beforeEach(() => {
  setLocale("en");
  localStorage.clear();
  g = newTestGame();
  bot = makeBotVillage(g);
  sent = [];
  useUiStore.setState({ view: "map", mapCenter: null });
});
afterEach(() => {
  cleanup();
  useGameStore.getState().shutdown();
  vi.unstubAllGlobals();
});

describe("marking other players", () => {
  it("puts the marked names straight under the map, above the hint", async () => {
    stubMarks();
    await showMap();
    await mark(g.players.bot_test!.name);
    const hint = screen.getByText(/Click a village to open its menu/);
    expect(
      panel().compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // No panel box of its own: the marks are bare rows right under the map.
    expect(panel().tagName.toLowerCase()).not.toBe("fieldset");
  });

  it("shows a mark's colour in front of its name", async () => {
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    const row = within(panel()).getByRole("listitem");
    const swatch = within(row).getByLabelText(`Colour for ${name}`);
    const label = within(row).getByRole("button", { name });
    expect(
      swatch.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("puts the + chip after the last mark, in the same wrapping row", async () => {
    stubMarks();
    await showMap();
    const first = g.players.bot_test!.name;
    addPlayers("Grimgar");
    await mark(first);
    await mark("Grimgar");
    const add = within(panel()).getByRole("button", { name: "Mark a player" });
    const chips = [...panel().querySelectorAll(".mark-chip")];
    // The `+` is the row's last chip...
    expect(chips.at(-1)).toBe(add);
    // ...and an item of the same flex container as the marks, not a sibling of their list. A nested list fills the
    // line when its chips overflow, which pushed the `+` onto a line of its own even where there was room beside it.
    const row = panel().querySelector(".marks-row")!;
    const items = [...row.children];
    expect(items.at(-1)).toBe(add.parentElement);
    for (const chip of chips.slice(0, -1)) expect(chip.parentElement).toBe(row);
    expect(row.tagName.toLowerCase()).toBe("ul");
  });

  it("keeps the label with only the + chip until a player is marked", async () => {
    stubMarks();
    await showMap();
    // No mark to list, but the row is there for the `+` it holds.
    expect(within(panel()).queryAllByRole("listitem")).toHaveLength(0);
    expect(
      within(panel()).getByRole("button", { name: "Mark a player" }),
    ).toBeTruthy();
    // The label is what the `+` chip is for, so it stays put rather than appearing with the first mark.
    expect(panel().querySelector(".marks-label")?.textContent).toBe(
      "Marked players",
    );
  });

  it("puts the label above the row of marks", async () => {
    stubMarks();
    await showMap();
    await mark(g.players.bot_test!.name);
    const label = panel().querySelector(".marks-label")!;
    const list = within(panel()).getByRole("list");
    expect(
      label.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // Each part is its own line: the strip is a column, not a wrapping row.
    expect(panel().className).toMatch(/\bmarks-strip\b/);
  });

  it("asks for the name and colour in a dialog, not in the row", async () => {
    stubMarks();
    await showMap();
    // Nothing to type into until the + chip is pressed.
    expect(within(panel()).queryByRole("combobox")).toBeNull();
    openAdd();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Mark a player")).toBeTruthy();
    expect(within(dialog).getByRole("combobox")).toBeTruthy();
    expect(within(dialog).getByLabelText("Colour")).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: "Mark" })).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeTruthy();
  });

  it("adds nothing until Mark is pressed, and closes the dialog after", async () => {
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), {
      target: { value: g.players.bot_test!.name },
    });
    expect(marksNow()).toEqual([]);
    fireEvent.click(markButton());
    expect(screen.queryByRole("dialog")).toBeNull();
    await settle();
    expect(marksNow()).toHaveLength(1);
  });

  it("closes the dialog on Cancel without marking anyone", async () => {
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), {
      target: { value: g.players.bot_test!.name },
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(marksNow()).toEqual([]);
    // Reopening starts from an empty field rather than the abandoned name.
    openAdd();
    expect((nameField() as HTMLInputElement).value).toBe("");
  });

  it("draws a circle in the marked player's colour on the corner of their villages", async () => {
    stubMarks();
    const { container } = await showMap();
    expect(container.querySelector("[data-mark]")).toBeNull();
    await mark(g.players.bot_test!.name);
    const circle = container.querySelector(
      `[data-mark="${nextMarkColor([])}"]`,
    );
    expect(circle).toBeTruthy();
    expect(circle!.tagName.toLowerCase()).toBe("circle");
    expect(circle!.getAttribute("fill")).toBe(nextMarkColor([]));
  });

  it("marks the player's villages on the minimap too", async () => {
    stubMarks();
    const { container } = await showMap();
    expect(container.querySelector("[data-minimap-dot]")).toBeNull();
    await mark(g.players.bot_test!.name);
    expect(
      container.querySelector(`[data-minimap-dot="${nextMarkColor([])}"]`),
    ).toBeTruthy();
  });

  it("keeps the barbarians' dots on the minimap", async () => {
    stubMarks();
    const barb = barbarianVillage(g);
    const { container } = await showMap();
    // A barbarian is not a player and cannot be marked, but it has always had its small brown dot.
    const brown = [
      ...container.querySelectorAll("svg[data-minimap] rect"),
    ].filter((r) => r.getAttribute("fill") === "#5c4528");
    expect(brown.length).toBeGreaterThan(0);
    expect(barb.x).toBeGreaterThanOrEqual(0);
  });

  it("matches the name without regard to case or surrounding spaces", async () => {
    stubMarks();
    const { container } = await showMap();
    const name = g.players.bot_test!.name;
    await mark(`  ${name.toUpperCase()}  `);
    expect(marksNow()).toEqual([
      { name: name.toUpperCase(), color: nextMarkColor([]) },
    ]);
    expect(container.querySelector("[data-mark]")).toBeTruthy();
  });

  it("takes the mark off again once the removal is confirmed", async () => {
    stubMarks();
    const { container } = await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    expect(container.querySelector("[data-mark]")).toBeTruthy();
    // The name itself is the button that asks to remove it.
    fireEvent.click(within(panel()).getByRole("button", { name }));
    fireEvent.click(screen.getByRole("button", { name: "Yes" }));
    await settle();
    expect(marksNow()).toEqual([]);
    expect(container.querySelector("[data-mark]")).toBeNull();
    expect(container.querySelector("[data-minimap-dot]")).toBeNull();
  });

  it("keeps the mark when the removal is turned down", async () => {
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    fireEvent.click(within(panel()).getByRole("button", { name }));
    fireEvent.click(screen.getByRole("button", { name: "No" }));
    expect(marksNow()).toEqual([{ name, color: nextMarkColor([]) }]);
    expect(within(panel()).queryByRole("dialog")).toBeNull();
  });

  it("asks before removing, and does nothing until then", async () => {
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(within(panel()).getByRole("button", { name }));
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(`Remove the mark on ${name}?`),
    ).toBeTruthy();
    expect(marksNow()).toHaveLength(1);
  });

  it("leaves room under the buttons, so pressing one does not flash a scrollbar", async () => {
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    fireEvent.click(within(panel()).getByRole("button", { name }));
    // `.btn:active` sinks a button 1px, which pokes out of the dialog's scroll box and grows a scrollbar for a moment.
    expect(screen.getByRole("dialog").className).toMatch(/\bpb-px\b/);
  });

  it("keeps the dropdown shut until a letter is typed", async () => {
    addPlayers("Grimgar", "Wenard");
    stubMarks();
    await showMap();
    openAdd();
    // The field has focus (the dialog focuses it), but an untouched field gets no list.
    expect((nameField() as HTMLInputElement).value).toBe("");
    expect(screen.queryByRole("listbox")).toBeNull();
    fireEvent.focus(nameField());
    expect(screen.queryByRole("listbox")).toBeNull();
    fireEvent.change(nameField(), { target: { value: "G" } });
    expect(screen.getByRole("listbox", { name: "Players" })).toBeTruthy();
    // Emptying the field puts the list away again.
    fireEvent.change(nameField(), { target: { value: "" } });
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("offers at most five names at once, and narrows them as the name is typed", async () => {
    addPlayers(
      "Grimgar",
      "Wenard",
      "Norius",
      "Marus",
      "Dobald",
      "Corric",
      "Bran",
    );
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "r" } });
    const list = screen.getByRole("listbox", { name: "Players" });
    // Six names hold an "r"; enough to pick from, few enough not to bury the dialog's Cancel and Mark.
    expect(within(list).getAllByRole("option")).toHaveLength(5);
    fireEvent.change(nameField(), { target: { value: "Mar" } });
    expect(
      within(list)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["Marus"]);
  });

  it("offers the world's players in a dropdown that filters as the name is typed", async () => {
    addPlayers("Grimgar", "Wenard");
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "e" } });
    const list = screen.getByRole("listbox", { name: "Players" });
    expect(within(list).getAllByRole("option").length).toBeGreaterThan(1);
    fireEvent.change(nameField(), { target: { value: "Gri" } });
    const options = within(list).getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["Grimgar"]);
    // Picking one fills the field and closes the list.
    fireEvent.mouseDown(within(list).getByRole("option", { name: "Grimgar" }));
    expect((nameField() as HTMLInputElement).value).toBe("Grimgar");
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("leaves names already marked out of the dropdown", async () => {
    addPlayers("Grimgar", "Wenard");
    stubMarks();
    await showMap();
    await mark("Grimgar");
    openAdd();
    fireEvent.change(nameField(), { target: { value: "r" } });
    const list = screen.getByRole("listbox", { name: "Players" });
    expect(
      within(list)
        .queryAllByRole("option")
        .map((o) => o.textContent),
    ).not.toContain("Grimgar");
  });

  it("walks the dropdown with the arrow keys and takes the highlighted name with Enter", async () => {
    addPlayers("Grimgar", "Wenard");
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "e" } });
    const list = screen.getByRole("listbox", { name: "Players" });
    const first = within(list).getAllByRole("option")[0]!;
    expect(first.getAttribute("aria-selected")).toBe("true");
    fireEvent.keyDown(nameField(), { key: "ArrowDown" });
    const second = within(list).getAllByRole("option")[1]!;
    expect(second.getAttribute("aria-selected")).toBe("true");
    fireEvent.keyDown(nameField(), { key: "Enter" });
    expect((nameField() as HTMLInputElement).value).toBe(second.textContent);
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("closes the dropdown on Esc without closing the map's own menus", async () => {
    addPlayers("Grimgar");
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "G" } });
    expect(screen.getByRole("listbox")).toBeTruthy();
    fireEvent.keyDown(nameField(), { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("refuses a name no player has, and says so while it is typed", async () => {
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "Nobody" } });
    expect(panel().querySelector('[data-mark-problem="unknown"]')).toBeTruthy();
    // The warning is not the only guard: there is nothing to mark, so Mark is off and does nothing.
    expect(markButton().disabled).toBe(true);
    fireEvent.click(markButton());
    expect(marksNow()).toEqual([]);
    expect(screen.getByRole("dialog")).toBeTruthy();
    // Correcting the name to one a player has clears the warning and lets the mark through.
    fireEvent.change(nameField(), {
      target: { value: g.players.bot_test!.name },
    });
    expect(panel().querySelector("[data-mark-problem]")).toBeNull();
    expect(markButton().disabled).toBe(false);
    fireEvent.click(markButton());
    await settle();
    expect(marksNow()).toEqual([
      { name: g.players.bot_test!.name, color: nextMarkColor([]) },
    ]);
  });

  it("refuses a player who is already marked, and says so while it is typed", async () => {
    // The world's players come in with the state, so a second one has to be there before the map loads.
    addPlayers("Grimgar");
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    openAdd();
    fireEvent.change(nameField(), { target: { value: name.toUpperCase() } });
    // The reason is the same line the unknown name gets, so the dialog keeps its one height.
    const note = screen.getByRole("dialog").querySelector("p.marks-note")!;
    expect(note.getAttribute("data-mark-problem")).toBe("already");
    expect(note.textContent).toContain(
      `Player "${name.toUpperCase()}" is already marked.`,
    );
    expect(markButton().disabled).toBe(true);
    fireEvent.click(markButton());
    await settle();
    // Nothing changed: still one mark, in the colour it had, and the dialog is still open.
    expect(marksNow()).toEqual([{ name, color: nextMarkColor([]) }]);
    expect(screen.getByRole("dialog")).toBeTruthy();
    // A different player is fine again.
    fireEvent.change(nameField(), { target: { value: "Grimgar" } });
    expect(note.getAttribute("data-mark-problem")).toBeNull();
    expect(markButton().disabled).toBe(false);
  });

  it("does not add an already-marked player when the form is submitted anyway", async () => {
    stubMarks();
    await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    openAdd();
    fireEvent.change(nameField(), { target: { value: name } });
    fireEvent.submit(screen.getByRole("dialog").querySelector("form")!);
    await settle();
    expect(marksNow()).toEqual([{ name, color: nextMarkColor([]) }]);
  });

  it("keeps the warning's space whether or not it says anything, so the dialog holds one height", async () => {
    stubMarks();
    await showMap();
    openAdd();
    const note = () =>
      screen.getByRole("dialog").querySelector("p.marks-note") as HTMLElement;
    // The paragraph is always laid out; only its text and visibility change. A paragraph that appeared and vanished
    // would grow and shrink the dialog every time a name crossed between known and unknown.
    expect(note()).toBeTruthy();
    expect(note().className).toMatch(/\binvisible\b/);
    expect(note().getAttribute("aria-hidden")).toBe("true");
    fireEvent.change(nameField(), { target: { value: "Nobody" } });
    expect(note().className).not.toMatch(/\binvisible\b/);
    expect(note().getAttribute("aria-hidden")).toBeNull();
    expect(note().textContent).toContain("Nobody");
    fireEvent.change(nameField(), {
      target: { value: g.players.bot_test!.name },
    });
    expect(note().className).toMatch(/\binvisible\b/);
    expect(note().textContent).not.toContain("No player called");
  });

  it("does not mark an unknown name even when the form is submitted anyway", async () => {
    stubMarks();
    await showMap();
    openAdd();
    fireEvent.change(nameField(), { target: { value: "Nobody" } });
    // Enter in the field reaches the form even with Mark disabled, so the submit path checks the name too.
    const form = screen.getByRole("dialog").querySelector("form")!;
    fireEvent.submit(form);
    expect(marksNow()).toEqual([]);
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("recolours a mark", async () => {
    stubMarks();
    const { container } = await showMap();
    const name = g.players.bot_test!.name;
    await mark(name);
    const picker = within(panel()).getByLabelText(`Colour for ${name}`);
    fireEvent.change(picker, { target: { value: "#123456" } });
    await settle();
    expect(marksNow()[0]!.color).toBe("#123456");
    expect(container.querySelector('[data-mark="#123456"]')).toBeTruthy();
  });

  it("keeps the mark when the owner's own colours are switched off", async () => {
    stubMarks();
    const { container } = await showMap();
    await mark(g.players.bot_test!.name);
    fireEvent.click(screen.getByRole("checkbox", { name: "Player colours" }));
    expect(container.querySelector("[data-mark]")).toBeTruthy();
    expect(container.querySelector("[data-minimap-dot]")).toBeTruthy();
  });

  it("sends the marks to the player's record, not to this browser", async () => {
    stubMarks();
    await showMap();
    await mark(g.players.bot_test!.name);
    // The server keeps them, so the same player on another device gets them back with the state; nothing about
    // them is written to this browser.
    expect(sent).toEqual([
      [{ name: g.players.bot_test!.name, color: nextMarkColor([]) }],
    ]);
    expect(localStorage.getItem("lh-ui-prefs")).toBeNull();
  });

  it("draws the marks the server sends, so another device shows the same ones", async () => {
    g.marks = [{ name: g.players.bot_test!.name, color: "#e6194b" }];
    stubMarks();
    const { container } = await showMap();
    // Nothing was marked in this browser: the mark came in with the state.
    expect(container.querySelector('[data-mark="#e6194b"]')).toBeTruthy();
    expect(
      within(panel()).getByRole("button", { name: g.players.bot_test!.name }),
    ).toBeTruthy();
  });

  it("drops a stored mark that is not a name and a hex colour", () => {
    expect(
      cleanMarks([
        { name: "Keeper", color: "#abcdef" },
        { name: "NoColour" },
        { name: "  ", color: "#ffffff" },
        { name: "BadColour", color: "red" },
        { name: "KEEPER", color: "#000000" },
        "junk",
        null,
      ]),
    ).toEqual([{ name: "Keeper", color: "#abcdef" }]);
    expect(cleanMarks(undefined)).toEqual([]);
    expect(cleanMarks("nope")).toEqual([]);
    expect(markKey(" Keeper ")).toBe("keeper");
  });
});

describe("your own villages", () => {
  /** The circle on a village's tile, by the attribute that says whose circle it is. */
  const circleAt = (
    c: HTMLElement,
    v: { x: number; y: number },
    attr: string,
  ) =>
    c
      .querySelector(`[data-tile-village="v:${v.x}:${v.y}"] [${attr}]`)
      ?.getAttribute(attr);

  it("marks every village of yours yellow, the one you are in too", async () => {
    stubMarks();
    const home = activeVillage(g);
    const other = addOwnVillage(g, home.x + 2, home.y, "Second village");
    const { container } = await showMap();
    // One colour for all of them: the village you are in is already told apart by the map's gold outline, so a second
    // circle colour only made the same fact say two things.
    expect(circleAt(container, home, "data-own-village")).toBe(
      OWN_VILLAGE_COLOR,
    );
    expect(circleAt(container, other, "data-own-village")).toBe(
      OWN_VILLAGE_COLOR,
    );
  });

  it("leaves other players' villages to the marks", async () => {
    stubMarks();
    const { container } = await showMap();
    expect(circleAt(container, bot, "data-own-village")).toBeUndefined();
    expect(circleAt(container, bot, "data-mark")).toBeUndefined();
  });

  it("draws both circles when a mark and your own village meet on one tile", async () => {
    stubMarks();
    const home = activeVillage(g);
    // Marking your own name is pointless in the panel, but a mark on the same name must not eat your circle.
    g.marks = [{ name: g.players[HUMAN_ID]!.name, color: "#e6194b" }];
    const { container } = await showMap();
    expect(circleAt(container, home, "data-mark")).toBe("#e6194b");
    expect(circleAt(container, home, "data-own-village")).toBe(
      OWN_VILLAGE_COLOR,
    );
  });

  it("still draws them with Player colours off", async () => {
    stubMarks();
    const home = activeVillage(g);
    const { container } = await showMap();
    fireEvent.click(screen.getByRole("checkbox", { name: "Player colours" }));
    expect(circleAt(container, home, "data-own-village")).toBe(
      OWN_VILLAGE_COLOR,
    );
  });

  it("marks them on the minimap too", async () => {
    stubMarks();
    const home = activeVillage(g);
    const { container } = await showMap();
    expect(
      container.querySelector(`[data-minimap-own="${OWN_VILLAGE_COLOR}"]`),
    ).toBeTruthy();
    expect(home).toBeTruthy();
  });
});

describe("nextMarkColor", () => {
  it("starts from a colour no other mark uses, and wraps once the palette is spent", () => {
    const taken: { name: string; color: string }[] = [];
    for (let i = 0; i < MARK_COLORS.length; i++) {
      taken.push({ name: `p${i}`, color: nextMarkColor(taken) });
    }
    expect(new Set(taken.map((m) => m.color)).size).toBe(MARK_COLORS.length);
    // The palette is spent, so the next mark takes a colour again rather than none.
    expect(nextMarkColor(taken)).toBe(taken[0]!.color);
  });
});
