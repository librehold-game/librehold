<p align="center">
  <img src="public/account/og/home.png" alt="LibreHold: pixel art villages, castles and mountains on a green map" width="720">
</p>

<h1 align="center">LibreHold</h1>

<p align="center">
  A free, open-source browser strategy game: grow a settlement from a hut to a castle, raise an army, raid and conquer.
</p>

<p align="center">
  <a href="https://librehold.org"><b>▶ Play now in your browser</b></a> — no download, no sign-up
</p>

<p align="center">
  <a href="https://librehold.org">Website</a> ·
  <a href="docs/development.md">Development</a> ·
  <a href="deploy/README.md">Self-hosting</a> ·
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-alpha%200.1-orange" alt="Status: alpha 0.1">
  <a href="https://github.com/librehold-game/librehold/actions/workflows/ci.yml"><img src="https://github.com/librehold-game/librehold/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/code-AGPL--3.0-blue" alt="Code licence: AGPL-3.0"></a>
  <img src="https://img.shields.io/badge/server-Go-00ADD8" alt="Server: Go">
  <img src="https://img.shields.io/badge/client-React-61DAFB" alt="Client: React">
</p>

<p align="center">
  <img src="docs/media/gameplay.gif" alt="Gameplay: a village growing inside its palisade, then an army marching across the map" width="720">
</p>

## About

LibreHold is a multiplayer village strategy game that runs in the browser. Every world lives on the server, so your
villages keep growing and your armies keep marching while you are away. Bots fill the map, so a world is never empty,
even with a handful of friends.

> **Alpha 0.1:** the rules and balance still change, and worlds or accounts may be reset between updates.

- **Instant games** — one-hour matches on a small map at ×100 speed. Press *Play now*, get a guest nickname and a
  developed village, and make your first conquest within minutes. No sign-up needed.
- **Private worlds** — host a short game for friends with an invite link and a QR code, and tune its length, seats,
  bots, pace and rules.
- **Long worlds** — persistent worlds created by the server admin, played over days.
- **Real strategy** — 15 buildings, 12 unit types, scouting, supports, field battles between marching armies,
  loyalty and conquest, a market and a farm assistant.
- **Bots that play** — computer players that build, farm barbarians, save for hetmans and wage war on each other and
  on you.
- **Own pixel art** and a layout that works on phones from 360 px.
- **Easy to host** — one Go binary, Postgres and Valkey in Docker Compose, with optional HTTPS through Caddy and
  nightly backups.

## Quick start

### Play

Open [librehold.org](https://librehold.org) and press **Play now**.

### Host your own server

You need a Linux server with Docker and Docker Compose.

```sh
git clone https://github.com/librehold-game/librehold.git
cd librehold/deploy
cp .env.example .env        # set the passwords and the admin account
docker compose build
docker compose up -d
```

The game is then at `http://<server>:8080/`. Before you open it to the internet, read
[`deploy/README.md`](deploy/README.md): it covers HTTPS, running behind a reverse proxy, backups and updates.

### Develop

Requirements: Node.js 24+, Go 1.27+ and Docker (for Postgres, Valkey and the database tests).

```sh
npm install
cd server && make db        # Postgres on localhost:5433, Valkey on localhost:6380
LH_DATABASE_URL='postgres://librehold:librehold@localhost:5433/librehold?sslmode=disable' LH_COOKIE_SECURE=false \
  LH_ADMIN_USERNAME=admin LH_ADMIN_PASSWORD=adminadmin go run ./cmd/server   # API on :8080
```

In a second terminal, from the repository root:

```sh
npm run dev:account         # account site: login, lobby, admin panel
npm run dev                 # the game; /api is proxied to :8080
```

Run the tests with `npm test` (client) and `npm run server:test` (server). The full guide, including the
environment variables and the end-to-end tests, is in [`docs/development.md`](docs/development.md).

## How it works

The Go server owns every rule and every balance value, and the Postgres database is the single source of truth: each
order and each scheduled event is one transaction under a per-world lock, so a restart loses nothing. The React client
only displays the state and sends orders. It reads the balance tables generated from the Go rules (`npm run gen`), so
the two never drift apart.

```text
server/          Go game server (hexagonal)
  internal/game    game rules, balance and bots; no I/O
  internal/app     use cases: accounts, lobby, orders, views, event worker
  internal/adapters  Postgres, HTTP API, WebSocket, file gate
  cmd/             server, bot simulator, archive export, generator
src/             React client (account site and game)
tests/           client tests (Vitest)
deploy/          Docker Compose, Caddy, backups
```

## Documentation

| Document | Contents |
|---|---|
| [`docs/development.md`](docs/development.md) | running locally, server layout, environment variables, time model, scripts |
| [`docs/gameplay.md`](docs/gameplay.md) | instant games, player worlds, world settings, bots, quests |
| [`docs/balance.md`](docs/balance.md) | balance formulas, where to change them, pacing tests |
| [`docs/interface.md`](docs/interface.md) | village view, map, art layers, phone layout |
| [`deploy/README.md`](deploy/README.md) | production deployment, HTTPS, backups and restore |
| [`ASSETS.md`](ASSETS.md) | origin and licence of every art and font file |
| [`CHANGELOG.md`](CHANGELOG.md) | notable changes per version |

## Contributing

Bug reports, ideas and pull requests are welcome. Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md) first: every
contributor signs the Contributor License Agreement ([`CLA.md`](CLA.md) for individuals,
[`CLA-ENTITY.md`](CLA-ENTITY.md) for organisations), and a bot will guide you through it on your first pull request.
Artwork is not open, so please ask before sending any graphics.

## Security

Please do not report security problems in public issues. Follow [`SECURITY.md`](SECURITY.md) instead.

## Licence

You are free to play, self-host, study, change and share LibreHold.

- **Code** — [GNU AGPL 3.0](LICENSE), with a few additional terms in [`LICENSING.md`](LICENSING.md): keep the credits,
  and give a modified version its own name. If you run a modified version for other people, share its source code.
- **Artwork** (`src/assets/pixel/`, `public/account/og/`, `docs/media/`) — free to use, unchanged, on any LibreHold
  server that is not commercial ([`LICENSE-ART.md`](LICENSE-ART.md)).
- **Fonts** — their authors' licence (SIL OFL 1.1), described in [`ASSETS.md`](ASSETS.md).

## Credits

Made by [Szymon Kocur](https://szymonkocur.com) and [contributors](CONTRIBUTORS.md), hosted by
[TomcatFlow](https://tomcatflow.com).
