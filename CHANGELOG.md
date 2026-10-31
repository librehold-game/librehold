# Changelog

Changes for people who run or develop LibreHold. Players see the same story in the game's own changelog page. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/). This file is written from the git history by `scripts/release.sh`; do not
edit it by hand.

## [0.6.0](https://github.com/librehold-game/librehold/releases/tag/v0.6.0) — 2026-10-31 (alpha)

### Added

- Draw the interface's signs as pixel icons and open the quest lines side by side ([7c6b813](https://github.com/librehold-game/librehold/commit/7c6b81371c8404fc8dc7b6dfce48dc840a5c8c9a)) by @SKocur

### Contributors

@SKocur

## [0.5.0](https://github.com/librehold-game/librehold/releases/tag/v0.5.0) — 2026-10-31 (alpha)

### Added

- Add a public world page with every setting, the full ranking and player pages ([307b3ec](https://github.com/librehold-game/librehold/commit/307b3ecf2436e0cf80815fba207c9e7ca3060b87)) by @SKocur
- Record what a player had before each order, for teaching bots ([bc43ec7](https://github.com/librehold-game/librehold/commit/bc43ec7eb338877d6188703bdf0d1361da982ab0)) by @SKocur

### Fixed

- Count a paladin away from its village toward the one-paladin limit ([db55e89](https://github.com/librehold-game/librehold/commit/db55e8947fae7b0ab9c420cee95faed23662eaac)) by @SKocur

### Contributors

@SKocur

## [0.4.0](https://github.com/librehold-game/librehold/releases/tag/v0.4.0) — 2026-10-30 (alpha)

### Changed

- Self-host from the published image and write the changelog with git-cliff ([8aaeff4](https://github.com/librehold-game/librehold/commit/8aaeff4975a11ac653d516e4f7ce762dc441e424)) by @SKocur
- Show a minimal compose file for self-hosting in the README ([05f92a9](https://github.com/librehold-game/librehold/commit/05f92a9529547537a08f94a6468587d6031a1d87)) by @SKocur
- Show what unlocks buildings and units inside the game ([db83bb0](https://github.com/librehold-game/librehold/commit/db83bb0abf1c4edc03852586a374458bfaafdbb4)) by @SKocur

### Contributors

@SKocur

## [0.3.0](https://github.com/librehold-game/librehold/releases/tag/v0.3.0) — 2026-10-30 (alpha)

### Added

- Open the village card on the map only after the cursor rests on it ([#13](https://github.com/librehold-game/librehold/pull/13)) by @SKocur

### Changed

- Move the performance tests out of e2e and make the late-join test steady ([0e785bf](https://github.com/librehold-game/librehold/commit/0e785bf33450a9b2d9ef7c3d1f4838aafc25b344)) by @SKocur
- Show ended worlds as ended in the admin panel ([1e393fd](https://github.com/librehold-game/librehold/commit/1e393fd4ec3a4bc4385355c40537ffa1cb0395e0)) by @SKocur
- Publish the server image to GHCR on every version tag ([aad24dc](https://github.com/librehold-game/librehold/commit/aad24dcbcbe2981f6f6625e96c62d6edaacede95)) by @SKocur
- Use less Actions cache in CI and skip tests for docs-only changes ([3ed4d90](https://github.com/librehold-game/librehold/commit/3ed4d901fec6b0dc1270f57bbdb27395584859e4)) by @SKocur

### Fixed

- Refresh the ranking every minute in instant games ([#12](https://github.com/librehold-game/librehold/pull/12)) by @SKocur

### Contributors

@SKocur

## [0.2.1](https://github.com/librehold-game/librehold/releases/tag/v0.2.1) — 2026-10-29 (alpha)

### Changed

- Gameplay clip and play link in the README; Dependabot for compose files ([7324086](https://github.com/librehold-game/librehold/commit/7324086412c97a910210e272d64038d2b904761d)) by @SKocur
- Friendlier licence summaries ([f609917](https://github.com/librehold-game/librehold/commit/f609917442a2e84d4989278be1614f36f764447f)) by @SKocur
- Unmount React roots after hydration tests; build before tests in CI ([0a41ec5](https://github.com/librehold-game/librehold/commit/0a41ec53176219abe3fbd317e6aedeb54899c629)) by @SKocur
- Choose an instant game from a list, join until it ends, end abandoned games ([96b0631](https://github.com/librehold-game/librehold/commit/96b0631f56396157c64bed0fc2253dc92d3d9953)) by @SKocur
- Let a player leave an instant game in any state ([08b8bd2](https://github.com/librehold-game/librehold/commit/08b8bd207d41b77bd05e44fe90b185d2a181d5f6)) by @SKocur

### Contributors

@SKocur

## [0.1.0](https://github.com/librehold-game/librehold/releases/tag/v0.1.0) — 2026-10-28 (alpha)

### Changed

- LibreHold, a browser village strategy game ([e680f73](https://github.com/librehold-game/librehold/commit/e680f7323ce4e3b1678860bf3934365522878067)) by @SKocur

### Contributors

@SKocur

- @SKocur made their first contribution

