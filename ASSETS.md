# Art assets and licences

The village illustration, the map terrain and villages, the armies, the UI chrome and the icons are our own pixel art
(sections below); the only third-party material is the two fonts. All of our own art: © Szymon Kocur, all rights
reserved (`LICENSE-ART.md`); the code licence (`LICENSE`) does not cover it.

## Fonts — SIL Open Font License 1.1

The `.woff2` files live in `src/assets/fonts/` and are served locally so the game works without internet.
Licence texts: `src/assets/fonts/OFL-MedievalSharp.txt`, `src/assets/fonts/OFL-Cinzel.txt`.

| Font | Author | Used for |
| --- | --- | --- |
| [MedievalSharp](https://fonts.google.com/specimen/MedievalSharp) | wmk69 | `h1`–`h4` headings |
| [Cinzel](https://fonts.google.com/specimen/Cinzel) | Natanael Gama / The Cinzel Project | panel headers, tabs, table headers |

## Pixel art

The village: base, roads, buildings in three stages, scenery, wall, animations and villagers. © Szymon Kocur
(`LICENSE-ART.md`). Source files live in `art/pixel-spike/`, outside the repository. Only the export goes into the game:

`npm run assets:pixel` (`scripts/pixel/export.py`) → `src/assets/pixel/village/`. The scripts in `scripts/pixel/` are
the only source of the scene geometry (slots, roads, wall ellipse, gates); `compose.py` builds a preview outside the
game from the same layout (a day GIF and a night frame). Tests: `tests/pixelAssets.test.ts` (layout consistency, size
≤ 3 MB) and `tests/assets.test.ts` (no unused files).

| Directory | Contents |
|---|---|
| `layout.json` | 880×600 scene layout: file sizes, slots, square, gates, roads with route points, scenery, lights, wall bands, frame strips |
| `ground.png` | grass, terrain zones and the cobbled square, without roads |
| `roads/` | every road as a transparent overlay — the game lays only the roads of existing buildings |
| `buildings/` | `<id>-<1|2|3>.png` (statue only `-3`), construction site, rally point |
| `scenery/` | trees, bushes, rocks, fields, forest, cliffs, well, cart, fence, woodpile |
| `walls/` | palisade and stone wall: back and front half of the ring, gate frame strip |
| `anim/` | frame strips: mill wheel, ironworks fire, grain |
| `villagers/` | `<state>-<direction>.png` frame strips: walk, log, sack, spearman, woodcutter, builder |

### UI chrome and icons

The skin's UI chrome and the icons fall under the same rights terms. Sources: `art/pixel-spike/ui/` —
element kits (`ui-sample.png`, `kit-a.png`, `kit-b.png`), background `bg/`, all icon candidates
`icons/<name>/NN.png` and the picks in `icon-picks.txt` (progress notes: `PROGRESS.md`). Export:
`npm run assets:pixel-ui` (`scripts/pixel/ui.py`) → `src/assets/pixel/ui/`; used by `src/ui/pixelSkin.css` (only
`html.pixel`, set in `game.html`) and `src/ui/pixelIcons.tsx`. Tests: `tests/pixelUi.test.ts`, `tests/pixelIcons.test.tsx`.

| Directory | Contents |
|---|---|
| `chrome/` | panel, header bar and plain plank, normal and red button, tab and active tab, text field, picture frame, active icon button, slider, background leather tile |
| `icons/` | 32×32: resources and population, 11 navigation icons, 12 units, and 18 interface signs (lock, warning, attack, shield, return, cart, lightning, coin, hourglass, clock, hammer, running figure, dust, magnifier, closed eye, skull, laurel, star) |

### Account site (public)

The login and registration page shows only our pixel art and OFL fonts — no game code
(`tests/accountBundle.test.ts`). `src/assets/pixel/account/`:

| File | Contents |
|---|---|
| `hero.png` | a landscape strip from the map preview (`art/pixel-spike/map/variety.sh hero-a.png --origin 7x176 --tiles 10x5`), 128 colours |
| `shot-village.webp`, `shot-map.webp` | game screenshots in the pixel skin (the settlement at night, the map) |

#### Video and screenshots recorded from the game

The home page video and the screenshots are not drawn — scripts record them from the running game (a guest clicks
"Play now" in headless Chrome). Recording needs a running server and a local `puppeteer-core` with Chrome
(`PUPPETEER_CORE`, `CHROME`) and `ffmpeg` (`FFMPEG`); none of these is a project dependency.

| File | Script | Contents |
|---|---|---|
| `clip.mp4`, `clip.webm`, `clip-poster.webp` | `node scripts/record/clip.mjs --url <url>` | 10 s: the settlement under construction, then the map with a marching army |
| `feature-village.webp`, `feature-army.webp`, `feature-reports.webp`, `feature-incoming.webp` | `node scripts/record/shots.mjs --url <url>` | game features on the home page: build queue, rally point on a phone, battle report, incoming attacks |
| `guides/*.webp` | `node scripts/record/shots.mjs --url <url>` | guide illustrations; two guests in one game, so the attack, the field battle and the reports are real |

### Map terrain

The map terrain falls under the same rights terms. Sources: `art/pixel-spike/map/` —
Wang sets `tilesets/<name>/` (`sheet.png` + `meta.json`), all 16 peak candidates `peaks/NN.png`,
the `preview.py` preview and the progress notes `PROGRESS.md`. Trees are the village sprites. Export:
`npm run assets:pixel-map` (`scripts/pixel/map.py`) → `src/assets/pixel/map/`; used by `src/ui/map/TerrainLayer.tsx`
(biome field `biomes.ts`, tiles and sprites `terrainPaint.ts`) and `src/ui/map/PixelVillage.tsx`. Village candidates:
`art/pixel-spike/map/villages/tier-<n>/`. Tests: `tests/pixelMap.test.ts`, `tests/biomes.test.ts`,
`tests/terrainPaint.test.ts`, `tests/terrainLayer.test.tsx`, `tests/pixelVillage.test.tsx`.

| File | Contents |
|---|---|
| `wang-water.png`, `wang-forest.png`, `wang-mountain.png` | 4×4 atlases of 32 px tiles (lake with a beach, forest, rock), mask tile m at (m % 4, m / 4) |
| `tree-oak.png`, `tree-pine.png` | village trees ×0.5 |
| `peak-0.png` … `peak-6.png` | the picked peaks (00, 01, 04, 05, 08, 12, 13) ×1.15 |
| `village-<0–5>.png` (+ `-barb`, `-tint`, `-roof`) | a village for each points threshold (picks in `villages/picks.txt`), the barbarian variant, one with grey roofs for tinting and the roof mask |
| `banner.png`, `banner-cloth.png` | banner (bonus, active village) and the mask of its cloth |
| `villages.json` | sizes of the village and banner sprites |
| `decal-dry-<0–2>.png`, `decal-dirt-<0–1>.png` | patches of dry grass and bare earth (`decals/NN.png`, blurred into an oval, opacity 0.7) |
| `flower-<0–4>.png`, `decor-<0–11>.png` | flower clumps and meadow decorations: bushes, boulders, stumps, a log, haystacks (`decor/NN.png`) |

### Armies on the map

Marching armies on the map fall under the same rights terms. Sources: `art/pixel-spike/`. Export:
`npm run assets:pixel-army` (`scripts/pixel/army.py`) → `src/assets/pixel/army/`; a piece missing from `army.json`
is drawn as a stand-in.

| File | Contents |
|---|---|
| `<unit>-<direction>.png` | frame strips of the 12 marching units, four directions each (`south`, `east`, `north`, `west`) |
| `torch-<direction>.png`, `shield-<direction>.png` | generic torch and shield groups |
| `boat-<direction>.png` | an army crossing water |
| `ground-<meadow|forest|mountain|water>.png` | ground backdrops |
| `army.json` | sizes, frame counts and anchor points of the exported pieces |

## Sound

The sound cues (`src/ui/sound.ts`) are generated in WebAudio — there are no audio files.
