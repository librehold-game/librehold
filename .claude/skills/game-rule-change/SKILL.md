---
name: game-rule-change
description: Use when changing how LibreHold plays - a game rule, a balance number or table, combat, conquest, support, trading, scouting, reports, intel or bot behaviour in server/internal/game.
---

# Changing a game rule

Rules live only in `server/internal/game`; the browser shows what the server decides. Rule changes that come back
from review usually missed a player who also sees the change, or a moment when it meets another rule. Agree on a rule
or balance change in the issue before writing it.

## Steps

1. **Find every path into the rule.** `git grep` the function and the fields it touches. Bots, the auto-farm,
   scheduled sends and the farm assistant often reach the same code as a player's order.
2. **Write failing tests** in `server/internal/game/*_test.go`:
   - start from `gametest.NewTestWorld(seed)` (a human with a village) or `gametest.NewWorldWithHuman(seed, cfg)`
   - add others with `gametest.NewHumanPlayer`, `gametest.NewBotPlayer`, `gametest.AddStartVillage`,
     `gametest.MakeBotVillage`
   - move time with `gametest.Advance(w, until)`; never sleep or read the clock
   - one test for each row of the checklist below that applies
3. **Change the rule.** Time comes from `w.Now()`, randomness from the seeded helpers in `rng.go`.
4. **Check what each player sees** (section below), then the browser: `src/engine` may have a display helper for the
   same numbers, held to the server's answers by `tests/rules.test.ts`.
5. **Regenerate and run.** `npm run gen` if a table or rule the browser shows changed, then
   `cd server && go test ./internal/game/...`. For a deliberate bot change, re-record the bands with
   `go test ./internal/game/ -run Bands -update` and say so in the PR.
6. **Update what players read:** `src/i18n/catalog/{en,pl}/guides.json`, the encyclopedia and FAQ,
   `docs/gameplay.md` or `docs/balance.md`, and code comments the change makes wrong.

## Checklist

| Case | Check |
|---|---|
| Who reads it | Owner, attacker, defender, supporters, other players, bots. Each sees their own side and nobody gets more than their view allows. |
| A village changes hands | Conquest or abandonment while something is tied to it: armies on the way (cut loose), support stationed in it or sent from it, market offers, build plans, queues, merchants. |
| Other players | Support can go to any player's village except a barbarian one. The browser has only the player's own villages in full, so it can't add up what is stationed with others. |
| Bots | They go through the same rules (`CanBeConquered`, `PopulationUsed`, ...). Test with a bot on each side. |
| World kinds | Instant, hosted and admin worlds; settings switched mid-game (`world-setting`). |
| Speed | World speed and unit speed scale different things, and some durations aren't scaled at all. |
| Abuse | Can it hide resources or troops from raids (the Hideout's job), move them between accounts (guest accounts cost nothing), win from bots by trading back and forth, or make something from nothing? Test that a round trip never ends with more than it started with. |
| Many at once | An auto-farm burst lands many raids in the same moment. Events run in (time, sequence) order and reports sort by id, so never rely on times being different. |

## What each player sees

- `PlayerState` in `server/internal/app/views.go` holds the player's own villages in full, and other players'
  commands only when they are coming at the player, without the troops (`foreignCommand`). Never add something a
  player may not see, even if the UI would hide it.
- Reports go to `reportRecipients` in `battle.go`, and each reader gets their own version.
- Intel (`w.PutIntel`) and what bots learn reveal no more than the report did.
- A change that affects another player has to reach them. `Changes.Updates` in `server/internal/app/updates.go`
  decides who gets a "state" update; a report alone only refreshes the inbox.

## Performance

On Postgres, `w.CommandsHome`, `w.SupportHosts` and `w.VillagesIn` are one SQL query per call. Code that runs for
every village (bots, population) must not loop over them. The in-memory tests won't show the cost.
