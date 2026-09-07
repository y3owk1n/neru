#!/usr/bin/env bash
#
# Assemble the release layout: bin/neru, share/man/man1 and, on macOS, an
# ad-hoc signed Neru.app. It is the exact tree a release zip unpacks to, so
# scripts/install.sh --from and CI's publish jobs both consume it.
#
#   scripts/dist.sh [BIN] [OUT] [BUNDLE_VERSION] [SHORT_VERSION] [BUILD_ID]
#
# Invoked by `just dist` on macOS and Linux, which passes the git describe
# string in NERU_DIST_VERSION. Windows uses scripts/dist.ps1. Kept out of
# the justfile because a shebang recipe needs cygpath on Windows.
set -euo pipefail
cd "$(dirname "$0")/.."

case "$(uname -s)" in
    Darwin) host=macos ;;
    MINGW* | MSYS* | CYGWIN*) host=windows ;;
    *) host=linux ;;
esac
exe=""
[ "$host" = windows ] && exe=.exe

bin="${1:-}"
out="${2:-build/dist}"
bundle_version="${3:-}"
short_version="${4:-}"
build_id="${5:-}"
version="${NERU_DIST_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

[ -n "$bin" ] || bin="bin/neru$exe"
if [ ! -f "$bin" ]; then
    echo "dist: $bin not found; run 'just build' first" >&2
    exit 1
fi

rm -rf "$out"
mkdir -p "$out/bin" "$out/share/man/man1"
cp "$bin" "$out/bin/neru$exe"
chmod +x "$out/bin/neru$exe"
go run ./cmd/genman "$out/share/man/man1" >/dev/null

if [ "$host" = macos ]; then
    # Local builds derive the plist versions from the git describe string:
    # v1.52.0-3-gabc-dirty becomes 1.52.0, anything else 0.0.0.
    if [ -z "$short_version" ]; then
        short_version="$(printf '%s' "$version" | sed -e 's/^v//' -e 's/-.*//')"
        case "$short_version" in
            [0-9]*.[0-9]*.[0-9]*) ;;
            *) short_version=0.0.0 ;;
        esac
    fi
    [ -n "$bundle_version" ] || bundle_version="$short_version"
    [ -n "$build_id" ] || build_id="$version"
    app="$out/Neru.app"
    mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
    cp "$bin" "$app/Contents/MacOS/neru"
    chmod +x "$app/Contents/MacOS/neru"
    cp resources/icon.icns "$app/Contents/Resources/icon.icns"
    cp resources/Neru.entitlements "$app/Contents/Resources/Neru.entitlements"
    sed \
        -e "s/BUNDLE_VERSION/$bundle_version/g" \
        -e "s/SHORT_VERSION/$short_version/g" \
        -e "s/BUILD_ID/$build_id/g" \
        resources/Info.plist.template >"$app/Contents/Info.plist"
    codesign --force --deep --sign - --entitlements resources/Neru.entitlements --options runtime "$app"
fi
echo "✓ Release layout assembled in $out"
