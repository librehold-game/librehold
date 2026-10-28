# Changelog

Notable changes for people who run or develop LibreHold. Players see the same story in the game's own changelog
page. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

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

[0.1.0]: https://github.com/librehold-game/librehold/releases/tag/v0.1.0
