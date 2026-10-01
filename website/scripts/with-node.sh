#!/usr/bin/env bash
# Runs a command in website/ with the Node toolchain website/oku.toml pins.
# The just recipes start from the repo root, whose oku.toml carries no Node.
set -euo pipefail

cd "$(dirname "$0")/.."
command -v oku >/dev/null && eval "$(oku env)"
if ! command -v node >/dev/null; then
  echo "with-node.sh: node not found, run 'oku sync && oku allow' in website/" >&2
  exit 1
fi
exec "$@"
