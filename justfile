# Neru Build System
# Version information (can be overridden)

VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
GIT_COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
BUILD_DATE := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

# macOS deployment target (used in CGO CFLAGS and as an env var for clang/ld).
MACOSX_DEPLOYMENT_TARGET := "14.0"

# Ldflags for version injection; Windows uses GUI subsystem (no console window).

LDFLAGS := "-s -w -X github.com/y3owk1n/neru/internal/cli.Version=" + VERSION + " -X github.com/y3owk1n/neru/internal/cli.GitCommit=" + GIT_COMMIT + " -X github.com/y3owk1n/neru/internal/cli.BuildDate=" + BUILD_DATE
WIN_LDFLAGS := "-H windowsgui -s -w -X github.com/y3owk1n/neru/internal/cli.Version=" + VERSION + " -X github.com/y3owk1n/neru/internal/cli.GitCommit=" + GIT_COMMIT + " -X github.com/y3owk1n/neru/internal/cli.BuildDate=" + BUILD_DATE

# Default build
default: build

# Build the application (development)
# Uses CGO on macOS (required for Objective-C bridge) and Linux (required for

# X11/Wayland native backends). Windows currently builds with CGO disabled.
build:
    @echo "Building Neru..."
    @echo "Version: {{ VERSION }}"
    {{ if os() == "windows" { "CGO_ENABLED=0" } else { "CGO_ENABLED=1" } }} go build -ldflags="{{ if os() == "windows" { WIN_LDFLAGS } else { LDFLAGS } }}" -o bin/neru{{ if os() == "windows" { ".exe" } else { "" } }} ./cmd/neru
    @echo "✓ Build complete: bin/neru"

# Build a Linux binary. Must run on a Linux host (CGO required for native backends).
build-linux ARCH="amd64":
    @echo "Building Neru for linux/{{ ARCH }}..."
    mkdir -p bin
    CGO_ENABLED=1 GOOS=linux GOARCH={{ ARCH }} go build -ldflags="{{ LDFLAGS }}" -o bin/neru-linux-{{ ARCH }} ./cmd/neru
    @echo "✓ Build complete: bin/neru-linux-{{ ARCH }}"

# Generate Windows resource files (.syso) for embedding the app icon and manifest.
#
# Must be run before go build on/for Windows.  The .syso files are written into
# cmd/neru/ so go build picks them up automatically.
generate-winres ARCH="amd64":
    #!/usr/bin/env bash
    set -euo pipefail
    cd cmd/neru
    echo "Generating Windows resources for {{ ARCH }}..."
    go run github.com/tc-hib/go-winres@v0.3.3 simply \
        --icon ../../assets/neru-appicon.png \
        --manifest gui \
        --arch {{ ARCH }} \
        --file-description "Neru keyboard-driven navigation tool" \
        --product-name "Neru" \
        --original-filename "neru.exe"
    echo "✓ Windows resources generated"

# Build a Windows binary from any host.
# This produces a binary with grid, recursive grid, scroll, global hotkeys,
# mouse injection, IPC, and initial UIA accessibility.
build-windows ARCH="amd64":
    @echo "Building Neru for windows/{{ ARCH }}..."
    mkdir -p bin
    just generate-winres {{ ARCH }}
    CGO_ENABLED=0 GOOS=windows GOARCH={{ ARCH }} go build -ldflags="{{ WIN_LDFLAGS }}" -o bin/neru-windows-{{ ARCH }}.exe ./cmd/neru
    @echo "✓ Build complete: bin/neru-windows-{{ ARCH }}.exe"

# Build a macOS binary for the current host.

# macOS requires CGO because the native bridge is part of the real product.
build-darwin:
    @echo "Building Neru for macOS..."
    mkdir -p bin
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -o bin/neru-darwin ./cmd/neru
    @echo "✓ Build complete: bin/neru-darwin"

# Build with optimizations for release
release:
    @echo "Building release version..."
    @echo "Version: {{ VERSION }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Release build complete: bin/neru"

# Build with custom version
build-version VERSION_OVERRIDE:
    @echo "Building Neru with custom version..."
    CGO_ENABLED=1 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Build complete: bin/neru (version: {{ VERSION_OVERRIDE }})"

# Build a macOS release artifact for CI on a native macOS host.

# Usage: just release-ci-darwin arm64 v1.2.3
release-ci-darwin ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (darwin/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    CGO_ENABLED=1 GOOS=darwin GOARCH={{ ARCH }} MACOSX_DEPLOYMENT_TARGET={{ MACOSX_DEPLOYMENT_TARGET }} CGO_LDFLAGS_ALLOW='-Wl,.*' CGO_LDFLAGS='-Wl,-macosx_version_min,{{ MACOSX_DEPLOYMENT_TARGET }}' go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-darwin-{{ ARCH }} ./cmd/neru
    @echo "✓ Release artifact for darwin/{{ ARCH }} built successfully"

# Build a Linux release artifact for CI on a native Linux host.

# Usage: just release-ci-linux amd64 v1.2.3
release-ci-linux ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (linux/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    CGO_ENABLED=1 GOOS=linux GOARCH={{ ARCH }} go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-linux-{{ ARCH }} ./cmd/neru
    @echo "✓ Release artifact for linux/{{ ARCH }} built successfully"

# Build a Windows release artifact for CI.

# Usage: just release-ci-windows amd64 v1.2.3
release-ci-windows ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (windows/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    just generate-winres {{ ARCH }}
    CGO_ENABLED=0 GOOS=windows GOARCH={{ ARCH }} go build -ldflags="-H windowsgui -s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-windows-{{ ARCH }}.exe ./cmd/neru
    @echo "✓ Release artifact for windows/{{ ARCH }} built successfully"

# Bundle the application
bundle: release
    @echo "Bundling Neru..."
    mkdir -p build/Neru.app/Contents/{MacOS,Resources}

    cp -r bin/neru build/Neru.app/Contents/MacOS/neru

    cp resources/icon.icns build/Neru.app/Contents/Resources/icon.icns

    sed "s/VERSION/{{ VERSION }}/g" resources/Info.plist.template > build/Neru.app/Contents/Info.plist

    codesign --force --deep --sign - build/Neru.app

    @echo "✓ Bundle complete: build/Neru.app"

# Platform-specific installer. Only macOS is implemented so far. Runs five
# confirmed steps: copy the app bundle to /Applications, register the login
# agent, link the CLI onto PATH, install shell completions, and install man
# pages. Run `just bundle` first on macOS.
install:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{ os() }}" in
    macos)
        app_dst="/Applications/Neru.app"
        neru_bin="$app_dst/Contents/MacOS/neru"
        cli="$neru_bin" # how the CLI is referred to in messages; becomes "neru" once linked onto PATH
        service_label="com.y3owk1n.neru"
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
        ;;
    linux)
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
            read -r -p "Build it now with 'just build'? [y/N] " build_reply || build_reply=""
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
        read -r -p "Install a systemd user service so Neru starts with your session? [y/N] " svc_reply || svc_reply=""
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
                    echo "Nice=-10"
                    echo ""
                    echo "[Install]"
                    echo "WantedBy=graphical-session.target"
                } > "$unit_dir/neru.service"
                systemctl --user daemon-reload || true
                if systemctl --user enable --now neru.service; then
                    echo "✓ Service installed and started"
                    echo "  Manage with: systemctl --user status|stop|restart neru"
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
            read -r -p "Add yourself to the 'input' group for Wayland keyboard capture? [y/N] " grp_reply || grp_reply=""
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
        read -r -p "Install shell completions for the shells you have? [y/N] " comp_reply || comp_reply=""
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
        read -r -p "Generate and install man pages to ~/.local/share/man? [y/N] " man_reply || man_reply=""
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
        ;;
    windows)
        # Minimal Windows install: a per-user binary plus optional autostart via
        # the registry Run key. Windows has no launchd or systemd and `neru
        # services` is macOS-only, so autostart is authored here, not by Neru.
        # This recipe needs a bash (e.g. Git Bash) to run. Windows support is
        # partial (grid, recursive grid, scroll, hotkeys, mouse injection, UIA);
        # see docs/CROSS_PLATFORM.md.
        arch="$(uname -m)"
        case "$arch" in
            aarch64 | arm64) arch=arm64 ;;
            *) arch=amd64 ;;
        esac
        exe_src="bin/neru-windows-$arch.exe"
        if [ ! -f "$exe_src" ]; then
            echo "Neru has not been built for Windows yet ($exe_src is missing)."
            read -r -p "Build it now with 'just build-windows $arch'? [y/N] " build_reply || build_reply=""
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
        read -r -p "Start Neru at login (a registry Run entry)? [y/N] " run_reply || run_reply=""
        case "$run_reply" in
            [Yy] | [Yy][Ee][Ss])
                win_exe="$(cygpath -w "$dst_dir/neru.exe" 2>/dev/null || echo "$dst_dir/neru.exe")"
                if reg add "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" /v Neru /t REG_SZ /d "\"$win_exe\" launch" /f >/dev/null 2>&1; then
                    echo "✓ Neru will start at login"
                    echo "  Remove later with: reg delete \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\" /v Neru /f"
                else
                    echo "Could not write the Run key; add autostart manually." >&2
                fi
                ;;
            *)
                echo "Skipped autostart. Start Neru with: neru.exe launch"
                ;;
        esac
        echo "Windows support is partial; see docs/CROSS_PLATFORM.md."
        ;;
    *)
        echo "just install: unsupported platform '{{ os() }}'" >&2
        exit 1
        ;;
    esac

# Run tests

# Run all tests (unit + integration)
test: test-unit test-integration
    @echo "Running all tests..."

# Run unit tests
test-unit:
    @echo "Running unit tests..."
    go test -v ./...

# Run a small cross-platform-safe test slice that avoids most native platform
# integration requirements. Useful as a fast confidence check before or during

# Linux/Windows work.
test-foundation:
    @echo "Running cross-platform foundation tests..."
    go test ./internal/config ./internal/core/domain/action ./internal/core/ports
    @echo "✓ Cross-platform foundation tests passed"

# Run integration tests
test-integration:
    @echo "Running integration tests..."
    go test -tags=integration -v ./...

# Run with race detection
test-race: test-race-unit test-race-integration
    @echo "Running tests with race detection..."

# Run unit tests with race detection
test-race-unit:
    @echo "Running unit tests with race detection..."
    go test -race -v ./...

# Run integration tests with race detection
test-race-integration:
    @echo "Running integration tests with race detection..."
    go test -tags=integration -race -v ./...

test-all: test test-race

# Check if files are formatted correctly
fmt-check:
    #!/usr/bin/env bash
    echo "Not checking formatting for go files... It will be checked in lint"
    echo "Checking Objective-C file formatting..."
    EXIT_CODE=0
    while IFS= read -r -d '' file; do
        case "$file" in *.c) af=file.c;; *) af=file.m;; esac
        OUTPUT=$(clang-format --dry-run -Werror --style=file --assume-filename="$af" "$file" 2>&1)
        RESULT=$?
        # Filter out the "does not support C++" warnings
        FILTERED=$(echo "$OUTPUT" | grep -v "Configuration file(s) do(es) not support C++")
        if [ -n "$FILTERED" ]; then
            echo "$FILTERED"
        fi
        if [ $RESULT -ne 0 ] && [ -n "$FILTERED" ]; then
            EXIT_CODE=1
        fi
    done < <(find internal/core/infra \( -name "*.h" -o -name "*.m" -o -name "*.c" \) -print0)
    if [ $EXIT_CODE -ne 0 ]; then
        echo "Some Objective-C files are not properly formatted. Run 'just fmt' to fix them."
        exit 1
    fi
    echo "✓ All Objective-C files are properly formatted"

# Generate man pages
genman OUTPUT_DIR="build/man":
    @echo "Generating man pages..."
    go run ./cmd/genman {{ OUTPUT_DIR }}
    @echo "✓ Man pages generated in {{ OUTPUT_DIR }}/"

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/
    rm -rf build/
    rm -rf *.app
    rm -f cmd/neru/rsrc_windows_*.syso
    @echo "✓ Clean complete"

# Format code
fmt:
    @echo "Formatting Go files..."
    golangci-lint fmt
    golangci-lint run --fix
    @echo "Formatting Objective-C files..."
    @find internal/core/infra \( -name "*.h" -o -name "*.m" -o -name "*.c" \) -exec sh -c 'case "$1" in *.c) af=file.c;; *) af=file.m;; esac; clang-format -i --style=file --assume-filename="$af" "$1"' _ {} \;
    @echo "✓ Format complete"

# Lint code
lint:
    @echo "Linting code..."
    golangci-lint run
    @echo "Linting Objective-C files..."
    echo "Skipping Objective-C linting due to header issues"
    @echo "✓ Lint complete"

# Vet
vet:
    @echo "Vetting code..."
    go vet ./...
    @echo "✓ Vet complete"

# Download dependencies
deps:
    @echo "Downloading dependencies..."
    go mod download
    go mod tidy
    @echo "✓ Dependencies updated"

# Verify dependencies
verify:
    @echo "Verifying dependencies..."
    go mod verify
    @echo "✓ Dependencies verified"

# Generate icon.icns from a source PNG (e.g., just generate-icns icon-1024.png)
generate-icns SOURCE:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating icon.icns from {{ SOURCE }}..."
    ICONSET="icon.iconset"
    mkdir -p "$ICONSET"
    sips -z 16 16     "{{ SOURCE }}" --out "$ICONSET/icon_16x16.png"      >/dev/null
    sips -z 32 32     "{{ SOURCE }}" --out "$ICONSET/icon_16x16@2x.png"   >/dev/null
    sips -z 32 32     "{{ SOURCE }}" --out "$ICONSET/icon_32x32.png"      >/dev/null
    sips -z 64 64     "{{ SOURCE }}" --out "$ICONSET/icon_32x32@2x.png"   >/dev/null
    sips -z 128 128   "{{ SOURCE }}" --out "$ICONSET/icon_128x128.png"    >/dev/null
    sips -z 256 256   "{{ SOURCE }}" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
    sips -z 256 256   "{{ SOURCE }}" --out "$ICONSET/icon_256x256.png"    >/dev/null
    sips -z 512 512   "{{ SOURCE }}" --out "$ICONSET/icon_256x256@2x.png" >/dev/null
    sips -z 512 512   "{{ SOURCE }}" --out "$ICONSET/icon_512x512.png"    >/dev/null
    sips -z 1024 1024 "{{ SOURCE }}" --out "$ICONSET/icon_512x512@2x.png" >/dev/null
    iconutil -c icns "$ICONSET" -o resources/icon.icns
    rm -rf "$ICONSET"
    echo "✓ Generated resources/icon.icns"

# Generate systray tray icon PNGs from source PNGs
# Resizes to 44×44 pixels (22pt @2x retina for macOS menu bar)

# Usage: just generate-tray-icons active.png disabled.png
generate-tray-icons ACTIVE DISABLED:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating tray icons..."
    TRAY_DIR="internal/app/components/systray/resources"
    mkdir -p "$TRAY_DIR"
    sips -z 44 44 "{{ ACTIVE }}"   --out "$TRAY_DIR/tray-icon.png"          >/dev/null
    sips -z 44 44 "{{ DISABLED }}" --out "$TRAY_DIR/tray-icon-disabled.png"  >/dev/null
    echo "✓ Generated $TRAY_DIR/tray-icon.png (44×44, 22pt @2x)"
    echo "✓ Generated $TRAY_DIR/tray-icon-disabled.png (44×44, 22pt @2x)"

# Generate all icons from source PNGs

# Usage: just generate-icons app-icon.png tray-active.png tray-disabled.png
generate-icons APP_ICON TRAY_ACTIVE TRAY_DISABLED:
    just generate-icns {{ APP_ICON }}
    just generate-tray-icons {{ TRAY_ACTIVE }} {{ TRAY_DISABLED }}
    @echo "✓ All icons generated"

# =============================================================================
# Wayland Protocol Generation
# =============================================================================
# Downloads Wayland protocol XMLs from upstream repositories and generates
# wayland-scanner header/private code files.
#
# Protocols are sourced from:
# - wlroots: https://gitlab.freedesktop.org/wlroots/wlroots/-/tree/master/protocol
# - wlr-protocols: https://gitlab.freedesktop.org/wlroots/wlr-protocols/-/tree/master/unstable
# - wayland-protocols: https://gitlab.freedesktop.org/wayland/wayland-protocols/-/tree/master

PROTOCOL_DIR := "protocol"
WLR_PROTOCOL_DIR := "internal/core/infra/platform/linux/wlr_protocol"

# Download Wayland protocol XMLs from canonical upstream repositories
fetch-protocols:
    @echo "Fetching Wayland protocol XMLs..."
    mkdir -p {{ PROTOCOL_DIR }}
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlroots/-/raw/master/protocol/wlr-layer-shell-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/wlr-layer-shell-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlroots/-/raw/master/protocol/virtual-keyboard-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/virtual-keyboard-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlr-protocols/-/raw/master/unstable/wlr-virtual-pointer-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/wlr-virtual-pointer-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/unstable/xdg-output/xdg-output-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/xdg-output-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/stable/xdg-shell/xdg-shell.xml" -o {{ PROTOCOL_DIR }}/xdg-shell.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/unstable/relative-pointer/relative-pointer-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/relative-pointer-unstable-v1.xml
    @echo "✓ Protocol XMLs downloaded to {{ PROTOCOL_DIR }}/"

# Generate wayland-scanner files from XMLs
generate-protocols:
    @echo "Generating wayland-scanner protocol files..."
    mkdir -p {{ WLR_PROTOCOL_DIR }}

    # xdg-shell (stable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/xdg-shell.xml > {{ WLR_PROTOCOL_DIR }}/xdg-shell.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/xdg-shell.xml > {{ WLR_PROTOCOL_DIR }}/xdg-shell.c

    # xdg-output (unstable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/xdg-output-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/xdg-output.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/xdg-output-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/xdg-output.c

    # wlr-layer-shell (unstable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/wlr-layer-shell-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/layer-shell.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/wlr-layer-shell-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/layer-shell.c

    # wlr-virtual-pointer (unstable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/wlr-virtual-pointer-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/virtual-pointer.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/wlr-virtual-pointer-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/virtual-pointer.c

    # virtual-keyboard (unstable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/virtual-keyboard-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/virtual-keyboard.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/virtual-keyboard-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/virtual-keyboard.c

    # relative-pointer (unstable)
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/relative-pointer-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/relative-pointer-unstable-v1.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/relative-pointer-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/relative-pointer-unstable-v1.c
    @echo "✓ Protocol files generated in {{ WLR_PROTOCOL_DIR }}/"

# Download and generate all Wayland protocols
generate-all-protocols: fetch-protocols generate-protocols
    @echo "✓ All Wayland protocols downloaded and generated"
