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

# Platform-specific installer. Only macOS is implemented so far. Runs three
# confirmed steps: copy the app bundle to /Applications, register the login
# agent, and link the CLI onto PATH. Run `just bundle` first on macOS.
install:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{ os() }}" in
    macos)
        app_dst="/Applications/Neru.app"
        neru_bin="$app_dst/Contents/MacOS/neru"
        cli="$neru_bin" # how the CLI is referred to in messages; becomes "neru" once linked onto PATH
        # Safeguard: the app must be built first. Check for the actual binary,
        # not just the .app directory, so a partial bundle does not slip through.
        if [ ! -x "build/Neru.app/Contents/MacOS/neru" ]; then
            echo "Neru has not been built yet (build/Neru.app is missing or incomplete)."
            read -r -p "Build it now with 'just bundle'? [y/N] " build_reply
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
        # Record whether Neru is already installed before step 1 possibly
        # overwrites it. The service step uses this to choose between restarting
        # an existing agent and registering a fresh one.
        was_installed=0
        [ -e "$app_dst" ] && was_installed=1
        # Whether step 2 actually (re)loaded the login agent. The agent runs at
        # load, so once it is installed Neru is already running and the final
        # "run now" prompt is skipped.
        service_installed=0

        # Step 1: the app bundle. Overwriting an existing install is the only
        # branch that differs; either way the bundle ends up at $app_dst.
        echo "Step 1/3: App bundle"
        if [ "$was_installed" -eq 1 ]; then
            echo "Neru is already installed at $app_dst."
            read -r -p "Overwrite it with the freshly built bundle? [y/N] " reply
            case "$reply" in
                [Yy] | [Yy][Ee][Ss])
                    rm -rf "$app_dst"
                    cp -r build/Neru.app "$app_dst"
                    echo "✓ Overwrote $app_dst"
                    ;;
                *)
                    echo "Keeping the existing bundle"
                    ;;
            esac
        else
            read -r -p "Copy build/Neru.app → $app_dst? [y/N] " reply
            case "$reply" in
                [Yy] | [Yy][Ee][Ss])
                    cp -r build/Neru.app "$app_dst"
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

        # Step 2: the login agent. Registering it makes Neru start now and at
        # every login (RunAtLoad + KeepAlive).
        echo "Step 2/3: Login agent"
        read -r -p "Register the login agent so Neru starts now and at login? (neru services install) [y/N] " svc_reply
        case "$svc_reply" in
            [Yy] | [Yy][Ee][Ss])
                if [ "$was_installed" -eq 1 ]; then
                    # The agent may already be loaded from a previous install, so
                    # restarting picks up the current binary. If it was never
                    # loaded (app copied by hand, or previously uninstalled),
                    # restart fails, so register it cleanly instead.
                    if "$neru_bin" services restart; then
                        echo "✓ Service restarted onto the current build"
                    else
                        echo "Service was not loaded, registering it..."
                        "$neru_bin" services uninstall || true
                        "$neru_bin" services install
                        echo "✓ Service installed"
                    fi
                else
                    "$neru_bin" services install
                    echo "✓ Service installed"
                fi
                service_installed=1
                ;;
            *)
                echo "Skipped the login agent, so Neru will not start at login."
                ;;
        esac

        # Step 3: put `neru` on PATH by symlinking to the binary inside the app
        # bundle, so the command and the daemon are the exact same executable.
        # Skip the prompt when it is already linked correctly.
        echo "Step 3/3: CLI on PATH"
        link_dst="/usr/local/bin/neru"
        if [ -L "$link_dst" ] && [ "$(readlink "$link_dst")" = "$neru_bin" ]; then
            echo "Already linked: $link_dst → $neru_bin"
            cli="neru"
        else
            read -r -p "Symlink 'neru' onto your PATH at $link_dst → the app binary? [y/N] " link_reply
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

        # If the login agent was installed it is already running Neru. Otherwise
        # nothing is running yet, so offer to launch the daemon once, detached so
        # it survives this shell (logs mirror the launchd agent's paths).
        if [ "$service_installed" -eq 1 ]; then
            echo "Neru runs as a login agent and should be running now."
        else
            read -r -p "Run Neru now? [y/N] " run_reply
            case "$run_reply" in
                [Yy] | [Yy][Ee][Ss])
                    nohup "$neru_bin" launch >/tmp/neru.log 2>/tmp/neru.err.log &
                    echo "✓ Neru started (logs: /tmp/neru.log)"
                    ;;
                *)
                    echo "Not started. Launch it later with: $cli launch"
                    ;;
            esac
        fi
        echo "Manage the service with: $cli services status|stop|restart"
        echo "Grant Accessibility + Input Monitoring in System Settings →"
        echo "Privacy & Security for it to function."
        ;;
    linux)
        # TODO: implement Linux install (copy binary + systemd --user unit).
        echo "just install: not implemented on Linux yet" >&2
        exit 1
        ;;
    windows)
        # TODO: implement Windows install.
        echo "just install: not implemented on Windows yet" >&2
        exit 1
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
