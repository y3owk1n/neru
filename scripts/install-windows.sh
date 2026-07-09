#!/usr/bin/env bash
#
# neru Windows installer (minimal). Invoked by `just install` under a
# bash such as Git Bash; build the exe first with `just build-windows`.
set -euo pipefail

# Run from the repo root so build/, bin/, and `just` resolve.
cd "$(dirname "$0")/.."

# Any of -y / --yes on the command line auto-accepts every prompt.
assume_yes=0
for arg in "$@"; do
    case "$arg" in
        -y | --yes) assume_yes=1 ;;
        *) echo "unknown argument: $arg (use -y to auto-accept prompts)" >&2; exit 2 ;;
    esac
done

# ask "prompt" -> prints the reply on stdout. Under -y it echoes the prompt with
# a "y" and answers yes without reading, so the whole run is non-interactive.
ask() {
    if [ "$assume_yes" -eq 1 ]; then
        printf '%sy\n' "$1" >&2
        printf 'y'
        return 0
    fi
    local reply
    reply="$(ask "$1")"
    printf '%s' "$reply"
}


# Minimal Windows install: a per-user binary plus optional autostart via
# the registry Run key. Windows has no launchd or systemd and `neru
# services` is macOS-only, so autostart is authored here, not by Neru.
# Runs under a bash (e.g. Git Bash); `just` needs cygpath on PATH to
# translate the shebang, and reg is called with MSYS2_ARG_CONV_EXCL so
# its /flags are not path-mangled. Windows support is partial (grid,
# recursive grid, scroll, hotkeys, mouse injection, UIA); see
# docs/CROSS_PLATFORM.md.
arch="$(uname -m)"
case "$arch" in
    aarch64 | arm64) arch=arm64 ;;
    *) arch=amd64 ;;
esac
exe_src="bin/neru-windows-$arch.exe"
if [ ! -f "$exe_src" ]; then
    echo "Neru has not been built for Windows yet ($exe_src is missing)."
    build_reply="$(ask "Build it now with 'just build-windows $arch'? [y/N] ")"
    case "$build_reply" in
        [Yy] | [Yy][Ee][Ss]) just build-windows "$arch" ;;
        *) echo "Aborted. Run 'just build-windows' first, then 'just install'." >&2; exit 1 ;;
    esac
fi

# Step 1: the binary, to a per-user location. Editing PATH from a script
# is error-prone, so print the directory to add rather than modifying it.
echo "Step 1/2: Binary"
dst_dir="${LOCALAPPDATA:-$HOME/AppData/Local}/Programs/neru"
mkdir -p "$dst_dir"
cp "$exe_src" "$dst_dir/neru.exe"
echo "✓ Installed $dst_dir/neru.exe"
echo "  Add this directory to your PATH if it is not already:"
echo "      $dst_dir"

# Step 2: autostart via the per-user Run key (no admin). Uses the full
# exe path, so it does not depend on PATH.
echo "Step 2/2: Autostart"
run_reply="$(ask "Start Neru at login (a registry Run entry)? [y/N] ")"
case "$run_reply" in
    [Yy] | [Yy][Ee][Ss])
        win_exe="$(cygpath -w "$dst_dir/neru.exe" 2>/dev/null || echo "$dst_dir/neru.exe")"
        # MSYS2_ARG_CONV_EXCL='*' stops Git Bash from rewriting reg's
        # /v /t /d /f flags (and the HKCU\... key) as POSIX paths.
        if MSYS2_ARG_CONV_EXCL='*' reg add "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" /v Neru /t REG_SZ /d "\"$win_exe\" launch" /f >/dev/null 2>&1; then
            echo "✓ Neru will start at login"
            echo "  Remove later with: MSYS2_ARG_CONV_EXCL='*' reg delete \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\" /v Neru /f"
        else
            echo "Could not write the Run key; add autostart manually." >&2
        fi
        ;;
    *)
        echo "Skipped autostart. Start Neru with: neru.exe launch"
        ;;
esac
echo "Windows support is partial; see docs/CROSS_PLATFORM.md."
