#!/usr/bin/env bash
#
# neru Linux installer. Invoked by `just install`; can also be run
# directly from the repo root. Build the binary first with `just build`.
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
    read -r -p "$1" reply || reply=""
    printf '%s' "$reply"
}


# Minimal Linux install: a user-local binary and an optional systemd
# user service. `neru services` is macOS-only, so the service is a plain
# systemd unit in the shape the Nix modules use. Neru on Linux supports
# X11 and wlroots/KDE Wayland; GNOME Wayland is not supported. See
# docs/LINUX_SETUP.md.
arch="$(uname -m)"
case "$arch" in
    x86_64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
esac
# Prefer a native `just build` binary; fall back to a cross-built one.
bin_src=""
for c in "bin/neru" "bin/neru-linux-$arch"; do
    [ -x "$c" ] && { bin_src="$c"; break; }
done
if [ -z "$bin_src" ]; then
    echo "Neru has not been built yet."
    build_reply="$(ask "Build it now with 'just build'? [y/N] ")"
    case "$build_reply" in
        [Yy] | [Yy][Ee][Ss]) just build; bin_src="bin/neru" ;;
        *) echo "Aborted. Run 'just build' first, then 'just install'." >&2; exit 1 ;;
    esac
fi

# Step 1: the binary. User-local by default, no sudo. Warn if its
# directory is not on PATH.
echo "Step 1/5: Binary"
bin_dir="$HOME/.local/bin"
mkdir -p "$bin_dir"
cp "$bin_src" "$bin_dir/neru"
chmod +x "$bin_dir/neru"
neru_bin="$bin_dir/neru"
echo "✓ Installed $neru_bin"
case ":$PATH:" in
    *":$bin_dir:"*) : ;;
    *) echo "  $bin_dir is not on your PATH; add it in your shell rc." ;;
esac

# Step 2: the systemd user service. Tied to the graphical session so it
# starts with the desktop and restarts on failure.
echo "Step 2/5: systemd user service"
svc_reply="$(ask "Install a systemd user service so Neru starts with your session? [y/N] ")"
case "$svc_reply" in
    [Yy] | [Yy][Ee][Ss])
        unit_dir="$HOME/.config/systemd/user"
        mkdir -p "$unit_dir"
        {
            echo "[Unit]"
            echo "Description=Neru keyboard navigation daemon"
            echo "After=graphical-session.target"
            echo "PartOf=graphical-session.target"
            echo ""
            echo "[Service]"
            echo "ExecStart=$neru_bin launch"
            echo "Restart=on-failure"
            echo "RestartSec=5"
            echo ""
            echo "[Install]"
            echo "WantedBy=graphical-session.target"
        } > "$unit_dir/neru.service"
        # Give the user manager the current session's display variables so
        # the daemon can reach X11/Wayland. (Nice=-10 is intentionally not
        # set: an unprivileged user service cannot lower nice and would
        # fail to start with status 213/NICE.)
        systemctl --user import-environment DISPLAY WAYLAND_DISPLAY XAUTHORITY 2>/dev/null || true
        systemctl --user daemon-reload || true
        if systemctl --user enable --now neru.service; then
            echo "✓ Service enabled and started"
            echo "  Manage with: systemctl --user status|stop|restart neru"
            echo "  Auto-start at login needs your compositor to reach"
            echo "  graphical-session.target; see docs/LINUX_SETUP.md if it does not."
        else
            echo "Wrote the unit but could not start it (no systemd user session?)." >&2
            echo "Enable it later with: systemctl --user enable --now neru" >&2
        fi
        ;;
    *)
        echo "Skipped the service. Run manually with: neru launch"
        ;;
esac

# Step 3: input group. Wayland reads keyboard events via evdev, which
# needs membership in 'input'; X11 does not.
echo "Step 3/5: input group (Wayland)"
if id -nG 2>/dev/null | tr ' ' '\n' | grep -qx input; then
    echo "Already in the 'input' group."
else
    grp_reply="$(ask "Add yourself to the 'input' group for Wayland keyboard capture? [y/N] ")"
    case "$grp_reply" in
        [Yy] | [Yy][Ee][Ss])
            if sudo usermod -aG input "$USER"; then
                echo "✓ Added to 'input'. Log out and back in for it to take effect."
            else
                echo "Could not modify groups; run later: sudo usermod -aG input \$USER" >&2
            fi
            ;;
        *)
            echo "Skipped. On Wayland, add it later with: sudo usermod -aG input \$USER"
            ;;
    esac
fi

# Step 4: shell completions, to each installed shell's per-user location.
echo "Step 4/5: Shell completions"
comp_reply="$(ask "Install shell completions for the shells you have? [y/N] ")"
case "$comp_reply" in
    [Yy] | [Yy][Ee][Ss])
        if command -v bash >/dev/null 2>&1; then
            mkdir -p "$HOME/.local/share/bash-completion/completions"
            "$neru_bin" completion bash > "$HOME/.local/share/bash-completion/completions/neru"
            echo "✓ bash → ~/.local/share/bash-completion/completions/neru"
        fi
        if command -v zsh >/dev/null 2>&1; then
            mkdir -p "$HOME/.zsh/completions"
            "$neru_bin" completion zsh > "$HOME/.zsh/completions/_neru"
            echo "✓ zsh  → ~/.zsh/completions/_neru"
            echo "       add to ~/.zshrc before compinit: fpath=(~/.zsh/completions \$fpath)"
        fi
        if command -v fish >/dev/null 2>&1; then
            mkdir -p "$HOME/.config/fish/completions"
            "$neru_bin" completion fish > "$HOME/.config/fish/completions/neru.fish"
            echo "✓ fish → ~/.config/fish/completions/neru.fish"
        fi
        ;;
    *)
        echo "Skipped completions."
        ;;
esac

# Step 5: man pages, into the XDG user man directory.
echo "Step 5/5: Man pages"
man_reply="$(ask "Generate and install man pages to ~/.local/share/man? [y/N] ")"
case "$man_reply" in
    [Yy] | [Yy][Ee][Ss])
        just genman build/man >/dev/null
        mkdir -p "$HOME/.local/share/man/man1"
        cp build/man/*.1 "$HOME/.local/share/man/man1/"
        echo "✓ Installed man pages to ~/.local/share/man/man1"
        ;;
    *)
        echo "Skipped man pages."
        ;;
esac

echo "On Wayland, bind a global hotkey in your compositor to run Neru's"
echo "modes; on X11 they come from your config.toml. See docs/LINUX_SETUP.md."
