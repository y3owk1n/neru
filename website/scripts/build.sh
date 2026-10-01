#!/usr/bin/env bash
# Builds both docs channels into website/dist:
#   dist/latest   the newest v* tag's docs/, served at the root
#   dist/nightly  the checkout's docs/, served under nightly/
# The site code always comes from this checkout, so a theme or link fix
# reaches both channels without a release. Run from the repo root or website/.
set -euo pipefail

website="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(dirname "$website")"
cd "$website"

tag="$(git -C "$repo" tag --list 'v*' --sort=-v:refname | head -n1)"
if [[ -z "$tag" ]]; then
  echo "build.sh: no v* tag found, fetch tags first (git fetch --tags)" >&2
  exit 1
fi

worktree="$(mktemp -d)"
cleanup() { git -C "$repo" worktree remove --force "$worktree" >/dev/null 2>&1 || true; }
trap cleanup EXIT
git -C "$repo" worktree add --detach --quiet "$worktree" "$tag"

rm -rf dist
export NERU_LATEST_VERSION="$tag" ASTRO_TELEMETRY_DISABLED=1

# The content cache is keyed by entry id, not by docs/ directory, so each
# channel starts from an empty one.
build() {
  rm -rf .astro node_modules/.astro
  NERU_DOCS_CHANNEL="$1" NERU_DOCS_DIR="$2" NERU_DOCS_REF="$3" npx astro build
}
build nightly "$repo/docs" main
build latest "$worktree/docs" "$tag"

# Serve latest at the root with nightly beside it.
mv dist/nightly dist/latest/nightly
mv dist/latest dist/site
echo "build.sh: latest=$tag nightly=main -> $website/dist/site"
node scripts/check-links.mjs
