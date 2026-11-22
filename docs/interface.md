# Interface

The village view, the map, the art layers and the phone layout.

## Village illustration

The village illustration is a purely presentational layer: `src/ui/scene/sceneModel.ts` computes a scene model from `GameState` (building art stages, construction sites, smoke, alarm, training ground, research progress…), and `pixel/PixelVillageScene.tsx` assembles it into an 880×600 SVG from ready pixel art sprites (`src/assets/pixel/village/`, exported with `npm run assets:pixel` — see `ASSETS.md`). Layers are sorted by `z` depth; the wall is a ring in 8 px bands split into a back and a front half, so buildings inside are drawn between them. Building animations are frame strips with `steps()`, while villagers, guards and the gate are driven by a single `requestAnimationFrame` loop (`pixel/Walkers.tsx`).

- **Every building has three separate pieces of art** — not one rescaled. Level → tier is computed by `variantFor(level, maxLevel)` (`ceil(level/max·3)`), files `buildings/<id>-<tier>.png`. Preview of the whole village outside the game: `python3 scripts/pixel/compose.py` (usage in the script header).
- Modes: Settings → Interface → Village animations **full / limited / off**; `prefers-reduced-motion` ⇒ limited by default; paused when the tab is hidden. In limited mode the frame animations (mill wheel, ironworks fire, grain), the gate and the villagers stop; status markers stay (construction site, upgrade progress bar, ⏳ for the queue, alarm, smoke after a defeat).
- Constants: `SCENE_BALANCE` (`sceneModel.ts`) — cycle 1200–6000 ms, beat throttling 800 ms, clearing beats after a skip > 60 s, a limit of 30 animated elements, alarm/ruins windows.
- Sounds: `src/ui/sound.ts` (WebAudio, synthesised), turned on in Settings.
- Map: pixel art terrain drawn in chunks on a canvas (`src/ui/map/TerrainLayer.tsx`): a biome field from the game noise
  (`biomes.ts`, `terrain.ts`), Wang tiles, trees, peaks and meadow variety (`terrainPaint.ts`); the minimap from the
  same field (`minimap.ts`). Terrain does not affect gameplay. Art and export: `ASSETS.md`.
- **Villages on the map have six development stages** (points thresholds in `MAP.villageSizePoints`): hut → fenced
  settlement → palisade → stone wall → stronghold → castle; pixel art sprites in `src/ui/map/PixelVillage.tsx` (roofs
  in the owner's colour, slate for barbarians, a banner for a bonus).

## Materials (UI chrome and fonts)

The UI chrome is our own pixel art as well: panels, panel headers, buttons, tabs, table headers, form fields, the
bottom bar, the scrollbar thumb, the page background and the `.frame-wood` frame of the map and the minimap are
9-slice pieces from `src/assets/pixel/ui/chrome/` (exported with `npm run assets:pixel-ui`). `src/ui/pixelSkin.css`
applies them, every rule scoped to `html.pixel` (set in `game.html`); the account site uses the same pieces
(`src/account/account.css`). The origin of every file is described in **`ASSETS.md`**, and `tests/assets.test.ts`
checks the fonts.

- `src/index.css` holds the base component styles under the skin, in flat colours from `@theme`; the map ground
  under the terrain is the flat `GRASS` colour (`MAP_GROUND` in `src/ui/map/MapArt.tsx`).
- Fonts (MedievalSharp and Cinzel, SIL OFL 1.1) are hosted locally (`src/assets/fonts/`), `game.html` does not query
  any CDN — the game works offline.

## Village view

- A 2.5D scene in a panel headed "Name (N points)" (the "−" button collapses the illustration; same as Settings → Interface → Village illustration). Art: pixel art from `src/assets/pixel/village/` (layout in `layout.json`), model: `src/ui/scene/sceneModel.ts`, scene assembly and animations: `src/ui/scene/pixel/`.
- The wall changes its look with the stage (1/3 and 2/3 of the maximum level): palisade (stages 1–2) → stone wall with towers (stage 3); the gate opens when a villager approaches. Buildings have three art stages by the same rule.
- Green badges = building level (a click highlights the row in the table). A building that is not built has only a click area (a construction site while it is being built).
- Right sidebar: Production (per hour), Units (troops in the village, "» recruit"), Current parameters. The building table shows only buildings whose requirements are met; when resources are short, the Cost column shows the time until they are gathered.

## Map — village menu and sizes

- Clicking a village on the map opens a menu next to the tile (Esc closes it). Own village: resources, population, construction, troops, incoming attacks, Village/Switch/Support/Reports/More… buttons, plus collapsed Quick build and Quick recruit sections. Quick build queues the next level of an available building; Quick recruit orders units from recruitment buildings already built in that village. Both use the same client-side checks and server orders as the village screens. Foreign village: distance and march times, the latest intel (only what the player got by scouting/attacking), the last attack, the threat from that village, our commands on the way, the 3 latest reports (a click opens them) and "all →" (Reports filtered by the village), Attack/Support/Scout/Template A/More… buttons.
- Hovering a village shows a card describing it (`src/ui/map/MapHoverCard.tsx`): name, coordinates and continent (K + hundreds of y + hundreds of x), points, the player with their ranking points, bonus, the last attack with loot, troops from scouting, wall, distance; for an own village resources, Farmstead, troops and incoming attacks. It disappears while dragging the map; on touch there is none (a tap opens the menu).
- The village icon depends on points — thresholds in `server/internal/game/data_world.go` (`MapConfig.VillageSizePoints`): < 300 hut, 300 fenced settlement, 1,000 palisade, 3,000 stone wall, 9,000 stronghold with towers, 11,000 castle. Art: `src/ui/map/PixelVillage.tsx`.
- Menu data: `src/ui/map/villageInfo.ts` (a pure function, tested in `tests/map.test.ts`).
- "Show farming" (on by default) takes the player's farm raids off the map, the armies and the command dots alike:
  raids sent from a farm template or by the auto-farm, and their way home. The server keeps the choice per player and
  world (`hide-farm-on-map`), and only the sender's state marks a command as a farm raid (`farm`).

## Map — marked players

Under the map, above the line that explains how the map works, sits the **Marked players** list (`[data-player-marks]`,
`src/ui/map/PlayerMarksPanel.tsx`). It is the answer to "I know where they are but I cannot pick them out": pick a
player's name and a colour, and every village of theirs gets a dot in that colour on the top-left corner of its tile,
and their dots on the minimap take it too. The corner dot is the load-bearing mark, not the roofs: a roof is a few
pixels at the default zoom.

- The list is bare, not a panel of its own. It stacks in two parts: the **Marked players** label, then one row of
  chips — the colour swatch in front of the name — ending in a `+` chip. The `+` opens a **Mark a player** dialog
  holding the name field, the colour and **Cancel** / **Mark**; nothing is added until **Mark**. The label stays put
  even with nothing marked, since it is what the `+` chip below it is for; then the `+` chip is all there is under it.
- The row wraps when the marks outgrow it, and the `+` wraps with them: it is the row's last chip rather than a
  sibling of the list, so it trails the last mark wherever there is room for it and only moves to the next line when
  there is not. A list nested in the row would fill the line as soon as its chips overflowed and push the `+` down
  even where there was room beside it. The `+` is a presentational item so the list stays a list of marked players.
- A name in the row is a button: clicking it asks `Remove the mark on <name>?` with **Yes** / **No** before the mark
  goes. The swatch stays a colour input, so a mark is recoloured in place.
- Names are matched trimmed and without regard to case, since players type them from memory. The dialog's field is a
  combobox (`role="combobox"`) over a listbox of the world's players, which stays shut until a letter is typed: an
  untouched field gets no list, and emptying it puts the list away again. It filters as the name is typed, offers at
  most five names (typing is what narrows it), and takes the arrow keys and Enter to pick one, Esc or a click
  elsewhere to close it; names already marked are left out, since marking one again would only recolour it. The list
  hangs under the field as a dropdown, but is fixed rather than absolute and placed from the field's measured box
  (`useSuggestionBox`), because the dialog scrolls its content and a scroll box clips anything absolute inside it —
  which left most of the list unreachable. A name no player has is refused: the dialog says so as it is typed and
  **Mark** stays off, and the submit path checks the name again so Enter cannot slip past it. That line is laid out
  whether or not it has anything to say (hidden rather than removed), so the dialog keeps one height while a name is
  typed instead of growing and shrinking as it crosses between known and unknown.
- The mark outlives "Player colours": that switch turns the owners' own colours off to calm a busy map, while a mark is
  a player's deliberate choice, and would be useless exactly when the map is at its busiest.
- The corner dot owns the corner; a command dot on the same village moves along (`TileArt` in `src/ui/map/MapArt.tsx`).
- Your own villages carry the same corner dot, and are not marks — you cannot recolour or unmark them: **yellow**
  (`OWN_VILLAGE_COLOR`) for every one of them. The village you are in is not picked out by colour, because the map
  already gives it a gold outline. They outlive "Player colours" for the same reason a mark does, and they reach the
  minimap (`data-minimap-own` there, `data-own-village` on the map). A marked player's circle keeps the corner and
  yours sits beside it (`data-mark` and `data-own-village` on the same tile).
- Marks are a display choice in `UiPrefs.mapPlayerMarks` (`src/state/uiStore.ts`), remembered per browser in
  `lh-ui-prefs` and not tied to a world, so they follow the player between worlds. The matching is `markLookup` in
  `src/ui/map/playerMarks.ts`; a new mark starts from a colour no other mark uses (`nextMarkColor`). The server knows
  nothing of them: a mark changes nothing but the drawing.
- The minimap draws marked players' villages from the map chunks as they load (`MapCache.loaded()`), because the
  player's state carries only their own villages; that is why their dots appear as the player pans the map.

## Phone (mobile layout)

The game works on screens from 360 px. Three mechanisms, all checked by `tests/mobile.test.tsx`:

- **Tabs.** Below 768 px the tabs at the top disappear and a bar appears at the bottom with the four main views
  (Village, Map, Rally Point, Reports) and a **More** button with the rest. The split is in `TABS` (`primary: true`) in
  `src/ui/layout/Nav.tsx`; the unread counters of hidden tabs add up on "More". The `0`–`9` shortcuts
  work unchanged. The bar respects `env(safe-area-inset-bottom)`.
- **Tables as cards.** The `table-stack` class (next to `table-game`) turns every row into a card below 700 px, and the
  column header moves in front of the value from the `data-label` attribute. A cell without `data-label` would be a
  bare number on a phone — a test checks this, so **a new column needs `data-label`** (or the `stack-inline` class
  for cells with only buttons, which stay on one line). Matrices (units × villages, losses in a report)
  stay tables with horizontal scrolling — splitting them into cards gains nothing.
- **Map.** The window size is computed by `mapViewport(availW, availH, zoom)` from `src/ui/map/viewport.ts` — on
  desktop it comes out at the default 15 × 13 tiles, on a phone the window shrinks to the screen width
  and the number of tiles follows from the size. The side panel and the minimap move below the map when space runs out
  (the `SIDE_COL` threshold), and the village menu slides in from the bottom instead of sticking to the tile. Dragging
  works on `pointerdown` (mouse and finger), a **two-finger pinch** changes the zoom level (`pinchZoomDelta`,
  threshold 1.3 — below that it is finger jitter), and the threshold that tells a tap from a drag is 8 px.

On top of that: buttons and fields are at least 34 px with `pointer: coarse`, `overscroll-behavior-y: none` on `body`
(otherwise dragging the map down reloads the game) and `viewport-fit=cover` in `index.html`.
