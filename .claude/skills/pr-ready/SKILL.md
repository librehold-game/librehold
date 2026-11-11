---
name: pr-ready
description: Use before opening a pull request on LibreHold, before pushing new commits to an open one, or when asked whether a change is ready for review.
---

# Ready for a pull request

Pull requests keep coming back for the same things: CI red on `gofmt`, a local dev helper left in, guides that no
longer match the game, a description that claims something the code doesn't do. Go through all four steps.

## 1. Run the checks CI runs

```sh
bash .claude/skills/pr-ready/scripts/check.sh         # client and server
bash .claude/skills/pr-ready/scripts/check.sh --e2e   # also end-to-end: for changes to UI flows or the API
```

Everything must pass. A failing test is fixed in the code, never skipped, weakened or deleted.

## 2. Look for leftovers

```sh
bash .claude/skills/pr-ready/scripts/leftovers.sh     # compares with upstream/main, or origin/main
```

It lists suspicious added lines (debug output, connection strings, focused or skipped tests, TODOs) and gaps between
files (English text without Polish, rules without tests, an edited migration). Fix every hit, or be sure it belongs.

## 3. Read the whole diff

Read `git diff <base>` line by line and answer each question:

| Question | If not |
|---|---|
| Does every changed line serve this change? | Revert reformatting, re-indents and unrelated renames; restore deleted comments. |
| Is it one change? | Split it. A rule change and a redesign are two PRs. |
| Did something players see change? | Update `guides.json` (en and pl), `docs/*.md`, quest texts, FAQ, encyclopedia, comments. |
| Is there new player-facing text? | English and Polish, Polish plural forms, game names instead of code ids. |
| Is another game mentioned anywhere? | Remove it, comparisons and other games' settings names included. |
| Are strings left that nothing uses? | Delete them from both catalogues. |
| Is this your first contribution? | Add yourself to `CONTRIBUTORS.md`. |

## 4. Write the description

Fill in every section and box of `.github/PULL_REQUEST_TEMPLATE.md`. Tick a box that doesn't apply and add a note,
such as "(not needed here)".

- **Title:** what players or hosts notice, because it becomes the changelog line: "Players can hide the build plan from
  the Village tab", not "Add showBuildPlan pref".
- **Issue:** `Fixes #N`.
- **What and why:** written for the reviewer. Every claim is one you checked in the code. Name the trade-offs and
  anything you left out on purpose.
- **AI and translation:** say which parts an agent wrote, and whether a translator wrote the Polish.
- **Screenshots** for visible changes, desktop and phone (`run-and-screenshot`).

## Common mistakes

- CI stops at `gofmt`, so none of the server tests run.
- A `server/cmd/...` helper with a hard-coded connection string or world id ends up in the PR.
- The in-game guide says one thing and the game does another.
- The description states a rule the code doesn't have ("support only goes to your own villages").
- English text changed and the Polish entry didn't.
