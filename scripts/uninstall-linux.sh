#!/usr/bin/env bash
#
# neru Linux uninstaller. Invoked by `just uninstall`; can also be run
# directly from the repo root. Undoes scripts/install-linux.sh in reverse.
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

bin_dir="$HOME/.local/bin"
neru_bin="$bin_dir/neru"
unit_dir="$HOME/.config/systemd/user"
unit_file="$unit_dir/neru.service"

echo "This removes $neru_bin and the systemd user service, completions and man"
echo "pages that 'just install' created."
if [ "$purge" -eq 0 ]; then
    echo "Your config and logs are kept; pass --purge to remove those too."
fi

# Step 1: the systemd user service and any running instance. Done first:
# Restart=on-failure would otherwise respawn Neru from a binary we then delete.
echo "Step 1/5: systemd user service"
if [ -e "$unit_file" ]; then
    systemctl --user disable --now neru.service >/dev/null 2>&1 || true
    rm -f "$unit_file"
    systemctl --user daemon-reload >/dev/null 2>&1 || true
    echo "✓ Stopped and removed $unit_file"
else
    echo "  No unit at $unit_file"
fi
# A manually started daemon is not known to systemd, so stop it separately.
if pkill -f "$neru_bin launch" >/dev/null 2>&1; then
    echo "✓ Stopped a running Neru"
fi

# Step 2: the binary.
echo "Step 2/5: Binary"
if [ -e "$neru_bin" ]; then
    rm -f "$neru_bin"
    echo "✓ Removed $neru_bin"
else
    echo "  No binary at $neru_bin"
fi

# Step 3: the input group. Deliberately NOT reverted. Membership is a
# system-level change that other tools (other evdev readers, input remappers)
# may equally depend on, and dropping it could break them silently. Print the
# command instead and let the user decide.
echo "Step 3/5: input group (Wayland)"
if id -nG 2>/dev/null | tr ' ' '\n' | grep -qx input; then
    echo "  You are still in the 'input' group; left as-is, since other tools may"
    echo "  rely on it. Remove it yourself if nothing else needs it:"
    echo "      sudo gpasswd -d \$USER input"
else
    echo "  Not in the 'input' group"
fi

# Step 4: shell completions, from the same per-user locations the installer
# writes to.
echo "Step 4/5: Shell completions"
comp_found=0
for comp in \
    "$HOME/.local/share/bash-completion/completions/neru" \
    "$HOME/.zsh/completions/_neru" \
    "$HOME/.config/fish/completions/neru.fish"; do
    if [ -e "$comp" ]; then
        rm -f "$comp"
        echo "✓ Removed $comp"
        comp_found=1
    fi
done
if [ "$comp_found" -eq 0 ]; then
    echo "  No completions found"
fi

# Step 5: man pages. Only Neru's own pages, never the directory itself.
echo "Step 5/5: Man pages"
man_dir="$HOME/.local/share/man/man1"
# shellcheck disable=SC2086 # deliberate glob expansion
set -- "$man_dir"/neru*.1
if [ -e "$1" ]; then
    rm -f "$@"
    echo "✓ Removed man pages from $man_dir"
else
    echo "  No man pages found"
fi

# Config and logs. Opt-in via --purge, and still confirmed, because a
# hand-tuned config.toml is the one thing here that cannot be rebuilt.
echo "Config and logs"
if [ "$purge" -eq 0 ]; then
    echo "  Kept. Pass --purge to remove your config and logs too."
else
    note_purge_target "config" "${XDG_CONFIG_HOME:-$HOME/.config}/neru"
    note_purge_target "application data" "$HOME/.local/share/neru"
    note_purge_target "state and logs" "$HOME/.local/state/neru"

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
