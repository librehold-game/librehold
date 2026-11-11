---
name: world-setting
description: Use when adding, changing or removing a LibreHold world setting - an option an admin or a host picks for a world (WorldConfig), such as a feature switch, a pace or a limit.
---

# Adding a world setting

A world setting is chosen in two forms (the admin panel and the hosting form), has a value in instant games, is shown
on the world page, and is read by the rules. Setting PRs came back several rounds because one of those was missing,
or because switching the setting mid-game left things behind. Agree in the issue first whether the setting is wanted
and whether it may change after the world is created.

## Copy an existing setting

Pick the closest one and follow it everywhere:

```sh
git grep -n -i 'fieldBattles'   # a switch both admins and hosts choose
git grep -n -i 'noBuildPlans'   # a switch that can change mid-game
```

## Where a setting lives

| Place | Files |
|---|---|
| The rule | `WorldConfig` in `server/internal/game/types.go`, and the code that reads it |
| Instant games | `InstantWorldConfig` in `server/internal/game/instant.go` copies the defaults: choose the value there on purpose, since guest accounts cost nothing |
| Admin panel | `FIELDS` in `src/account/worldConfig.ts` (with `creationOnly`), `changeableSettings` in `server/internal/app/admin.go`, labels in `account.json` |
| Hosting form | `HostForm` and `PlayerWorldConfig` in `server/internal/app/hosted.go`, `src/account/host/HostForm.tsx`, `hostDefaults.ts`, `src/account/api.ts`, labels in `host.json` |
| World page | `PublicSettings` in `server/internal/app/publicworlds.go`, `src/account/worlds/WorldSettings.tsx`, labels in `worlds.json` |
| The game | `WorldConfig` in `src/engine/types.ts`, and the panels that show or hide the feature |
| Docs | the "World settings" table in `docs/gameplay.md`, and the guides if players see the effect |

Every label has an English and a Polish entry in `src/i18n/catalog/{en,pl}/`.

## Changing mid-game

A setting that can change after creation must deal with what its old value left behind, with a test for it.
`SyncFeatureToggles` in `server/internal/game/farm.go` shows the pattern: switching auto-farming or build plans off
clears the players' schedules and plans, so nothing stays hidden but still running, and nothing old starts again when
the switch comes back on. A setting that shapes the world itself (map size, barbarian density, bot count) stays
`creationOnly`.

## Tests

- `server/internal/app/admin_test.go`, `hosted_test.go`, `publicworlds_test.go`: the value is accepted, refused when
  out of range or after creation, and shown on the world page.
- `server/internal/game/*_test.go`: the rule with the setting on and off, bots included.
- `tests/worldForm.test.tsx`, `tests/hostUi.test.tsx`, `tests/worldPage.test.tsx`: the forms and the world page.

## Common mistakes

- Only the admin form gets the setting, so hosts can't choose it.
- Players can't see it on the world page before they join.
- Instant games inherit the default without anyone deciding it.
- Switched off mid-game, it leaves plans or schedules nobody can see or clear.
- An error message mentions "the host" in an admin world, which has none.
