#!/usr/bin/env bash
# Prepares a release: rewrites CHANGELOG.md from the git history with git-cliff (cliff.toml), commits it and tags it.
# Nothing is pushed; the script prints the push command. Usage: scripts/release.sh 0.4.0
set -euo pipefail

version=${1:?usage: scripts/release.sh 0.4.0}
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || { echo "not a version: $version" >&2; exit 1; }
tag="v$version"

cd "$(git rev-parse --show-toplevel)"
[[ $(git branch --show-current) == main ]] || { echo "release from main" >&2; exit 1; }
git diff --quiet HEAD || { echo "commit or stash your changes first" >&2; exit 1; }
# git-cliff reads pull requests and authors from GitHub, so every commit of the release must be pushed already.
git fetch -q origin main
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || { echo "push main first: it differs from origin/main" >&2; exit 1; }
! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || { echo "$tag already exists" >&2; exit 1; }

GITHUB_TOKEN=${GITHUB_TOKEN:-$(gh auth token)} git cliff --tag "$tag" --output CHANGELOG.md
git add CHANGELOG.md
git commit -m "Release $version"
git tag -a "$tag" -m "LibreHold $version"

echo "Check CHANGELOG.md, then publish with: git push origin main $tag"
