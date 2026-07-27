#!/usr/bin/env bash
#
# neru macOS uninstaller. Invoked by `just uninstall`; can also be run
# directly from the repo root. Undoes scripts/install-macos.sh in reverse.
set -euo pipefail

# Run from the repo root so build/, bin/, and `just` resolve.
cd "$(dirname "$0")/.."

# -y / --yes auto-accepts every prompt. --purge additionally offers to delete
# your config and logs; without it they are never touched, so -y on its own can
# never destroy a hand-tuned config.toml.
assume_yes=0
purge=0
for arg in "$@"; do
    case "$arg" in
        -y | --yes) assume_yes=1 ;;
        --purge) purge=1 ;;
        *) echo "unknown argument: $arg (use -y to auto-accept prompts, --purge to also remove config and logs)" >&2; exit 2 ;;
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
    read -r -p "$1" reply || reply=""
    printf '%s' "$reply"
}

# purge_list accumulates "label|dir" lines for the directories that actually
# exist. Collecting them before deleting anything is what lets the prompt show
# the fully resolved paths: XDG_CONFIG_HOME can point the config directory
# somewhere other than $HOME, and you should see where before saying yes.
purge_list=""
note_purge_target() {
    [ -d "$2" ] && purge_list="$purge_list$1|$2"$'\n'
    return 0
}

app_dst="/Applications/Neru.app"
neru_bin="$app_dst/Contents/MacOS/neru"
link_dst="/usr/local/bin/neru"

# Refuse to fight another installer, the same stance the install recipe takes.
# Homebrew's cask owns /Applications/Neru.app and the PATH symlink, and on disk
# its app is indistinguishable from a source install, so asking brew is the only
# reliable signal about who the files belong to.
if command -v brew >/dev/null 2>&1 && brew list --cask 2>/dev/null | grep -qxE 'neru|neru-nightly'; then
    echo "Neru is installed with Homebrew; this script would delete files brew tracks." >&2
    echo "Remove it with:  brew uninstall --cask y3owk1n/tap/neru" >&2
    exit 1
fi
# A Nix-managed install keeps its app inside the store rather than
# /Applications, so the check above never sees it. Its login agent does show up
# under a nix label, which is the signal to send the user to their config.
for plist in \
    "$HOME/Library/LaunchAgents/"org.nixos.*neru*.plist \
    "$HOME/Library/LaunchAgents/"org.nix-community.*neru*.plist \
    "/Library/LaunchAgents/"org.nixos.*neru*.plist; do
    [ -e "$plist" ] || continue
    echo "Neru is managed by nix-darwin or home-manager:" >&2
    echo "    plist: $plist" >&2
    echo "Remove Neru from that configuration and rebuild instead." >&2
    exit 1
done

echo "This removes $app_dst and the login agent, PATH symlink, completions"
echo "and man pages that 'just install' created."
if [ "$purge" -eq 0 ]; then
    echo "Your config and logs are kept; pass --purge to remove those too."
fi

# Step 1: the login agent and any running instance. Done first: a KeepAlive
# agent would respawn Neru from an app bundle we are about to delete.
echo "Step 1/5: Login agent"
if [ -x "$neru_bin" ]; then
    "$neru_bin" services uninstall >/dev/null 2>&1 || true
fi
# A detached "run now" instance from an earlier install is not known to
# launchd, so kill it separately.
pkill -f "$app_dst/Contents/MacOS/neru launch" >/dev/null 2>&1 || true
echo "✓ Login agent unloaded and any running Neru stopped"

# Step 2: the app bundle. Uses sudo when /Applications is not writable, the
# same escalation the installer uses to put it there.
echo "Step 2/5: App bundle"
if [ -e "$app_dst" ]; then
    app_reply="$(ask "Delete $app_dst? [y/N] ")"
    case "$app_reply" in
        [Yy] | [Yy][Ee][Ss])
            app_sudo=""
            [ -w "/Applications" ] || app_sudo="sudo"
            $app_sudo rm -rf "$app_dst"
            echo "✓ Removed $app_dst"
            ;;
        *)
            echo "  Kept $app_dst"
            ;;
    esac
else
    echo "  No app bundle at $app_dst"
fi

# Step 3: the PATH symlink. Only removed when it actually points into the app
# bundle we installed; a real file there is a hand-installed binary the
# installer also refuses to touch, so leave it alone.
echo "Step 3/5: CLI on PATH"
if [ -L "$link_dst" ] && [ "$(readlink "$link_dst")" = "$neru_bin" ]; then
    link_sudo=""
    [ -w "$(dirname "$link_dst")" ] || link_sudo="sudo"
    $link_sudo rm -f "$link_dst"
    echo "✓ Removed $link_dst"
elif [ -e "$link_dst" ] || [ -L "$link_dst" ]; then
    echo "  Kept $link_dst: it is not a symlink to $neru_bin"
    echo "  (a hand-installed binary, or another installer's). Remove it yourself if you want it gone."
else
    echo "  No symlink at $link_dst"
fi

# Step 4: shell completions, from the same per-user locations the installer
# writes to.
echo "Step 4/5: Shell completions"
comp_found=0
for comp in \
    "$HOME/.config/fish/completions/neru.fish" \
    "$HOME/.zsh/completions/_neru" \
    "$HOME/.local/share/bash-completion/completions/neru"; do
    if [ -e "$comp" ]; then
        rm -f "$comp"
        echo "✓ Removed $comp"
        comp_found=1
    fi
done
if [ "$comp_found" -eq 0 ]; then
    echo "  No completions found"
fi

# Step 5: man pages. The installer picks whichever manpath directory was
# writable, so scan the same set and remove only Neru's own pages — never the
# directory, which belongs to the system.
echo "Step 5/5: Man pages"
man_found=0
for man_base in $(manpath 2>/dev/null | tr ':' ' ') "/usr/local/share/man"; do
    man_dir="$man_base/man1"
    [ -d "$man_dir" ] || continue
    # shellcheck disable=SC2086 # deliberate glob expansion
    set -- "$man_dir"/neru*.1
    [ -e "$1" ] || continue
    man_sudo=""
    [ -w "$man_dir" ] || man_sudo="sudo"
    $man_sudo rm -f "$@"
    echo "✓ Removed man pages from $man_dir"
    man_found=1
done
if [ "$man_found" -eq 0 ]; then
    echo "  No man pages found"
fi

# Config and logs. Opt-in via --purge, and still confirmed, because a
# hand-tuned config.toml is the one thing here that cannot be rebuilt.
echo "Config and logs"
if [ "$purge" -eq 0 ]; then
    echo "  Kept. Pass --purge to remove your config and logs too."
else
    note_purge_target "config" "${XDG_CONFIG_HOME:-$HOME/.config}/neru"
    note_purge_target "application data" "$HOME/Library/Application Support/neru"
    note_purge_target "logs" "$HOME/Library/Logs/neru"

    if [ -z "$purge_list" ]; then
        echo "  Nothing to remove."
    else
        echo "  These directories will be permanently deleted:"
        while IFS='|' read -r label dir; do
            [ -n "$dir" ] || continue
            echo "      $dir  ($label)"
        done <<< "$purge_list"

        purge_reply="$(ask "Delete them? [y/N] ")"
        case "$purge_reply" in
            [Yy] | [Yy][Ee][Ss])
                while IFS='|' read -r label dir; do
                    [ -n "$dir" ] || continue
                    rm -rf "$dir"
                    echo "✓ Removed $label: $dir"
                done <<< "$purge_list"
                ;;
            *)
                echo "  Kept your config and logs."
                ;;
        esac
    fi
fi

echo "Neru has been uninstalled."
echo "macOS keeps its Accessibility and Input Monitoring entries; remove them in"
echo "System Settings → Privacy & Security if you are not reinstalling."
