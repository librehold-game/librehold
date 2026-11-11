---
name: player-order
description: Use when adding a LibreHold player action that changes the game - a new button or form that sends an order to the server, or a new order type in server/internal/app/orders.go.
---

# Adding a player order

An order goes from a button, through `run()` in the store, to an entry in `server/internal/app/orders.go`, which calls
a function in `server/internal/game` inside one transaction; then every affected player gets an update. The server
decides. The browser only offers the button and explains why it can't be pressed.

## Follow an existing order

`withdraw-support` is a small one, end to end: `git grep -n -e withdraw-support -e WithdrawSupport`.

## Steps

1. **The rule**, in `server/internal/game`: a function taking `(w World, actor PlayerID, ...)`. Check first that the
   actor may do it (`NotOwner(...)` when the village or army isn't theirs), then every input: counts, targets, costs.
   Refuse with `Rule("code", params)` so the browser can explain. Test each refusal and the success in
   `server/internal/game/*_test.go`.
2. **The order**, in `orders.go`: an input struct and an entry
   `"kebab-name": order(func(w WorldTx, me game.PlayerID, in yourIn) error { ... })`. The body is decoded strictly
   (unknown fields are refused). Give the struct a `validate()` for shapes the rule doesn't check, such as list lengths
   (`checkIDs`, `MaxOrderIDs`) and id formats. Test it in `server/internal/app/orders_test.go`.
3. **Who sees the result.** If it changes what another player sees (support, an attack, a trade), make sure
   `PlayerState` in `views.go` shows them only what they may see, and that `Changes.Updates` in `updates.go` sends
   them a "state" update.
4. **The store action**, in `src/state/gameStore.ts`: `run('kebab-name', { ... })`, next to the existing actions.
5. **The UI.** Disable the control with a reason when the order would be refused, using the same checks as the server
   (as hints only) and reusing the existing helpers, for example `blockerText` in `RecruitScreen.tsx`. Every refusal
   code needs an `errors.<code>` entry in both `common.json` catalogues, or players see "unknown error". It works with
   the keyboard and on a 360 px phone.
6. **Browser tests** in `tests/*.test.tsx`: the control, its disabled reason, and the store call.
7. **What players read:** the guide or encyclopedia entry for the feature, in both languages.

## Common mistakes

- The check exists only in the browser, so a hand-made request gets past it.
- The owner is read from the village when the order lands instead of from the command, which breaks after a conquest.
- A refusal code has no `errors.*` entry.
- A copied helper drifts from the one it copied.
- A new per-address limit locks out a LAN party; orders are already limited per account
  (`LH_WORLD_REQUESTS_PER_MINUTE`).
