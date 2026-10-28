# Balance

Where the numbers live and how the pace of a world is checked.

## Balance

Key rules: per-resource cost multipliers, the build time formula
(`base·1.18·1.2^(L−1−14/(L−1))·1.05^−HQ`), building points (Town Hall 10, Barracks 16… ≈ 12,380 in total), hetman
40k/50k/50k, the n-th hetman = `villages + nobles owned` coins (coin fixed at 28k/30k/25k), research +25%/+40%, base
defence only from the wall (20·L), ~220 rams for wall 20, ~390 catapults for Town Hall 20→1, hideout
`150·1.3335^(L−1)`, command cancel within 10 min, merchant 6 min/field, hetman range 70 fields.

## Balance formulas (where to change what)

Every value lives in `server/internal/game`; after changing one, run `npm run gen` so the browser's copy follows.

| Element | Where |
|---|---|
| Barbarian villages | `data_world.go` → `BarbarianTemplateData` — building levels drawn from ranges (≈ 40–70 pts), no wall, no troops, no rebuilding, evenly across the whole map |
| Quest rewards | `data_quests.go` |
| Points history | `stats.go` → `StatsBalance` |

| What | Formula | File |
|---|---|---|
| Resource production | `5 + round(30 · 1.163118^(L−1))` per hour × `worldSpeed` | `ResourceBalance` in `data_buildings.go` |
| Storehouse capacity | `round(1000 · 1.2294^(L−1))` | same |
| Farmstead limit | `round(240 · 1.1722^(L−1))` | same |
| Hideout | `round(150 · 1.3335^(L−1))` | same |
| Building cost | `round(base[r] · factor[r]^(L−1))` | `BuildingDefs` in `data_buildings.go` |
| Building population | `round(base_pop · popFactor^(L−1))` (total at the level) | `BuildingDefs[*].PopFactor` |
| Points | `round(base_pts · 1.2^(L−1))` (total building value) | `BuildingBalance.PointsFactor`, `BasePoints` |
| Build time | `base · 1.18 · 1.2^max(−13, L−1−14/(L−1)) · 1.05^−townHall / worldSpeed`, min 1 s | `BuildingBalance`, `BaseTimeSec` |
| Combat | luck ±25%, wall `1.037^L` + `20·L`, losses `(weaker/stronger)^1.5`, wall hp `0.27·L^1.5+0.7`, building hp `0.39·L^1.5+0.8` | `CombatBalance` in `data_units.go` |
| Conquest | loyalty +1/h, drop 20–35, coin 28k/30k/25k, coins per hetman = villages + hetmans | `ConquestBalance` in `data_conquest.go` |
| Units | stats table, recruitment time `base · 0.9666^(level−1)` | `data_units.go` |
| World | speeds, map size, queue slots, player start | `data_world.go` |
| Instant worlds | no coins, hetman 18k/18k/18k, academy holds level+1 hetmans, owned villages lose 50–60 loyalty per hetman, barbarians start at 15 | `InstantWorldConfig` in `instant.go` |

## Pace and early game

The pace is **deliberately fast** — this is a clicker, not a game that lasts half a year. Time is sped up, not the
rules of units, buildings and combat.

- New worlds start at **8× speed**: production ×8, building and recruitment ÷8, loyalty recovers 8× faster — **marching and merchants are not sped up** (army pace is set separately by *unit speed*). Settings let you go down to 1×.
- Side multipliers (*Your multipliers* / *Bot multipliers*): resources, recruitment, building — separately for you and for bots (0.25–10), barbarians always ×1. Research ignores the multiplier.
- The starting village has **10 spearmen** — the first raid on an empty barbarian village goes out in the first minutes.
- **Barbarians are passive**: small villages without a wall or troops; damage and looted resources do not come back on their own.
- Auto-farming unlocks as the reward for the "Raider" quest (5,000 loot) or at 3 villages.

Measured by the reference player (seed 1, default settings, 8×, marching not sped up):

| Milestone | Target | TypeScript, 2026-10-18 | Go, measured 2026-10-28 |
|---|---|---|---|
| first raid | ~30 min | 1 h | **1 h** |
| 1,000 pts | ~2 days | 3.4 d | **3.4 d** |
| first hetman | 10–14 days | 12.1 d | **12.4 d** |
| 2nd village | — | 12.6 d | **12.8 d** |
| 10th village | 5–6 weeks | 19.5 d | **19.3 d** |

The pacing test thresholds are the TypeScript measurement × 1.2, rounded up: first raid 2 h, 1,000 pts 5 d, first
hetman 15 d, 2nd village within 3 d of it, 10th village 24 d.

Measurement: `cd server && go run ./cmd/sim -player [-seed N] [-days N] [-speed X]`. The reference player
(`server/internal/game/gametest/reference_player.go`) is a scripted player in the human seat that knows only what the
UI shows: it builds the cheapest building, starts the Academy, farms with a party sized to the known defence, mints
coins and leads conquest campaigns. `server/internal/game/pacing_test.go` enforces the thresholds on the same run, plus
a **boredom wall** detector (no three days in a row without points growth). Instant worlds have their own check:
`instant_pacing_test.go` (`TestInstantPacing`) plays five seeds of the hour with `gametest.RunInstantPlayer` and wants
3–5 villages, a top-two finish, owned villages changing hands and a first conquest within 5 min.
