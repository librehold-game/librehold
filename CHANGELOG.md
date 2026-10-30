# Changelog

Notable changes for people who run or develop LibreHold. Players see the same story in the game's own changelog
page. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Every `v*` tag publishes the server image to `ghcr.io/librehold-game/librehold` (amd64 and arm64), tagged with
  the version, its minor line (`0.2`) and `latest`. Only org admins can create release tags.

### Changed

- The version lives in the release tag only: `package.json` has no version, and the README badge shows the latest
  tag.

### Fixed

- The ranking in instant games refreshes every minute instead of every full hour, so points from a capture show
  within a minute ([#10](https://github.com/librehold-game/librehold/issues/10)).
- A village's card on the map opens only after the cursor rests on it for 300 ms, so moving the mouse across the
  map no longer flashes cards ([#9](https://github.com/librehold-game/librehold/issues/9)).
- The admin panel shows finished worlds as ended, with the reason and time, instead of running, and hides Pause and
  Open/Close for them.

## [0.2.1] — 2026-10-29 (alpha)

### Fixed

- Leaving an instant game always works: after the game has ended, while the admin has it paused, and when you
  already left from another tab.

## [0.2.0] — 2026-10-29 (alpha)

### Changed

- **Play now** shows the running instant games (time left, players, leader) and you choose which one to join, instead
  of being placed in one. With no game running it starts a new one as before.
- Instant games accept players until they end, not only in the first 10 minutes. A player is in one running instant
  game at a time; leaving it frees the seat.
- An instant game that no human has had open for 5 minutes ends early (reason `abandoned`), keeps its results and no
  longer holds a place under `LH_INSTANT_MAX_WORLDS`.
- The results screen's **Play again** opens the game list.

## [0.1.0] — 2026-10-28 (alpha)

The first public release. Worlds and accounts may still be reset between alpha versions.

### Added

- Browser village strategy game: 15 buildings, 12 unit types, scouting, supports, field battles between marching
  armies, loyalty and conquest, a market and a farm assistant.
- Instant games: one-hour matches at ×100 speed that guests join with **Play now**, no account needed.
- Private worlds that a registered player hosts for friends, with an invite link and a QR code.
- Long worlds created by the admin, played over days.
- Bots that build, farm, defend and wage war, with their own daily rhythm.
- A public site in English and Polish: home page, guides, unit and building pages, player
  profiles and game results.
- Go game server on Postgres, with every order and event in one transaction under a per-world lock; limits shared
  through Valkey.
- Docker Compose deployment with optional HTTPS through Caddy and nightly database backups.

[Unreleased]: https://github.com/librehold-game/librehold/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/librehold-game/librehold/releases/tag/v0.2.1
[0.2.0]: https://github.com/librehold-game/librehold/releases/tag/v0.2.0
[0.1.0]: https://github.com/librehold-game/librehold/releases/tag/v0.1.0
