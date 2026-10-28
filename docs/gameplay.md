# Gameplay and worlds

The kinds of worlds, their settings, the bots and the quests.

## Instant games

The game's front door: one-hour worlds on a 40×40 map, world and unit speed ×100, 12 seats for humans and 18 bots
(`game.InstantWorldConfig`). A player starts with a developed village, an army and a hetman; barbarians start at 15
loyalty, so the first conquest is a few minutes away. Players join during the first 10 minutes. The game ends after an
hour, or earlier when one human holds 60% of the villages.

- **Instant rules**: no coins, a hetman costs 18k of each resource, an academy holds its level + 1 hetmans, and a
  hetman takes 50–60 loyalty from a village with an owner, so two in a row take it. The last village can be lost.
- A **guest** is an account without a password (at most `LH_GUESTS_PER_HOUR` per hour from one address, 10 by
  default); they can create a full account at any time (`/auth/claim`) and keep playing the same village. Guests
  absent for 30 days are deleted.
- The **server** runs at most `LH_INSTANT_MAX_WORLDS` playing worlds at once (4 by default; when all are full, a
  player is told how long until one ends) and always keeps one ready, paused until the first player arrives.
  Everything lives in the database, so a restart loses nothing.
- **After the end** the world becomes read-only and its result goes to the archive (`instant_archive`,
  `instant_leaderboard`). After `LH_INSTANT_KEEP_MIN` minutes (24 h by default) the world is deleted; the archive
  stays and feeds the home page statistics (shown from 20 games), player profiles `/players/<name>` and game results
  `/games/<id>`.
- **Action log**: every accepted order, join, leave, admin change, bot move and a periodic snapshot of each human's
  state (every 5 minutes in instant worlds) goes to `action_log` in the same transaction as the change, under a random
  player key (never the name). Rejected orders are only counted (`action_rejections`). A world being deleted packs its
  log into one gzip blob (`action_log_packed`): a one-hour game with 18 bots is about 14k entries and 135 KB.
  `app.Replay` rebuilds the world from the log, the seed and the world's secret.
- **Export** for offline work (admin only, no public API):
  `go run ./cmd/export -db "$LH_DATABASE_URL" -out ./export` writes `archive.jsonl`, `leaderboard.jsonl`,
  `logs/index.jsonl` and `logs/<world>.jsonl.gz`. The index holds each world's secret, which decides its luck and
  bots: keep an export of a running world private.

## Player worlds

A registered player can host a private short game for friends: "Host a game" in the lobby, with the same settings as an
instant game (length 30–120 min in 15-minute steps, seats, bots, world and unit speed ×50–200, built-up or fresh
starting village, few/normal/many barbarians, bot difficulty and stance, field battles, sight range 0–10, protection
0–15 min, early win 50–90% or off, joining up to 5–60 min after the start or only before it, registered players only).
The map is sized to the company (20×20 to 40×40). The world plays by instant-game rules but never shows up in any
public list, statistics, events or the sitemap.

- **Admin limits** (admin panel, "Player worlds"): maximum seats (2–30) and bots (0–30) per world, 10 and 5 by
  default; they apply to worlds created after the change. Guests cannot host; a player hosts one world at a time
  (waiting or running).
- **Circuit breaker**: at most `LH_PLAYER_MAX_WORLDS` player worlds wait or run at once (16 by default); once that is
  reached the form says the server is full. Player worlds do not take seats from instant games.
- **Invitation**: a `/join/{code}` link (`/pl/dolacz/{code}`) with a QR code, `noindex`, never in the sitemap. The
  host can replace it (the old one stops working, players already inside stay) and admit registered players only. The
  host opening the link goes to the full-map view.
- **Flow**: the world waits, stopped at time 0, until the host presses Start (at least one player). A pause lasts at
  most 10 minutes, 30 in total; past the limit the game resumes on its own. "End now" makes the player with the most
  points the winner; on a waiting world it deletes it. A kicked player loses their villages (they turn barbarian) and
  cannot come back. A world abandoned before the start disappears after 2 hours (its log is kept). The server checks
  pauses and abandoned worlds every `LH_PLAYER_SWEEP_SEC` seconds (10 by default). Every host action goes to the log,
  so `app.Replay` rebuilds these games too.
- **Full-map view**: the host does not play; they watch the world at `/game/?world={id}&watch=1`: the whole map or the
  normal map, every army on the move with its troops, a live ranking and the host panel. No order gets through from
  there.
- **Results** have their own unguessable code: `/games/{code}` (`/pl/gry/{code}`), `noindex`; under the world number
  the game does not exist. Players see their own games on their profile (visible only to them), the host in a list in
  the lobby.
- **Map replay**: every finished short game (instant and player-hosted) has "Watch the map" on its results page (the
  whole game in 30, 60 or 120 seconds) and can download it as a square video (recorded in the browser, nothing goes to
  the server). A one-hour instant game with 18 bots is about 16k entries and 100 KB of packed timeline.
- **Measuring the circuit breaker on the server**: after moving to a VPS, repeat it on that machine:
  `npm run build:web`, then `LH_LOAD_SEARCH=1 go -C server test -tags e2e ./e2e/ -run TestPlayerWorldsLoad -timeout 90m -v`.
  Beside four full instant worlds, the test doubles the number of busy player worlds from 2 to 64 and checks that
  orders in a long-running world do not slow down; set `LH_PLAYER_MAX_WORLDS` to half the largest number that passed.

## World settings

The admin creates and configures worlds in the **Admin panel** on the account site. Some settings define the world and
are fixed at creation, the rest can change during the game: resources are first settled at the old pace, and queues
and marches in progress finish at their own times.

| Setting | Default | Range | After creation |
|---|---|---|---|
| World speed | 8× | 1–100 | editable |
| Unit speed (marching, merchants) | 1× | 0.5–10 | editable |
| Map size, barbarian density | 200 × 200, normal | 100/200/400; sparse/normal/dense | at creation only |
| Number of bots | 100 | 0–300 | at creation only |
| Bot arrival | all at once / gradually over N days (14) | 1–60 days | at creation only |
| Beginner protection | 7 days (can be turned off) | 0–30 days | editable |
| Last village | protected / can be lost | — | editable |
| Human / bot multipliers | ×1 (resources, recruitment, building) | 0.25–10 | editable |
| Battle losses, difficulty, bots towards humans | normal, normal, normal | normal/reduced; easy–brutal; four stances | editable |
| Field battles | on | on/off | editable |
| Sight range (passing armies) | 1 tile | 1–10 | at creation only |

- With "gradually", bots join at the edge of the occupied area with a basic village — a message goes to Rumours.
- Protection: a protected player cannot be attacked or scouted; a protected player can only attack barbarians.
- Last village "protected": nobles lower loyalty to 1 at most. "Can be lost": the player is out, and a human starts
  over at the edge of the world (without coins).
- The admin can pause a world: world time stands still and players see the banner "The world is paused by the
  administrator".

## Bot players

The world is populated by bots that play like humans (`server/internal/game/bot_*.go`, constants in
`server/internal/game/data_bots.go` and `data_playbook.go` — that is the only place to tune them).

- **Wall clock.** Every bot has its own daily rhythm (6–9 h of sleep, sessions, a "glance at the phone" before a hit)
  computed from local time (`World.ClockOrigin`), so changing the world speed does not shift its habits. It only
  thinks in sessions; offline it reacts only through the phone. On its first day after joining (`BotConfig.StartRush`)
  it also checks in every 1–2 h like players at a server launch — almost always if engaged, rarely if casual.
- **Skill and style.** `skill` (0–1) decides how often a bot sticks to playbook moves (`Plays`) and how rarely it
  makes typical mistakes (`Mistakes`); choices go through a softmax whose temperature rises for weak bots, so moves
  are not predictable. Weak bots also log in less often (`BotConfig.WeakSkipCheckIn`). Style (farmer, turtle,
  aggressor, point builder, opportunist) weights goals and village roles. World difficulty is the minimum share of
  strong bots.
- **Economy and farming.** Building by village role, recruitment with a Farmstead reserve, raids on barbarians sized to
  the loot remembered (scouting, wall; dried-up farms drop off the list and come back after a few days).
- **Expansion.** An Academy in the best village (from three villages also in the second), coins where the world has
  them, hetmans against barbarians and the grey villages of players who left.
- **War.** Moods (build-up → campaign → consolidation), targets from public data and report memory; operations:
  takeover with a full train from several villages (clear + up to 4 hetmans, every 100 ms for strong bots and with
  fakes), demolition, farming a weaker player, harassment. Strong bots land in the target's offline window and learn
  its habits (e.g. its dodge rhythm). After a village is taken, the previous owner's troops outside it (marching,
  returning, as support) are lost, and an attack that reaches a village that is already its own (e.g. the next hetman
  of a train) stays there as support.
- **Defence.** Stacking support when it arrives in time and will hold; otherwise a dodge; strong bots snipe noble
  trains. A weak bot "doesn't notice" some attacks.
- **Quitting.** Being overwhelmed (village losses, attacks, falling behind peers) fades over time; past a threshold
  the bot quits. Its villages stay grey for `BotConfig.AbandonAfterDays` (21) days — they can be taken despite
  last-village protection — and then turn barbarian.
- **Short worlds** (instant and player-hosted): bots are always online and think every 20 s, their plans scaled to the
  hour. A bot keeps the next hetman's price instead of spending it on buildings; with a full train (two hetmans) it
  goes for scouted villages of active players its army can clear (or scouts the nearest unknown one) before
  barbarians; from three villages it leaves barbarian and grey villages to the others.
- **Bots towards you** (world setting): *peaceful — strike back only when attacked*, *peaceful — by reputation*
  (attacking one marks you for all), *normal*, *everyone against me* (your villages are the campaign target of every
  bot, with no distance limit). Farming barbarians and
  inactive players is not a provocation; a grudge expires after 5 days.
- **Your tools.** Scheduled sends ("Send at" / "Arrive at", works offline), incoming attack labels by the slowest unit,
  one-click dodge and snipe.
- **Public world.** Ranking (points, villages, defeated, daily, takeovers; points from world creation, then every full
  hour of the local clock), player profiles, player colours on the map
  (grey — inactive) and Rumours built from public facts. Bots see only what a player sees: public data and their own
  reports.

## Quests

`server/internal/game/data_quests.go` — a chain of 42 quests (`Requires` → active, a condition, an optional progress bar, `Reward`: resources / +1 queue slot / auto-farming). The server marks conditions as met and sends each active quest's bar with the player state; the player claims the reward in the **Quests** tab (resources above storage capacity are lost — the UI warns about it). Adding a quest = a new entry in the list, its texts in the catalogues and `npm run gen`; ids must be unique and requirements must point to earlier quests (a test checks this).
