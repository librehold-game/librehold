---
name: run-and-screenshot
description: Use when a LibreHold change needs checking in the running game or account site, or a pull request needs screenshots on desktop and phone.
---

# Running the game and taking screenshots

## Start it

```sh
cd server && make db && cd ..   # Postgres :5433 and Valkey :6380 in Docker (make db-down stops them)
npm run dev:all                 # builds and starts the API :8080, then the account site :5173 and the game :5174
```

Run `dev:all` in the background. It is ready when it prints
`account site http://localhost:5173/, game http://localhost:5174/game/game.html?world=<id>`. Stopping it stops all
three.

## Get into a game

- **Quickest:** open `http://localhost:5173/` and press **Play now**. A guest gets a developed village in an instant
  game. Skip the tutorial.
- **A long world:** log in at `http://localhost:5173/` as `admin` / `adminadmin`, create a world in the Admin panel,
  and join it from the lobby. The game opens at `http://localhost:5174/game/game.html?world=<id>`.
- Add `&dev=1` to the game address to get the store as `window.__lh` in the console for reading state. Time and
  resources can't be changed from the browser: to show a later stage, play to it, or set it up in a test instead.

## Screenshots

Take every view the change touches at both sizes:

| Size | Viewport |
|---|---|
| desktop | 1280 × 800 |
| phone | 360 × 740, the narrowest width the layout supports |

If the change adds text, take both languages (the `lang` cookie, `en` or `pl`). Save the images outside the
repository and attach them to the pull request; never commit them.

With a browser tool (Claude in Chrome, Playwright and similar), drive the page directly. With Node and a local Chrome,
`scripts/record/browser.mjs` has `launch()` and `playAsGuest(browser, baseUrl, viewport)`, which presses Play now and
skips the tutorial. It needs `puppeteer-core` (`npm i --no-save puppeteer-core`) and `CHROME` set to a Chrome or
Chromium binary.

## Check, don't only look

- Keyboard: Tab reaches every new control, and Enter or Space works it.
- Phone: nothing scrolls sideways, and stacked tables show their labels.
- Settings → Interface → Village animations: the change still works in "limited" and "off".
- The browser console shows no errors or warnings.
