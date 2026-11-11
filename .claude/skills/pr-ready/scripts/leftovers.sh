#!/usr/bin/env bash
# Lists what reviews keep finding in pull requests: suspicious added lines and gaps between changed files.
# Every hit needs a look; not every hit is wrong.
# Usage: bash .claude/skills/pr-ready/scripts/leftovers.sh [base-ref]   (default: upstream/main, else origin/main)
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

base_ref=${1:-}
if [ -z "$base_ref" ]; then
  if git rev-parse --verify -q upstream/main >/dev/null; then base_ref=upstream/main; else base_ref=origin/main; fi
fi
base=$(git merge-base HEAD "$base_ref") || { echo "No merge base with $base_ref; pass a base ref."; exit 2; }
echo "Comparing the working tree with $base_ref (merge base ${base:0:7})."

hits=0
note() { hits=$((hits + 1)); printf '  - %s\n' "$1"; }

# Added lines, as "file:line: text", for files matching the given pathspecs.
added() {
  git diff -U0 --no-color "$base" -- "$@" | awk '
    /^\+\+\+ / { file = substr($0, 7); next }
    /^@@/ { split($3, a, ","); line = substr(a[1], 2) + 0; next }
    /^\+/ { print file ":" line ": " substr($0, 2); line++ }'
}

scan() { # title, regex, pathspecs...
  local title=$1 regex=$2
  shift 2
  local found
  found=$(added "$@" | grep -E "$regex" | head -20)
  if [ -n "$found" ]; then
    printf '\n%s\n' "$title"
    printf '%s\n' "$found" | sed 's/^/    /'
    hits=$((hits + 1))
  fi
}

scan "Debug output:" 'console\.(log|debug)\(|debugger;|fmt\.Print(ln|f)?\(' 'src' 'server/internal'
scan "Focused or skipped tests:" '\b(it|describe|test)\.(only|skip)\(|\bt\.Skip(Now|f)?\(' 'tests' 'server' 'src'
scan "Connection strings or local addresses:" 'postgres://|redis://|sslmode=|localhost:(5433|6380|8080)' \
  'src' 'server' 'tests' ':!server/Makefile'
scan "TODO, FIXME or XXX:" '\b(TODO|FIXME|XXX)\b' 'src' 'server' 'tests'

changed=$(git diff --name-only "$base"; git ls-files --others --exclude-standard)
status=$(git diff --name-status "$base")
has() { printf '%s\n' "$changed" | grep -Eq "$1"; }

printf '\nFiles:\n'
for f in $(printf '%s\n' "$status" | awk '$1 != "A" && $2 ~ /^server\/internal\/adapters\/postgres\/migrations\// {print $2}'); do
  note "$f was edited: never change a migration already on main, add a new one"
done
for f in $(git ls-files --others --exclude-standard); do
  note "$f is untracked and not scanned above: add it if it belongs to the change, otherwise keep it out of the PR"
done
for lang in en pl; do
  other=$([ "$lang" = en ] && echo pl || echo en)
  for f in $(printf '%s\n' "$changed" | grep -E "^src/i18n/catalog/$lang/.*\.json$"); do
    twin="src/i18n/catalog/$other/${f#src/i18n/catalog/$lang/}"
    printf '%s\n' "$changed" | grep -qx "$twin" || note "$f changed but $twin didn't"
  done
done
if has '^server/internal/game/[^/]+\.go$' && ! has '^server/internal/game/.*_test\.go$'; then
  note "rules in server/internal/game changed without a test there"
fi
if has '^src/' && ! has '^tests/'; then note "src/ changed without a change in tests/"; fi
if has '^server/internal/game/(data_|balance)' && ! has '^src/data/balance\.json$'; then
  note "balance data changed: run npm run gen if the browser shows it (go test fails while it is stale)"
fi
if has '^(src/data/balance\.json|tests/fixtures/)' && ! has '^server/'; then
  note "generated files changed without a server change: were they edited by hand?"
fi
if has '^server/cmd/'; then note "server/cmd/ changed: make sure it isn't a local dev helper"; fi
if has '^server/internal/game/' && ! has '^(docs/|src/i18n/catalog/(en|pl)/guides\.json)'; then
  note "game rules changed: do the guides (guides.json) or docs/ need the same change?"
fi

if [ "$hits" -eq 0 ]; then echo "  nothing found"; fi
