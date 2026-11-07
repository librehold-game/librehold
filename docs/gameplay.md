# Gameplay and worlds

The kinds of worlds, their settings, the bots and the quests.

## Instant games

The game's front door: one-hour worlds on a 40×40 map, world and unit speed ×100, 12 seats for humans and 18 bots
(`game.InstantWorldConfig`). A player starts with a developed village, an army and a hetman; barbarians start at 15
loyalty, so the first conquest is a few minutes away. Players can join at any time until the game ends, as long as it
has a free seat, so a late joiner starts with the same village as everyone else. The game ends after an hour, or earlier
when one human holds 60% of the villages.

- **Instant rules**: no coins, a hetman costs 18k of each resource, an academy holds its level + 1 hetmans, and a
  hetman takes 50–60 loyalty from a village with an owner, so two in a row take it. The last village can be lost.
- A **guest** is an account without a password (at most `LH_GUESTS_PER_HOUR` per hour from one address, 10 by
  default); they can create a full account at any time (`/auth/claim`) and keep playing the same village. Guests
  absent for 30 days are deleted.
- **Choosing a game**: Play now shows the running games (`GET /api/v1/instant/games`): name, time left, players,
  the leader and a Join button while a seat is free. `POST /api/v1/instant/play` with `{"worldId": N}` joins that game;
  without it, it continues the player's own game or starts a new one. A player is in one running instant game at a
  time (leave it with `POST /api/v1/worlds/{wid}/leave` to join another). When nothing is running, Play now starts a
  game straight away.
- The **server** runs at most `LH_INSTANT_MAX_WORLDS` playing worlds at once (4 by default; when all are full, a
  player is told how long until one ends) and always keeps one ready, paused until the first player arrives.
- **Abandoned games**: a request from a human in the game, or a live connection, counts as presence (stored in
  `worlds.last_present_ms`, so every server process sees it). A running instant game with no presence for 5 minutes
  ends early with the reason `abandoned`, is archived like any other and frees its place under the cap. The ready
  world and paused worlds are never ended this way.
  Everything lives in the database, so a restart loses nothing.
- **After the end** the world becomes read-only and its result goes to the archive (`instant_archive`,
  `instant_leaderboard`). After `LH_INSTANT_KEEP_MIN` minutes (24 h by default) the world is deleted; the archive
  stays and feeds the home page statistics (shown from 20 games), player profiles `/players/<name>` and game results
  `/games/<id>`.
- **Action log**: every accepted order, join, leave, admin change, bot move and a periodic snapshot of each human's
  state (every 5 minutes in instant worlds) goes to `action_log` in the same transaction as the change, under a random
  player key (never the name). With `LH_RECORD_DECISIONS=true` (off by default) each human order is preceded by an
  `observe` entry: what the player had just before it (villages with resources, buildings, research, queues, troops,
  armies leaving and coming, the farm assistant's settings and the wall time). An order sent from a village at a tile
  also carries the choice: the target and the nearest 20 farms of the village's farm list, as the player's reports
  know them. Rejected orders are only counted (`action_rejections`). A world being deleted packs its
  log into one gzip blob (`action_log_packed`): a one-hour game with 18 bots is about 14k entries and 135 KB.
  `app.Replay` rebuilds the world from the log, the seed and the world's secret.
- **Export** for offline work (admin only, no public API):
  `go run ./cmd/export -db "$LH_DATABASE_URL" -out ./export` writes `archive.jsonl`, `leaderboard.jsonl`,
  `training.jsonl` (one human decision per line: what the player had, then the order), `logs/index.jsonl` and
  `logs/<world>.jsonl.gz`. The index holds each world's secret, which decides its luck and
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
  `npm run build:web`, then `LH_LOAD_SEARCH=1 go -C server test -tags 'e2e perf' ./e2e/ -run TestPerfPlayerWorldsLoad -timeout 90m -v`.
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
| Trading (players and bots alike) | on for a new world, **off for a world stored before the setting existed** | on/off (`TradingEnabled`) | editable — closes player-to-player trading: posting, accepting and sending resources to another player, plus the bots' own deals; moving resources between your own villages and the travelling trader stay open, as does withdrawing |
| Trade offer duration | 24 game hours | admin-set hours | editable — applies to offers posted after the change; a world stored before this setting keeps the 24 h default until an admin picks a value |

- With "gradually", bots join at the edge of the occupied area with a basic village — a message goes to Rumours.
- Protection: a protected player cannot be attacked or scouted; a protected player can only attack barbarians.
- Last village "protected": nobles lower loyalty to 1 at most. "Can be lost": the player is out, and a human starts
  over at the edge of the world (without coins).
- The admin can pause a world: world time stands still and players see the banner "The world is paused by the
  administrator".

## Player trading

Trading is a **world setting** (`WorldConfig.TradingEnabled`), editable at any time. It governs deals *between
players*: posting and accepting offers, sending resources to somebody else's village, and the bots' own trading (a bot
checks the flag before it plans any deal). Those paths refuse with a `tradingOff` blocker. What it does not close is
anything that is not a deal with another player: moving resources between your own villages (`CheckTrade` and
`SendTrade` allow it when both villages share an owner), the travelling trader's fixed-rate exchange, and withdrawing an
offer, so nobody is stuck with resources set aside. A world with trading off therefore keeps the market it had before
player trading existed. New worlds start with it **on**. A
world stored before the setting existed has no `tradingEnabled` key in its config; the flag is therefore stored as
`TradingEnabled` rather than `NoTrading`, so the zero value means **off** and no running world changes behaviour. An
admin flips it per world, both in the admin world form and in the player hosting form (where it defaults to on, so an
omitted field keeps it). It is part of `PublicSettings`, so the world page shows on/off before anyone joins — the
field carries no `omitempty`, since an absent key could not be told apart from a world that has it off. The world
page prints the offer duration as its own row, only while trading is on. Switching it off hides the two offer panels
and the travelling trader outright — only the merchant line stays, since it still explains the village's capacity —
and the server refuses posting, accepting and sending. The switch also **closes the market**: every open offer is taken
down and its reserved share refunded (`game.CloseMarket`). Each share returns to the village that reserved it and is
**capped at that village's storehouse**, exactly like any other refund: a village already above capacity, or one whose
storehouse was
demolished while the offer was up, keeps what fits and the excess is lost. That is what makes it safe to flip on a
live world, because the panels are gone while it is off and a stranded offer would hold its resources where no player
could reach them.

Players trade resources with each other through **trade offers** (engine: `game.TradeOffer`, orders `create-trade-offer`,
`accept-trade-offer`, `cancel-trade-offer`). An offer is posted from one of the player's villages and has two sides: what the
poster's village gives, and what the taker brings back.

- Every offer sits on the **public board** and is visible to every player whose village the post can reach inside its delivery
  limit (1–24 merchant hours, or unlimited). There are no targeted offers: a player who wants to trade with one particular
  neighbour simply posts the deal that neighbour needs.
- An offer waits **24 game hours** before it lapses (`TradeBalance.OfferExpiryMs`), so at higher world speeds it lasts less real
  time. A village may keep `PublicPerVillage` (6) *different* deals out, so a player's cap is that times their village count;
  **copies of a deal already on the board are free of that cap**, so one deal can be split into many copies and several takers
  can each pick up a piece. There is no world-wide limit on the board. Posting reserves the give side on the spot, so the
  village must hold it.
- Storage: one migration, `0017_trade_offers.sql`, purely additive — the table plus one index and nothing else. The
  trade settings live in the existing `worlds.config` jsonb and the settlement marker in `commands.data`, so no other
  schema changes and no backfill. Migrations run at boot (`postgres.Migrate` in `cmd/server/main.go`), so an existing
  database upgrades itself. Its Down refunds the reserved give side of every open offer back to its origin village
  before dropping the table, so a rollback returns resources rather than destroying them.
- Accepting settles the deal as **two market shipments**: each side loads free merchants with what it owes, pays its own merchant
  fee, and the arrival works like any trade (storage capping, merchants return). Both villages need a level 1 market and must be
  able to ship at the moment of acceptance. Both legs are marked `Command.Settled`, so **neither can be recalled** — the offer is
  already consumed and the other side has shipped against it, and allowing a recall would let either party take the goods *and*
  the payment. A plain market trade keeps its 5-minute cancel window; only settlement legs are frozen.
- **Merchants are the hard limit on trade size**, and the market's level decides how many you have: one per level up to
  **11**, then quadratic, `(L − 10)² + 10` — the same curve Tribal Wars uses (`TradeBalance.MerchantCurveAt`,
  `MerchantsForLevel`). A level 12 market gives 14, level 15 gives 35, level 19 gives 91 and a maxed level 25 gives 235. One
  merchant carries `CommandBalance.MerchantCapacity` (1000) resources, so a maxed market moves 235 000 per shipment — more
  than a level 25 storehouse holds of one resource — while a level 10 market moves 10 000. The quadratic step is there
  because a building's cost grows exponentially: without it the top Market levels would buy a single extra merchant each
  and nobody would ever build them.

  A deal needing more merchants than the market will *ever* have is refused as `marketTooSmall`; one needing more than are
  *free right now* (because some are already on the road) is refused as `merchants`. The market screen raises the reason
  in the error toast when the player presses Accept, so they know whether to upgrade the Market or simply wait.
- Identical offers — same poster, same delivery time, same amounts — collapse into **one board row** with an "available" count.
  A taker can pick up several copies at once: `accept-trade-offer` takes `offerIds` (a batch). The whole batch is checked against
  the totals before anything moves, so the taker must hold every copy's cost and have merchants for the combined haul; the batch
  then books **one haul each way** and consumes every copy (`AcceptTradeOffers`).
- Visibility is per viewer: their own posts, and posts that can deliver to one of their villages — newest first
  (`VisibleOffers`). Settlement sends each human side a news item and a market report.
- A merchant shipment is an ordinary `trade` command, so it shows up wherever commands do: the **Village** view's
  Movements list (under the building table, only while something is on the road) and **Rally Point → Commands**. Both read
  the haul from the seller's village to the buyer's, with each resource and its amount, the merchant count and an arrival
  countdown; a shipment another player sent to you is included and highlighted, because the state carries commands aimed
  at your villages as well as your own. Marches show the troops they carry the same way.

The market screen shows board and own offers with accept/withdraw actions and the post form. Both offer lists group
identical deals (same sender, same delivery, same amounts) into one row with a count; on your own row a withdraw-count
box takes back that many copies at a time (`cancel-trade-offer`, once per copy). The global board is
a filterable table (player search, player or bot poster, the resource it offers and the resource it wants, max delivery
time, minimum ratio, sort) paged ten rows at a time, with a take-N input per row. Bots trade too (`bot_trade.go`): a bot
short of one resource posts a public offer for it priced by how badly it needs it — a ratio of 1.15 while it is only
mildly lopsided, rising to 1.40 as the missing resource runs out — and a bot takes any board post that gives what it
lacks and asks for something it can spare, checked against its free merchants and its batch affordability. A bot also
**withdraws its own post** (`CancelTradeOffer`) as soon as the situation behind it changed — it is now short of what it
was offering, it no longer needs what it asked for, or a different resource became the one worth trading away. Everything
goes through the ordinary `CreateTradeOffer` / `AcceptTradeOffers` / `CancelTradeOffer` rules, so bots trade with bots and
with humans on the same terms.

## Bot players

The world is populated by bots that play like humans (`server/internal/game/bot_*.go`, constants in
`server/internal/game/data_bots.go` and `data_playbook.go` — that is the only place to tune them).

- **Wall clock.** Every bot has its own daily rhythm (6–9 h of sleep, sessions, a "glance at the phone" before a hit)
  computed from local time (`World.ClockOrigin`), so changing the world speed does not shift its habits. It only
  thinks in sessions; offline it reacts only through the phone. On its first day after joining (`BotConfig.StartRush`,
  longer for strong and medium bots) it also checks in every 1–2 h like players at a server launch — almost always if engaged, rarely if casual.
- **Skill and style.** `skill` (0–1) decides how often a bot sticks to playbook moves (`Plays`) and how rarely it
  makes typical mistakes (`Mistakes`); choices go through a softmax whose temperature rises for weak bots, so moves
  are not predictable. Weak bots also log in less often (`BotConfig.WeakSkipCheckIn`). Style (farmer, turtle,
  aggressor, point builder, opportunist) weights goals and village roles. World difficulty is the minimum share of
  strong bots.
- **Economy and farming.** Building by village role, recruitment with a Farmstead reserve, raids on barbarians sized to
  the loot remembered (scouting, wall; dried-up farms drop off the list and come back after a few days).
- **Tiers on normal worlds** (`EconomyPlays`). Strong bots (skill ≥ 0.7)
  open with the mines, keep iron 3 levels behind wood and clay and build only the economy until the mines reach 18;
  they grow the Farmstead when less than a quarter of it is free and the storage while it holds less than 12 hours
  of income (production plus recent loot), never leave the build queue empty for want of one building, leave a
  24-step build plan for the break, keep the hiding place at 3 and the wall at 5 unless farmed hard, and check in
  often for 4 days after joining. Farming styles (farmer, aggressor, opportunist) build barracks 5, smithy 5 and
  stable 3 from mines 8, research light cavalry first and recruit before building until their troops fill 55 % of
  the Farmstead; the other styles do so from mines 14 with 10 %. Medium bots (0.4–0.7) do the same with the
  military from mines 12–15, 30 % of troops, a 12-step plan and 2 days of check-ins. Strong and medium bots use the
  auto-farm as humans do, once unlocked the same way (5000 looted or 3 villages): up to 20 sized raids per village
  every 15 minutes, online or not, at barbarian and abandoned villages only. Weak bots and every bot in a short world play the classic way.
- **Trading.** Bots trade like humans (`bot_trade.go`, run before the economy planners each think). A village with a
  market that is lopsided — it holds less than half of its best resource — posts a public offer for its scarcest one.
  The rate it pays (`botTradeRate`) climbs with need: 1.15 units of spare per unit wanted just past that threshold, up
  to 1.40 when the resource is nearly gone, so a starving village bids harder than a comfortable one; the reserved
  share comes out of the village like a human's. A bot takes any board post that gives a resource it is comparably
  short of and asks for one it can spare, paying no more than `botTradeMaxCost` per unit received and checked against
  its free merchants, batch affordability included. It keeps one post per village, and **withdraws** it
  (`CancelTradeOffer`) once the situation behind it changed: it is now shortest of what it was offering, it no longer
  needs what it asked for, or the village became balanced. On instant worlds only a share of the bots trade
  (`BotConfig.ShortTradeChance`), where every minute counts. The board itself is a map, so its iteration order varies
  between runs: the planner reads it through `boardOffers`, which sorts by creation time and then id, or two runs of the
  same world would take different offers and a replayed log would not match.
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
- **Quitting.** Being overwhelmed (village losses, attacks) fades over time; past a threshold a medium or strong
  bot quits (weak bots always play). Its villages stay grey for `BotConfig.AbandonAfterDays` (21) days — they can be
  taken despite last-village protection — and then turn barbarian.
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
