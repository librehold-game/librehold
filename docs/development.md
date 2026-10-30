# Development

How to run, test and find your way around the code. Deployment: [`deploy/README.md`](../deploy/README.md).

## Running

```bash
npm install
cd server && make db          # Postgres (localhost:5433) and Valkey (localhost:6380) for development
LH_DATABASE_URL='postgres://librehold:librehold@localhost:5433/librehold?sslmode=disable' LH_COOKIE_SECURE=false \
  LH_ADMIN_USERNAME=admin LH_ADMIN_PASSWORD=adminadmin go run ./cmd/server    # API on :8080
npm run dev:account           # account site (login, registration, lobby, admin panel)
npm run dev                   # game: /game/game.html?world=<id>; /api is proxied to :8080
npm test                      # client tests (Vitest)
npm run server:test           # server tests (Go; the Postgres tests need Docker)
npm run build                 # typecheck + both builds: dist/account and dist/game
```

First steps: log in as admin and create a world in the **Admin panel**. Anyone can create a player account on the
registration page (at most 3 accounts per hour from one address, `LH_REGISTRATIONS_PER_HOUR` changes that); then join
a world from the lobby and enter the game. Without an account, **Play now** on the home page is enough: the guest gets
a nickname and a seat in an instant game.

## Server

The game server is a single Go program (`server/`) with a hexagonal layout:

- `internal/game` — the game rules (building, troops, combat, conquest, bots, quests…). It knows
  nothing about the database or the clock: it gets the world through the `game.World` interface and time as an argument.
- `internal/app` — use cases: accounts, sessions, admin panel, lobby, orders, player views, the event worker. It
  imports only the standard library and `game` (`internal/archtest` enforces this).
- `internal/adapters` — Postgres (world unit of work under an advisory lock, `LISTEN/NOTIFY` notifications), HTTP API
  (`/api/v1`), WebSocket with refresh hints, a file gate (game files only with a session), argon2id and tokens,
  Valkey counters for the limits.
- `cmd/server` — server startup; `cmd/sim` — bot simulation on the Go rules (`-instant`, `-instant-player`: instant
  game); `cmd/gen` — the generated browser files (`npm run gen`); `cmd/export` — export of the archive and action logs.

Every order and every event is one transaction: world lock, pending events in order, the change, the write, and only
then the response and notifications. After a restart the worker catches up on events in order, and the result is the
same as without the interruption.

```bash
cd server
make db        # Postgres (localhost:5433) and Valkey (localhost:6380) in Docker (make db-down stops them)
make test      # all Go tests (the Postgres tests start their own container, Docker required)
make e2e       # end-to-end tests on the built binary; first run: npm run build:web (from the root directory)
make perf      # performance tests (order latency, event lateness, catch-up, load beside long worlds), about 15 min
               # in real time; run locally only, CI never runs them
```

Server environment variables:

| variable | default | meaning |
|---|---|---|
| `LH_DATABASE_URL` | — (required) | Postgres connection |
| `LH_HTTP_ADDR` | `:8080` | listen address |
| `LH_WEB_DIR` | `/app/web` | directory with the `account/` and `game/` builds |
| `LH_COOKIE_SECURE` | `true` | session cookie over HTTPS only (`false` locally) |
| `LH_TRUSTED_PROXIES` | — | addresses or CIDR ranges of reverse proxies whose `X-Forwarded-For` names the client |
| `LH_VALKEY_URL` | — | Valkey for the limits (`redis://:password@host:6379/0`), shared by every server process; empty counts them in the one process |
| `LH_ADMIN_USERNAME`, `LH_ADMIN_PASSWORD` | — | admin account created on first start |
| `LH_REGISTRATIONS_PER_HOUR` | 3 | how many new accounts one address can create per hour |
| `LH_GUESTS_PER_HOUR` | 10 | how many guest accounts one address can create per hour |
| `LH_WORLD_REQUESTS_PER_MINUTE` | 300 | how many state reads and orders one account can send per minute, over all its worlds |
| `LH_INSTANT_MAX_WORLDS` | 4 | how many instant games can run at once |
| `LH_INSTANT_KEEP_MIN` | 1440 | how many minutes a finished instant game's result stays in the game before the world is deleted |
| `LH_INSTANT_SWEEP_SEC` | 60 | how often, in seconds, the server checks on instant games (ready world, archive, cleanup) |
| `LH_PLAYER_MAX_WORLDS` | 16 | how many player worlds can wait for the start or run at once; once reached, the form says the server is full |
| `LH_PLAYER_SWEEP_SEC` | 10 | how often, in seconds, the server ends host pauses past the limit and deletes worlds abandoned before the start |
| `LH_PUBLIC_URL` | — | site origin (`https://example.com`) for canonical links, previews and the sitemap; empty uses the request's address |
| `LH_TIME_SCALE`, `LH_TIME_EPOCH_MS` | 1 | **tests only**: server clock N times faster from the given moment |

Deployment (image, Caddy with HTTPS, nightly database backups, restore): [`deploy/README.md`](../deploy/README.md).

## Architecture

- `server/` — the Go game server (hexagonal): `internal/game` holds every rule and every balance value,
  `internal/app` the use cases, `internal/adapters` Postgres, the HTTP API, WebSocket and the file gate. The database is
  the single source of truth: every order and every event is one transaction under the world lock.
- `server/internal/gen` — the files the browser and its tests read from the Go rules: `src/data/balance.json` (the
  tables), `tests/fixtures/rules.json` (Go's answers for the browser's display formulas) and `tests/fixtures/world-*.json`
  (worlds the server built, for the UI tests). `npm run gen` writes them; `go test ./...` fails while they are stale.
- `src/engine/` — display helpers only: costs, march times, incoming attack labels, resources growing between server
  replies. `tests/rules.test.ts` holds them to the server's answers.
- `src/data/` — typed readers of the generated `balance.json`; no value is written there by hand.
- `src/net/` — API client, live connection ("what changed" hints), the map tile cache, the conversion of server
  replies into `GameState` (`toGameState.ts`) and bug reports.
- `src/state/` — Zustand store (+immer): player state from the server in the shape of the engine's `GameState`, orders
  through the API, a 250 ms clock for display only.
- `src/account/` — the separate account site: public pages, login, lobby, admin panel (no game code; it shares
  only the pixel UI chrome and the fonts with the game).
- `src/shared/` — code both builds use: terrain noise and biomes, report fields.
- `src/i18n/` — translations (`catalog/en`, `catalog/pl`) and number, date and time formatting.
- `src/ui/` — React, Tailwind v4 (tokens in `src/index.css`), pixel art skin in `src/ui/pixelSkin.css`.
- `tests/` — Vitest.

### Time model

`state.now` is world time in ms since the world was created; it runs 1:1 with real time (except while the admin or the host has the
world paused), and the server computes it — the client only adds the time since the last response. The UI shows it as
local time (`state.clock.originLocalMs + now`): the server clock "09:14:05 19.10.2026" labelled "Day N", moments
relative to today's date — "today at 14:05:30", "tomorrow at …", "on 21.10. at …" (`src/i18n/format.ts`, the `useClock`
hook); sends are scheduled by date and time, and the time of day in the village follows the same clock (the server's
Warsaw time).
Resources are not accrued incrementally — `accrue(v, now)` computes the amount as a function of
the last save and `now`, so 8 h offline and 8 h online give an identical result. Events (buildings, commands, the
trader, bot sessions, scheduled sends…) are handled by the server: every world has a worker that pops events in
`(time, sequence)` order when their time comes, and after a restart catches up on the backlog — the result is the same,
because every rule computes from its own event's time. `gametest.Advance` plays a world forward the same way for the Go
tests and `cmd/sim`.

## Scripts and dev mode

- `npm run gen` — regenerates `src/data/balance.json` and `tests/fixtures/*` from the server; `go test ./...` fails while
  they are stale. Run it after changing a table or a rule the browser displays.
- `cd server && go run ./cmd/sim -player [-seed N] [-days N] [-speed X]` — the reference player's run: the points curve
  per day and the milestones (see [Pace and early game](balance.md#pace-and-early-game)).
- `cd server && go run ./cmd/sim -bots 100 -days 60 -seed 7 [-difficulty easy|normal|hard|brutal]` — a bots-only world (the player does nothing); every 5 days:
  active / quit / eliminated, median villages, conquests, points of strong and weak bots, raids on barbarians.
  `TestBotWorldStaysInBands` holds 30 bots over 30 days on seeds 1–3 to the recorded bands; after a deliberate bot change re-record
  them with `go test ./internal/game/ -run Bands -update`.
- `npm run coverage` — test coverage (text report per file).
- `?dev=1` — exposes the store as `window.__lh` on the console (debugging, e2e tests). Nothing else: time and
  resources cannot be changed from the browser.

## Releasing

Versions follow [Semantic Versioning](https://semver.org/): a patch release for fixes only, a minor one when anything
is added or changed (and, before 1.0, for breaking changes too). Only org admins can push `v*` tags. From a clean
`main`:

```sh
scripts/release.sh 0.4.0       # rewrites CHANGELOG.md with git-cliff, up to 0.4.0, commits and tags it
git push origin main v0.4.0    # after reading CHANGELOG.md
```

The tag starts `.github/workflows/release.yml`: it publishes `ghcr.io/librehold-game/librehold` as `0.4.0`, `0.4`
and `latest`, then creates the GitHub release with the same notes. `cliff.toml` sorts merged pull requests by label
(`enhancement` or `idea` → Added, `bug` → Fixed, `security` → Security, anything else → Changed) and credits their
authors; `GITHUB_TOKEN=$(gh auth token) git cliff --unreleased` previews the next release.

## Game state

The state of every world lives in Postgres on the server — the browser stores nothing except UI preferences,
the language, the last selected village and a few one-time hints. Database backups are made by the `backup` service (nightly, 7 days back,
`deploy/README.md`).
