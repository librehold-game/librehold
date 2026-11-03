# Changelog

Changes to LibreHold for players, for people who run it and for developers, newest first; the site's footer links
here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/). This file is written from the git history by `scripts/release.sh`; do not
edit it by hand.

## [0.10.0](https://github.com/librehold-game/librehold/releases/tag/v0.10.0) — 2026-11-03 (alpha)

### Added

- A map checkbox hides your farm raids, remembered per world ([#31](https://github.com/librehold-game/librehold/pull/31)) by @SKocur
- Villagers train and work in their buildings, and the armies march in the same style ([#38](https://github.com/librehold-game/librehold/pull/38)) by @SKocur

### Changed

- Auto-farm reports carry a tag, arrive read, and can be hidden from the reports list ([#28](https://github.com/librehold-game/librehold/pull/28)) by @ciefa
- Stamp release versions in server logs and bug reports ([#33](https://github.com/librehold-game/librehold/pull/33)) by @Sinkleberg

### Fixed

- Number fields can be emptied and typed into without snapping back ([#32](https://github.com/librehold-game/librehold/pull/32)) by @SKocur
- Steady the logout socket test and speed up the game simulations ([#37](https://github.com/librehold-game/librehold/pull/37)) by @SKocur

### Contributors

@SKocur, @Sinkleberg, @ciefa

- @Sinkleberg made their first contribution in [#33](https://github.com/librehold-game/librehold/pull/33)
## [0.9.0](https://github.com/librehold-game/librehold/releases/tag/v0.9.0) — 2026-11-02 (alpha)

### Added

- A farmer mows the wheat, and the village people are smaller and sharper ([2c4814a](https://github.com/librehold-game/librehold/commit/2c4814aff43f311b65505995d2369bb8ec82bf58)) by @SKocur
- Instant worlds show your place in the ranking next to the time left ([#29](https://github.com/librehold-game/librehold/pull/29)) by @SKocur

### Changed

- The site's changelog link opens CHANGELOG.md on GitHub instead of a page of its own ([32a784b](https://github.com/librehold-game/librehold/commit/32a784bc69da66cce85b054e84bfac7255d093d6)) by @SKocur

### Fixed

- Villagers stay visible while they walk through an open village gate ([61866cf](https://github.com/librehold-game/librehold/commit/61866cf4e8796bf6b9c9c6156a657dc1811ea063)) by @SKocur

### Contributors

@SKocur

## [0.8.0](https://github.com/librehold-game/librehold/releases/tag/v0.8.0) — 2026-11-02 (alpha)

### Added

- Add quick build and recruitment to the map village menu ([#21](https://github.com/librehold-game/librehold/pull/21)) by @right-ward
- Strong and medium bots build faster and use the auto-farm ([#23](https://github.com/librehold-game/librehold/pull/23)) by @SKocur

### Contributors

@SKocur, @right-ward

- @right-ward made their first contribution in [#21](https://github.com/librehold-game/librehold/pull/21)
## [0.7.0](https://github.com/librehold-game/librehold/releases/tag/v0.7.0) — 2026-11-01 (alpha)

### Added

- Scan every pull request and push to main with CodeQL ([#15](https://github.com/librehold-game/librehold/pull/15)) by @ciefa
- Start the whole development environment with one command, npm run dev:all ([#16](https://github.com/librehold-game/librehold/pull/16)) by @ciefa

### Changed

- Pin the Docker images by digest ([#17](https://github.com/librehold-game/librehold/pull/17)) by @ciefa

### Fixed

- Stop cancelling requests without a body after 30 seconds ([bf25de7](https://github.com/librehold-game/librehold/commit/bf25de7a33cdfd30c44a7f9809f6376292eabce8)) by @SKocur
- The bottom bar switches views on the first tap in Firefox on Android ([#20](https://github.com/librehold-game/librehold/pull/20)) by @ciefa
- The village card stays out of the way of the village popup, and a tap on the map opens the tile ([#22](https://github.com/librehold-game/librehold/pull/22)) by @SKocur

### Contributors

@SKocur, @ciefa

- @ciefa made their first contribution in [#20](https://github.com/librehold-game/librehold/pull/20)
## [0.6.1](https://github.com/librehold-game/librehold/releases/tag/v0.6.1) — 2026-10-31 (alpha)

### Fixed

- Keep battles out of the public feed ([16dc738](https://github.com/librehold-game/librehold/commit/16dc73898f3ab0e5a6b7a6ec324f6831a6862969)) by @SKocur
- Farm assistant counts the local day and estimates only from own reports ([8f78d2c](https://github.com/librehold-game/librehold/commit/8f78d2c5f62475bf2968bb817c5760f3babedaef)) by @SKocur

### Contributors

@SKocur

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

