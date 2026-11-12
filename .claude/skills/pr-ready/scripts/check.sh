#!/usr/bin/env bash
# Runs the checks CI runs on a pull request, in the same order.
# Usage: bash .claude/skills/pr-ready/scripts/check.sh [--e2e]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

step() { printf '\n== %s\n' "$1"; }

step "client: format"
npm run --silent format:check || { printf '\nNot formatted. Run: npm run format\n'; exit 1; }
step "client: typecheck"
npm run --silent typecheck
step "client: build (some tests check the built site)"
npm run --silent build:web
step "client: tests"
npm test --silent

step "server: gofmt"
unformatted=$(cd server && gofmt -l .)
if [ -n "$unformatted" ]; then
  printf '%s\nNot formatted. Run: cd server && gofmt -w .\n' "$unformatted"
  exit 1
fi
step "server: go vet"
(cd server && go vet ./...)
step "server: go test (the Postgres tests need Docker)"
(cd server && go test -timeout 20m ./...)
step "server: govulncheck"
(cd server && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...)

if [ "${1:-}" = "--e2e" ]; then
  step "end-to-end"
  (cd server && make e2e)
fi

printf '\nAll checks passed.\n'
