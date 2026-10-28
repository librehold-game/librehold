# Contributing

Thank you for helping. Bug reports, ideas and pull requests are all welcome. Please follow the
[Code of Conduct](CODE_OF_CONDUCT.md), and report security problems privately as described in [`SECURITY.md`](SECURITY.md).

## Contributor License Agreement

Before your first pull request can be merged, you need to sign the Contributor License Agreement:

- `CLA.md` if you contribute as an individual. A bot asks you to sign with a comment on your first pull request.
- `CLA-ENTITY.md` if you contribute on behalf of a company or another organisation. It is signed by email.

In short: you keep the copyright to your contribution, and it always stays available under the project's licence
(GNU AGPL 3.0 with the additional terms in `LICENSING.md`). The CLA also lets the maintainer offer the project under
other terms, for example to an organisation that cannot use the AGPL. The CLA itself is the binding text.

Add yourself to `CONTRIBUTORS.md` in your first pull request.

## Getting started

You need Node.js 24+, Go 1.27+ and Docker. The steps to run the game locally are in the
[README](README.md#develop), and the full guide is in [`docs/development.md`](docs/development.md).

Good places to start:

- issues labelled `good first issue`;
- a rule or balance change: everything lives in `server/internal/game`, with tests next to it;
- a UI fix: React components in `src/ui`, tests in `tests/`.

## Making a change

1. Fork the repository and create a branch from `main`.
2. Make your change, with a test that fails without it.
3. Run the checks below; CI runs the same ones on your pull request.
4. Open a pull request that says what changed and why. Link the issue it fixes, if there is one.

```sh
npm run typecheck && npm test           # client
cd server && go vet ./... && go test ./...   # server (the Postgres tests need Docker)
npm run build:web && cd server && make e2e   # end-to-end tests
```

Things that are easy to miss:

- **The Go server owns every rule and number.** After changing a table or a rule the browser displays, run
  `npm run gen`. `go test ./...` fails while the generated files are stale.
- **Text goes through the catalogues.** Every string the player sees has an English and a Polish entry in
  `src/i18n/catalog/`. Code, comments and docs are in English.
- **Phone layout.** A new column in a `table-stack` table needs a `data-label` (see
  [`docs/interface.md`](docs/interface.md#phone-mobile-layout)).
- **Format before you commit:** `gofmt` for Go; the client has no formatter, so match the surrounding code.

Keep pull requests focused: one change per pull request is easier to review than several.

## Artwork

The artwork is not open ([`LICENSE-ART.md`](LICENSE-ART.md)): please ask at me@szymonkocur.com before sending any
graphics. The `npm run assets:*` scripts export art from source files that are not in this repository, so only the
maintainer can run them.
