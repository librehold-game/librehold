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
- Green badges = building level (a click highlights the row in the table). A building that is not built has only a click area (a construction site while it is being built). The building table shows only buildings whose requirements are met; when resources are short, the Cost column shows the time until they are gathered.
- Right sidebar: Production (per hour), Units (troops in the village, "» recruit"), Current parameters. Between the building table and the build plan a Movements heading appears only while something is on the road. It is drawn with the same `CommandsTable` the Overview's Commands and Trading tabs use — the same four columns (type icon, route, units/loot, arrival), a countdown *and* the local clock time of the arrival, and `table-stack` turning the rows into cards on a phone; the type is an icon named by its title, as on the Overview. Each row shows the troops it carries (unit icons + counts) or the exact resources of its haul (resource icons + amounts) plus the merchant count; a haul another player is sending you has its countdown highlighted and is labelled incoming. A shipment between two of your own villages is listed on both ends — the cart goes out on the sender's panel and its arrival on the receiver's — and is not marked incoming, because it is yours. The panel holds **only this village's movements** — another of your villages' marches belongs on that village's panel — and shows at most `MOVEMENTS_PER_PAGE` (8) per page, soonest home first, with Prev/Next and a `1 / N` counter once there is more than one page. Page state is dropped when you switch village, so the counter never points into another village's list. The card is a `SidePanel` (`src/ui/components/SidePanel.tsx`) like Production, Units and Current parameters — same wooden header bar, italic title and − / + button — and that is the only one of them with no `defaultOpen`: its fold is a browser preference (`uiStore.prefs.movementsCollapsed`), so it survives the card unmounting when the last movement lands. The panel is mounted once more, at the bottom of the Market, with `scope="trade"` (see below), where the same toggle folds it.

## Map — village menu and sizes

- Clicking a village on the map opens a menu next to the tile (Esc closes it). Own village: resources, population, construction, troops, incoming attacks, Village/Switch/Support/Reports/More… buttons, plus collapsed Quick build and Quick recruit sections. Quick build queues the next level of an available building; Quick recruit orders units from recruitment buildings already built in that village. Both use the same client-side checks and server orders as the village screens. Foreign village: distance and march times, the latest intel (only what the player got by scouting/attacking), the last attack, the threat from that village, our commands on the way, the 3 latest reports (a click opens them) and "all →" (Reports filtered by the village), Attack/Support/Scout/Template A/More… buttons.
- Hovering a village shows a card describing it (`src/ui/map/MapHoverCard.tsx`): name, coordinates and continent (K + hundreds of y + hundreds of x), points, the player with their ranking points, bonus, the last attack with loot, troops from scouting, wall, distance; for an own village resources, Farmstead, troops and incoming attacks. It disappears while dragging the map; on touch there is none (a tap opens the menu).
- The village icon depends on points — thresholds in `server/internal/game/data_world.go` (`MapConfig.VillageSizePoints`): < 300 hut, 300 fenced settlement, 1,000 palisade, 3,000 stone wall, 9,000 stronghold with towers, 11,000 castle. Art: `src/ui/map/PixelVillage.tsx`.
- Menu data: `src/ui/map/villageInfo.ts` (a pure function, tested in `tests/map.test.ts`).
- "Show farming" (on by default) takes the player's farm raids off the map, the armies and the command dots alike:
  raids sent from a farm template or by the auto-farm, and their way home. The server keeps the choice per player and
  world (`hide-farm-on-map`), and only the sender's state marks a command as a farm raid (`farm`).

## Market

Everything trading looks like lives in `src/ui/views/MarketScreen.tsx`, and every table uses the shared
`table-game` styling (wooden header, zebra rows, `table-stack` cards on a phone).

- **Recall:** a settled shipment (a leg of an accepted offer) shows a disabled Recall, since the server refuses it
  (`settledTrade`). An ordinary market trade keeps its five-minute window.
- **World page and forms:** `PublicSettings.tradingEnabled` (always on the wire) makes the setting visible on a world's
  page before joining, as a `Trading` row with on/off plus a separate `Offer lasts (hours)` row shown while it is on.
  The admin world form has it as a live-editable checkbox (`FIELDS`), and the hosting form has it as
  "Players and bots can trade", defaulting to on.
- **Trading switched off in the world** (`WorldConfig.TradingEnabled` false): the screen looks the way the market did
  before players could trade with each other — the merchant line with the reason under it, the transfer form for your own
  villages, and the travelling trader. `Your trade offers`, `Global Market` and the merchant-shipments panel are not
  rendered, because those exist only for player-to-player deals. Moving resources between your own villages and the
  travelling trader's fixed-rate exchange are not deals with another player, so the server keeps them open; posting,
  accepting and sending to somebody else refuse with `tradingOff`, and an admin flipping the switch empties the board
  (`CloseMarket` refunds every open offer).
- **Merchant line** above the panels: "Merchants: free / total", with the capacity hint (1 merchant = 1000 resources,
  6 min/field). The market level decides the total — one per level to 11, then quadratic (level 19 = 91, level 25 = 235) — so
  this line is also the limit on how big a single trade can be. The building table's Market row shows the same number, so a
  level-up reads "91 merchants" rather than the level. The transfer form under the line (to one of your other villages,
  with **Surplus** and **Split evenly**) keeps **Send resources** pressable while the transfer is refused and raises the
  reason in the toast like Accept does, so the row of amounts and buttons never grows a sentence that a phone card would
  cut off.
- **Your trade offers** panel: only offers you posted (`myOffers` keeps `fromId === HUMAN_ID`). One table with the columns
  Offer | For | From village | Max delivery | Ratio | Expires | Action, and Withdraw as the only action — taking a deal is
  what the Global Market below is for. The second column names the posting village rather
  than repeating your own name, because the origin is what matters here: its Market supplies the merchants and the
  offer cap is per village, so identical deals from two of your villages stay on separate rows and each village
  fills its own slots. The fourth column is the **delivery limit you set when posting** ("no limit" for 0), not the
  merchant trip the board shows — on a public post there is no taker yet, so the trip is unknowable and would read as a dash. Identical deals — same sender, same delivery, same amounts —
  collapse into one row with a `×N` count, exactly like the board. `For` is tinted green where your village holds enough and
  red where it does not. On the board, Accept stays pressable while a deal is refused, and pressing
  it raises the reason in the app's error toast — "Not enough merchants: 2 needed, 1 free", "Needs 2 merchants but the
  market only has 1. Upgrade the Market to get more merchants" — rather than writing it into the row. A sentence under
  the button does not fit a phone card, and on a desktop it made the row taller for every offer on screen. A grouped
  row of your own offers puts a **withdraw-count** box next to the button (label "Withdraw N"), so copies can be taken
  back a few at a time rather than all at once; with a single copy the button is a plain "Withdraw".
- **Post an offer** (collapsed under a summary): the delivery limit in hours (0 = no limit), the give and get resource
  rows, **"How many:"** copies and the submit button, which reads "Post N offers" when the count is above one. There is no
  offer type to choose: every post goes on the board, because a targeted offer would only let one named village take it.
  The form shows no count of what is left: the limit is a concurrency one — six distinct deals per village at a time
  (`TradeBalance.PublicPerVillage`) — not a daily allowance. Once this village has its quota of distinct deals out, opening
  the form raises the refusal in the error toast and the submit button stays disabled with the same reason, while a copy of
  a deal already out still posts because copies do not count.
- **Global Market** panel: everyone else's public offers, as a table with the same columns plus Available. Identical deals
  — same poster, same delivery time, same amounts — collapse into one row with `×N`, and a take-N number box next to Accept
  posts `offerIds` in a batch. Filters above the table: player search, player-or-bot, the resource it gives, the resource
  it wants, maximum delivery time, minimum ratio and the sort order; ten rows a page. Your own offers never appear here
  (`publicBoard` keeps `o.public && o.fromId !== HUMAN_ID`), which is what makes the two panels disjoint: the Global
  Market is everyone else's public posts, Your trade offers is your own.
- **Travelling trader** panel: the market's own recurring single-resource offer, unchanged.
- **Merchant shipments** (bottom of the screen, under the travelling trader): the same Movements panel the village
  screen shows, mounted with `scope="trade"` so only merchant shipments appear — an attack or a support has nothing
  to do with the market and would only be noise here. It lists what this village's merchants are carrying and what
  another player is sending to it, with route, resources, merchant count and arrival countdown. Like the village
  panel it renders nothing when there is no shipment, so the market does not grow an empty heading, and it pages at
  eight as the village panel does.

## Overview

- The Overview is a set of tabs over every village in view, or over one village group picked on the right. **Construction**, **Units**, **Recruitment**, **Resources**, **Trading**, **Commands** and **Groups**.
- **Commands** lists the troop movements of those villages: attacks, support and returns, soonest arrival first. Merchant shipments are deliberately not here; they have their own tab. Both tabs are drawn with `CommandsTable` in `src/ui/components/CommandsTable.tsx` — the same table the village Movements panel uses.
- **Trading** is the same table (`CommandsTable`) filled with the merchant shipments of those villages, soonest first, including a shipment another player is bringing to one of them and both legs of a shipment between two of your own villages. The split is by kind, so no movement appears on both tabs. Neither tab pages: the full list is what the page is for.

## Reports

- **Filters:** all, unread, attacks, scouting, conquests, support, and **market** (settled trade deals).
- **Market reports.** When a deal settles (`AcceptTradeOffers`), each human side gets **one** report for the whole deal —
  a taker who picks up ten copies of a post reads one report, not ten; its totals cover every offer. The report says who
  the other party was (with the bot tag when it was a bot), the village the reader's incoming resources are shipped from
  and how long that shipment takes (`TradeTravelMs`), what the reader gives and what they get, and how many offers
  settled. Both sides are written by `sendMarketReports` in `src/game/trade.go`; the older world-news line still fires
  alongside it.
- A market report is drawn by `MarketReportDetail` in `src/ui/views/ReportsView.tsx` rather than the battle layout: it
  has no units, luck, wall or loot to show. In the list its "Loot" column shows what the player receives.

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
  stay tables with horizontal scrolling — splitting them into cards gains nothing. A cell must not carry its own
  `text-align`: in card mode the label is a flex item, so a right-aligned value would push the value across while the
  label stayed put, breaking the column of labels. The label column is `min(6.5rem, 45%)` wide, so it narrows on the
  smallest phones instead of pushing a longer value onto a second line — a player name with a bot tag after it must stay
  on one line, so **put such a name and its tag in one `inline-flex` `whitespace-nowrap` element** rather than letting
  them be two flex items of the cell. For a cell of controls only, add `action-end` next to `stack-inline` — the card CSS
  gives it the full width and pushes the controls to the right edge.
- **Toasts.** The error toast (`src/ui/components/ErrorToast.tsx`) sits at the bottom right, but below `md` the tab bar
  is fixed to the same edge and covers anything near it, so there the toast moves up by the page's own bottom padding
  (`62px` plus `env(safe-area-inset-bottom)`). `max-md` is exactly where the bar exists, since `Nav` hides it from `md`
  up. A toast placed at the bare `bottom-4` is half hidden behind the bar — `tests/mobile.test.tsx` guards this.
- **Map.** The window size is computed by `mapViewport(availW, availH, zoom)` from `src/ui/map/viewport.ts` — on
  desktop it comes out at the default 15 × 13 tiles, on a phone the window shrinks to the screen width
  and the number of tiles follows from the size. The side panel and the minimap move below the map when space runs out
  (the `SIDE_COL` threshold), and the village menu slides in from the bottom instead of sticking to the tile. Dragging
  works on `pointerdown` (mouse and finger), a **two-finger pinch** changes the zoom level (`pinchZoomDelta`,
  threshold 1.3 — below that it is finger jitter), and the threshold that tells a tap from a drag is 8 px.

On top of that: buttons and fields are at least 34 px with `pointer: coarse`, `overscroll-behavior-y: none` on `body`
(otherwise dragging the map down reloads the game) and `viewport-fit=cover` in `index.html`.
