#!/usr/bin/env bash
#
# neru macOS installer. Invoked by `just install`; can also be run
# directly from the repo root. Build the app first with `just bundle`.
set -euo pipefail

# Run from the repo root so build/, bin/, and `just` resolve.
cd "$(dirname "$0")/.."

app_dst="/Applications/Neru.app"
neru_bin="$app_dst/Contents/MacOS/neru"
cli="$neru_bin" # how the CLI is referred to in messages; becomes "neru" once linked onto PATH
# Copy the freshly built bundle into place. Uses sudo when the target
# directory is not writable (Step 3 does the same for the symlink) and
# stages the new bundle fully before moving the old one aside, so a
# failed copy never leaves you with no app installed.
install_bundle() {
    local dst parent staging previous sudo_maybe
    dst="$1"
    parent="$(dirname "$dst")"
    sudo_maybe=""
    [ -w "$parent" ] || sudo_maybe="sudo"
    staging="$parent/.neru-install.$$"
    previous="$parent/.neru-previous.$$"
    $sudo_maybe rm -rf "$staging" "$previous"
    $sudo_maybe cp -r build/Neru.app "$staging"
    [ -e "$dst" ] && $sudo_maybe mv "$dst" "$previous"
    $sudo_maybe mv "$staging" "$dst"
    $sudo_maybe rm -rf "$previous"
}
# Stop any running Neru so a freshly registered agent is the only
# instance. A launchd agent must be booted out (KeepAlive would relaunch
# a plain stop), and a detached "run now" instance from an earlier run is
# not known to launchd, so kill it too. Either would otherwise keep the
# IPC socket, and a newly loaded agent then sees an instance already up
# and exits 0 (root.go), which KeepAlive respawns into a loop.
stop_running_neru() {
    "$neru_bin" services uninstall >/dev/null 2>&1 || true
    pkill -f "$app_dst/Contents/MacOS/neru launch" >/dev/null 2>&1 || true
}
# Refuse to fight another installer, the same stance `neru services
# install` takes. First conflict: Homebrew. Its cask installs Neru.app to
# /Applications (the exact path this recipe writes) and symlinks `neru`
# onto PATH, so overwriting here clobbers files brew tracks. On disk that
# app is indistinguishable from a source install, so asking brew is the
# only reliable signal that /Applications/Neru.app belongs to it.
if command -v brew >/dev/null 2>&1 && brew list --cask 2>/dev/null | grep -qxE 'neru|neru-nightly'; then
    echo "Neru is already installed with Homebrew." >&2
    echo "Update it with:  brew upgrade --cask y3owk1n/tap/neru" >&2
    echo "Or remove it first to switch to a source install:" >&2
    echo "                 brew uninstall --cask y3owk1n/tap/neru" >&2
    exit 1
fi
# Second conflict: a login agent that already runs a different neru.
# nix-darwin (org.nixos.neru), home-manager (org.nix-community.home.neru)
# and a prior manual install (com.y3owk1n.neru) each drop a plist under
# LaunchAgents/LaunchDaemons, under labels this recipe should not assume.
# The Nix packages keep their app inside the store, not /Applications, so
# the app checks above never see them. Enumerate every neru agent instead:
# ours launches $neru_bin inside /Applications, so any agent whose
# ProgramArguments[0] is something else (a nix-darwin agent runs it via
# /bin/sh, home-manager runs the store binary directly) belongs to another
# installer. Point at the right removal path using the plist label.
for plist in \
    "$HOME/Library/LaunchAgents/"*neru*.plist \
    "/Library/LaunchAgents/"*neru*.plist \
    "/Library/LaunchDaemons/"*neru*.plist; do
    [ -e "$plist" ] || continue
    agent_prog="$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments:0' "$plist" 2>/dev/null || true)"
    [ -n "$agent_prog" ] && [ "$agent_prog" != "$neru_bin" ] || continue
    case "$(basename "$plist")" in
        org.nixos.*|org.nix-community.*) agent_hint="Remove Neru from your nix-darwin or home-manager config and rebuild." ;;
        *) agent_hint="Remove it with 'neru services uninstall'." ;;
    esac
    echo "A Neru login agent is already registered by another installer:" >&2
    echo "    plist:    $plist" >&2
    echo "    launches: $agent_prog" >&2
    echo "$agent_hint Then retry." >&2
    exit 1
done
# Safeguard: the app must be built first. Check for the actual binary,
# not just the .app directory, so a partial bundle does not slip through.
if [ ! -x "build/Neru.app/Contents/MacOS/neru" ]; then
    echo "Neru has not been built yet (build/Neru.app is missing or incomplete)."
    read -r -p "Build it now with 'just bundle'? [y/N] " build_reply || build_reply=""
    case "$build_reply" in
        [Yy] | [Yy][Ee][Ss])
            just bundle
            ;;
        *)
            echo "Aborted. Run 'just bundle' first, then 'just install'." >&2
            exit 1
            ;;
    esac
fi
# Whether step 2 actually (re)loaded the login agent. The agent runs at
# load, so once it is installed Neru is already running and the final
# "run now" prompt is skipped.
service_installed=0
overwrote=0

# Step 1: the app bundle. Before overwriting, stop any running Neru (a
# launchd agent or a detached "run now" instance) so the binary is not
# swapped under a live process. Step 2 re-registers it.
echo "Step 1/5: App bundle"
if [ -e "$app_dst" ]; then
    echo "Neru is already installed at $app_dst."
    read -r -p "Overwrite it with the freshly built bundle? [y/N] " reply || reply=""
    case "$reply" in
        [Yy] | [Yy][Ee][Ss])
            echo "Stopping any running Neru before replacing the app..."
            stop_running_neru
            install_bundle "$app_dst"
            overwrote=1
            echo "✓ Overwrote $app_dst"
            ;;
        *)
            echo "Keeping the existing bundle"
            ;;
    esac
else
    read -r -p "Copy build/Neru.app → $app_dst? [y/N] " reply || reply=""
    case "$reply" in
        [Yy] | [Yy][Ee][Ss])
            install_bundle "$app_dst"
            echo "✓ Installed $app_dst"
            ;;
        *)
            # Without the bundle in place the later steps have nothing to
            # register or link, so there is no point continuing.
            echo "Aborted. Nothing installed." >&2
            exit 1
            ;;
    esac
fi

# Step 2: the login agent. Stop any running instance and clear a leftover
# plist (both done by stop_running_neru, which uninstalls), then install
# so registration is clean from any starting state and the new agent is
# the only instance. Installing points the agent at the current binary
# and starts it (RunAtLoad + KeepAlive).
echo "Step 2/5: Login agent"
read -r -p "Register the login agent so Neru starts now and at login? (neru services install) [y/N] " svc_reply || svc_reply=""
case "$svc_reply" in
    [Yy] | [Yy][Ee][Ss])
        stop_running_neru
        if "$neru_bin" services install; then
            echo "✓ Service installed"
            service_installed=1
        else
            echo "Service install failed; continuing. Register later with 'neru services install'." >&2
        fi
        ;;
    *)
        echo "Skipped the login agent, so Neru will not start at login."
        ;;
esac

# Step 3: put `neru` on PATH by symlinking to the binary inside the app
# bundle, so the command and the daemon are the exact same executable.
# Skip the prompt when it is already linked correctly, and warn before
# replacing a real file there (e.g. a hand-copied binary from the docs'
# source-install steps) instead of clobbering it silently.
echo "Step 3/5: CLI on PATH"
link_dst="/usr/local/bin/neru"
if [ -L "$link_dst" ] && [ "$(readlink "$link_dst")" = "$neru_bin" ]; then
    echo "Already linked: $link_dst → $neru_bin"
    cli="neru"
else
    if [ -e "$link_dst" ] && [ ! -L "$link_dst" ]; then
        echo "$link_dst already exists and is not a symlink (a hand-installed neru binary?)."
        link_prompt="Replace it with a symlink to the app binary? [y/N] "
    else
        link_prompt="Symlink 'neru' onto your PATH at $link_dst → the app binary? [y/N] "
    fi
    read -r -p "$link_prompt" link_reply || link_reply=""
    case "$link_reply" in
        [Yy] | [Yy][Ee][Ss])
            link_dir="$(dirname "$link_dst")"
            if [ -w "$link_dir" ]; then
                ln -sf "$neru_bin" "$link_dst"
            else
                echo "$link_dir is not writable, creating the link with sudo..."
                sudo mkdir -p "$link_dir"
                sudo ln -sf "$neru_bin" "$link_dst"
            fi
            echo "✓ Linked 'neru' → $neru_bin"
            cli="neru"
            ;;
        *)
            echo "Skipped the symlink. Link it later with:"
            echo "    sudo ln -sf $neru_bin $link_dst"
            ;;
    esac
fi

# Step 4: shell completions. Optional; the binary generates them and each
# goes to the installed shell's per-user location. zsh needs its directory
# on fpath, so print the line to add when zsh is present.
echo "Step 4/5: Shell completions"
read -r -p "Install shell completions for the shells you have? [y/N] " comp_reply || comp_reply=""
case "$comp_reply" in
    [Yy] | [Yy][Ee][Ss])
        if command -v fish >/dev/null 2>&1; then
            mkdir -p "$HOME/.config/fish/completions"
            "$neru_bin" completion fish > "$HOME/.config/fish/completions/neru.fish"
            echo "✓ fish → ~/.config/fish/completions/neru.fish"
        fi
        if command -v zsh >/dev/null 2>&1; then
            mkdir -p "$HOME/.zsh/completions"
            "$neru_bin" completion zsh > "$HOME/.zsh/completions/_neru"
            echo "✓ zsh  → ~/.zsh/completions/_neru"
            echo "       if completions do not load, add to ~/.zshrc before compinit:"
            echo "       fpath=(~/.zsh/completions \$fpath)"
        fi
        if command -v bash >/dev/null 2>&1; then
            mkdir -p "$HOME/.local/share/bash-completion/completions"
            "$neru_bin" completion bash > "$HOME/.local/share/bash-completion/completions/neru"
            echo "✓ bash → ~/.local/share/bash-completion/completions/neru (needs bash-completion v2)"
        fi
        ;;
    *)
        echo "Skipped completions. Generate later with: $cli completion <bash|zsh|fish>"
        ;;
esac

# Step 5: man pages. Optional; generated from source, then installed into
# a writable directory already on the manpath (so `man neru` finds them),
# falling back to /usr/local/share/man when none is writable.
echo "Step 5/5: Man pages"
read -r -p "Generate and install man pages? [y/N] " man_reply || man_reply=""
case "$man_reply" in
    [Yy] | [Yy][Ee][Ss])
        just genman build/man >/dev/null
        man_base="/usr/local/share/man"
        for d in $(manpath 2>/dev/null | tr ':' ' '); do
            case "$d" in
                */share/man) if [ -w "$d" ]; then man_base="$d"; break; fi ;;
            esac
        done
        man_sudo=""
        [ -w "$man_base" ] || man_sudo="sudo"
        $man_sudo mkdir -p "$man_base/man1"
        $man_sudo cp build/man/*.1 "$man_base/man1/"
        echo "✓ Installed man pages to $man_base/man1"
        ;;
    *)
        echo "Skipped man pages. Generate later with: just genman"
        ;;
esac

# If the login agent was installed it is already running Neru. Otherwise
# nothing is running yet, so offer to launch the daemon once, detached so
# it survives this shell (logs mirror the launchd agent's paths).
if [ "$service_installed" -eq 1 ]; then
    echo "Neru runs as a login agent and should be running now."
else
    read -r -p "Run Neru now? [y/N] " run_reply || run_reply=""
    case "$run_reply" in
        [Yy] | [Yy][Ee][Ss])
            # `neru status` exits non-zero unless a daemon is already up,
            # and `launch` itself refuses to start a second instance, so
            # only launch (and only say so) when nothing is running.
            if "$neru_bin" status >/dev/null 2>&1; then
                echo "Neru is already running."
            else
                nohup "$neru_bin" launch >/tmp/neru.log 2>/tmp/neru.err.log &
                echo "Launching Neru (logs: /tmp/neru.log)."
            fi
            ;;
        *)
            echo "Not started. Launch it later with: $cli launch"
            ;;
    esac
fi
echo "Manage the service with: $cli services status|stop|restart"
echo "Grant Accessibility + Input Monitoring in System Settings →"
echo "Privacy & Security for it to function."
if [ "$overwrote" -eq 1 ]; then
    echo "You overwrote an existing app, which changes its code signature,"
    echo "so macOS may drop the Accessibility grant. Neru will ask again;"
    echo "re-approve it if navigation stops working."
fi
