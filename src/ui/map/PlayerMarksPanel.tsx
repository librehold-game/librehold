import { useEffect, useId, useMemo, useRef, useState } from "react";
import { HUMAN_ID } from "../../engine/types";
import { t as tr } from "../../i18n/i18n";
import { useGameStore } from "../../state/gameStore";
import { markKey, useUiStore, type PlayerMark } from "../../state/uiStore";
import { Overlay } from "../instant/Overlay";
import { nextMarkColor } from "./playerMarks";

/** The tallest the suggestion list gets: five rows at ~26px each, plus its padding. */
const MENU_MAX = 140;
/** Room kept between the list and the edge of the window. */
const MENU_GAP = 8;
/** How many names are offered at once: enough to pick from, few enough not to bury the dialog's buttons. */
const SUGGESTION_LIMIT = 5;

/** Where the suggestion list goes: under the field, or over it when there is no room below. */
type SuggestionBox = {
  left: number;
  top: number;
  minWidth: number;
  maxHeight: number;
};

/**
 * The suggestion list is fixed rather than absolute: the dialog it lives in scrolls its content, and a scroll box
 * clips anything absolute inside it, which left most of the list unreachable. So its place is measured from the field
 * instead, and follows the field when the dialog scrolls or the window is resized.
 */
function useSuggestionBox(
  field: React.RefObject<HTMLDivElement | null>,
  open: boolean,
) {
  const [box, setBox] = useState<SuggestionBox | null>(null);
  useEffect(() => {
    if (!open) {
      setBox(null);
      return;
    }
    const place = () => {
      const el = field.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const below = window.innerHeight - r.bottom - MENU_GAP;
      const above = r.top - MENU_GAP;
      // Under the field when that leaves more room, otherwise over it, so the list is never cut off by the window.
      const drop = below >= Math.min(MENU_MAX, above);
      setBox({
        left: r.left,
        top: drop ? r.bottom + 2 : r.top - Math.min(MENU_MAX, above) - 2,
        minWidth: r.width,
        maxHeight: Math.max(
          64,
          drop ? Math.min(MENU_MAX, below) : Math.min(MENU_MAX, above),
        ),
      });
    };
    place();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [field, open]);
  return box;
}

/**
 * Other players picked out by name, each in a colour the player chooses: a dot in that colour sits on the corner of
 * their villages and on their dots on the minimap. The marks are a display choice kept in the browser, so this sits
 * bare under the map, above the hint about how the map works: a row of chips, each with its colour in front of the
 * name, and a `+` chip at the end of the row that asks for a new one in a dialog.
 */
export function PlayerMarksPanel() {
  const marks = useUiStore((s) => s.prefs.mapPlayerMarks);
  const setPrefs = useUiStore((s) => s.setPrefs);
  const [name, setName] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<PlayerMark | null>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const field = useRef<HTMLDivElement>(null);
  const color = picked ?? nextMarkColor(marks);
  const titleId = useId();
  const addTitleId = useId();
  // Every player in the world is in the state, so the names can be offered as suggestions even where the map has not
  // loaded their villages yet. Marking yourself is pointless: your own villages are already told apart.
  const known = useGameStore((s) => s.game?.players);
  const names = useMemo(
    () =>
      Object.values(known ?? {})
        .filter((p) => p.id !== HUMAN_ID && p.status !== "pending")
        .map((p) => p.name)
        .sort((a, b) => a.localeCompare(b)),
    [known],
  );
  const typed = name.trim();
  const unknown =
    typed.length > 0 && !names.some((n) => markKey(n) === markKey(typed));
  // The list waits for a letter: a full list over an untouched field is noise, and typing is what narrows it.
  const suggesting = open && typed.length > 0;
  const box = useSuggestionBox(field, suggesting);
  // Names already marked are not offered again: marking one again would only recolour it. Only a few are offered:
  // enough to pick from without pushing the dialog's buttons off the screen, and typing narrows them anyway.
  const matches = useMemo(() => {
    const taken = new Set(marks.map((m) => markKey(m.name)));
    const q = markKey(typed);
    return names
      .filter(
        (n) => !taken.has(markKey(n)) && (q === "" || markKey(n).includes(q)),
      )
      .slice(0, SUGGESTION_LIMIT);
  }, [names, marks, typed]);

  // A click anywhere else, or Esc, closes the list. Esc is left to the map when the list is already closed.
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => {
      if (!field.current?.contains(e.target as Node)) setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopPropagation();
      setOpen(false);
    };
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", esc, true);
    return () => {
      document.removeEventListener("mousedown", away);
      document.removeEventListener("keydown", esc, true);
    };
  }, [open]);
  useEffect(() => setActive(0), [typed]);

  const add = () => {
    // Only a name some player has can be marked: the dialog says so as it is typed, and Mark stays off meanwhile.
    if (!typed || unknown) return;
    const key = markKey(typed);
    const rest = marks.filter((m) => markKey(m.name) !== key);
    setPrefs({ mapPlayerMarks: [...rest, { name: typed, color }] });
    closeAdd();
  };
  const openAdd = () => {
    setName("");
    setPicked(null);
    setOpen(false);
    setAdding(true);
  };
  const closeAdd = () => {
    setName("");
    setPicked(null);
    setOpen(false);
    setAdding(false);
  };
  const remove = (mark: PlayerMark) => {
    const key = markKey(mark.name);
    setPrefs({ mapPlayerMarks: marks.filter((m) => markKey(m.name) !== key) });
    setRemoving(null);
  };
  const recolor = (key: string, c: string) =>
    setPrefs({
      mapPlayerMarks: marks.map((m) =>
        markKey(m.name) === key ? { ...m, color: c } : m,
      ),
    });

  return (
    <section
      className="marks-strip"
      role="group"
      aria-label={tr("map.marks.title")}
      data-player-marks
    >
      <h3 className="sr-only">{tr("map.marks.title")}</h3>
      {/* The label stays even with nothing marked: it is what the `+` chip below it is for. */}
      <span className="marks-label" aria-hidden>
        {tr("map.marks.title")}
      </span>
      <ul className="marks-row">
        {marks.map((m) => (
          <li key={markKey(m.name)} className="mark-chip">
            <input
              type="color"
              className="mark-dot"
              aria-label={tr("map.marks.color", { name: m.name })}
              title={tr("map.marks.color", { name: m.name })}
              value={m.color}
              onChange={(e) => recolor(markKey(m.name), e.target.value)}
            />
            <button
              type="button"
              className="mark-name"
              onClick={() => setRemoving(m)}
            >
              {m.name}
            </button>
          </li>
        ))}
        {/* The `+` is the row's last chip, in the same flex flow as the marks: it trails the last one when there is
            room and wraps to the next line like any other chip when there is not. Presentational so the list stays a
            list of marked players, which the `+` is not. */}
        <li role="presentation">
          <button
            type="button"
            className="mark-chip mark-add"
            aria-label={tr("map.marks.addTitle")}
            title={tr("map.marks.addTitle")}
            onClick={openAdd}
          >
            +
          </button>
        </li>
      </ul>
      {adding && (
        <Overlay titleId={addTitleId} onClose={closeAdd}>
          <h2 className="panel-header" id={addTitleId}>
            {tr("map.marks.addTitle")}
          </h2>
          <form
            className="marks-add"
            onSubmit={(e) => {
              e.preventDefault();
              add();
            }}
          >
            <div className="marks-add-row">
              <div className="marks-field" ref={field}>
                <input
                  className="w-32 px-1.5 py-0.5"
                  placeholder={tr("map.marks.name")}
                  aria-label={tr("map.marks.name")}
                  role="combobox"
                  aria-expanded={suggesting && matches.length > 0}
                  aria-controls="map-player-marks-names"
                  aria-autocomplete="list"
                  aria-activedescendant={
                    suggesting && matches[active]
                      ? `map-player-marks-${markKey(matches[active]!)}`
                      : undefined
                  }
                  autoComplete="off"
                  autoFocus
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value);
                    setOpen(true);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                      if (!suggesting) return;
                      e.preventDefault();
                      const step = e.key === "ArrowDown" ? 1 : -1;
                      setActive(
                        (i) => (i + step + matches.length) % matches.length,
                      );
                    } else if (
                      e.key === "Enter" &&
                      suggesting &&
                      matches[active]
                    ) {
                      e.preventDefault();
                      setName(matches[active]!);
                      setOpen(false);
                    }
                  }}
                />
                {suggesting && matches.length > 0 && box && (
                  <ul
                    className="marks-menu"
                    id="map-player-marks-names"
                    role="listbox"
                    aria-label={tr("map.marks.suggestions")}
                    style={{
                      left: box.left,
                      top: box.top,
                      minWidth: box.minWidth,
                      maxHeight: box.maxHeight,
                    }}
                  >
                    {matches.map((n, i) => (
                      <li
                        key={n}
                        id={`map-player-marks-${markKey(n)}`}
                        role="option"
                        aria-selected={i === active}
                        className="marks-option"
                        // mousedown, not click: the field loses focus first and would close the list.
                        onMouseDown={(e) => {
                          e.preventDefault();
                          setName(n);
                          setOpen(false);
                        }}
                        onMouseEnter={() => setActive(i)}
                      >
                        {n}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <input
                type="color"
                className="mark-dot mark-dot-pick"
                aria-label={tr("map.marks.pick")}
                title={tr("map.marks.pick")}
                value={color}
                onChange={(e) => setPicked(e.target.value)}
              />
            </div>
            {/* The note's space is held whether or not it says anything: it is laid out for the name being typed and
                hidden while that name is fine, so the dialog keeps one height instead of growing the moment a name
                turns out to be unknown. Hidden text is out of the accessibility tree, so nothing is announced. */}
            <p
              className={`text-muted marks-note${unknown ? "" : " invisible"}`}
              data-mark-unknown={unknown ? "" : undefined}
              aria-hidden={unknown ? undefined : true}
            >
              {unknown ? tr("map.marks.unknown", { name: typed }) : ""}
            </p>
            <div className="flex justify-end gap-1 w-full">
              <button className="btn" type="button" onClick={closeAdd}>
                {tr("map.marks.cancel")}
              </button>
              <button
                className="btn"
                type="submit"
                disabled={!typed || unknown}
              >
                {tr("map.marks.add")}
              </button>
            </div>
          </form>
        </Overlay>
      )}
      {removing && (
        <Overlay titleId={titleId} onClose={() => setRemoving(null)}>
          <h2 className="panel-header" id={titleId}>
            {tr("map.marks.remove", { name: removing.name })}
          </h2>
          <p className="text-muted text-[12px] mt-2">
            {tr("map.marks.removeHint")}
          </p>
          <div className="flex justify-end gap-1 mt-3">
            <button className="btn" onClick={() => setRemoving(null)}>
              {tr("map.marks.no")}
            </button>
            <button className="btn" onClick={() => remove(removing)}>
              {tr("map.marks.yes")}
            </button>
          </div>
        </Overlay>
      )}
    </section>
  );
}
