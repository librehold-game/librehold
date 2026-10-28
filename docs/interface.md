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
  in the owner's colour, slate for barbarians, a banner for a bonus and for the active village).

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

- Clicking a village on the map opens a menu next to the tile (Esc closes it). Own village: resources, population, construction, troops, incoming attacks, Village/Switch/Support/Reports/More… buttons. Foreign village: distance and march times, the latest intel (only what the player got by scouting/attacking), the last attack, the threat from that village, our commands on the way, the 3 latest reports (a click opens them) and "all →" (Reports filtered by the village), Attack/Support/Scout/Template A/More… buttons.
- Hovering a village shows a card describing it (`src/ui/map/MapHoverCard.tsx`): name, coordinates and continent (K + hundreds of y + hundreds of x), points, the player with their ranking points, bonus, the last attack with loot, troops from scouting, wall, distance; for an own village resources, Farmstead, troops and incoming attacks. It disappears while dragging the map; on touch there is none (a tap opens the menu).
- The village icon depends on points — thresholds in `server/internal/game/data_world.go` (`MapConfig.VillageSizePoints`): < 300 hut, 300 fenced settlement, 1,000 palisade, 3,000 stone wall, 9,000 stronghold with towers, 11,000 castle. Art: `src/ui/map/PixelVillage.tsx`.
- Menu data: `src/ui/map/villageInfo.ts` (a pure function, tested in `tests/map.test.ts`).

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
