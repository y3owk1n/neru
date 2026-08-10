# Neru Build System
# Version information (can be overridden)

VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
GIT_COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
BUILD_DATE := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

# macOS deployment target (used in CGO CFLAGS and as an env var for clang/ld).
MACOSX_DEPLOYMENT_TARGET := "14.0"

# Ldflags for version injection. Windows deliberately builds for the console
# subsystem (no -H windowsgui) so a single neru.exe serves as both the CLI and
# the daemon: subcommands can write to the terminal they were typed into, and
# the daemon frees the console the shell allocated for it (see
# internal/cli/console_windows.go).

LDFLAGS := "-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version=" + VERSION + " -X github.com/y3owk1n/neru/internal/buildinfo.GitCommit=" + GIT_COMMIT + " -X github.com/y3owk1n/neru/internal/buildinfo.BuildDate=" + BUILD_DATE

# Default build
[doc('Build the development binary; what plain `just` with no recipe runs.')]
default: build

# Build the application (development)
# Uses CGO on macOS (required for Objective-C bridge) and Linux (required for

# X11/Wayland native backends). Windows currently builds with CGO disabled.
[doc('Build the development binary into bin/neru, CGO on except on Windows.')]
build:
    @echo "Building Neru..."
    @echo "Version: {{ VERSION }}"
    {{ if os() == "windows" { "CGO_ENABLED=0" } else { "CGO_ENABLED=1" } }} go build -ldflags="{{ LDFLAGS }}" -o bin/neru{{ if os() == "windows" { ".exe" } else { "" } }} ./cmd/neru
    @echo "✓ Build complete: bin/neru"

# Build a Linux binary. Needs a C compiler that targets linux/ARCH.
#
# The Linux build turns CGO on for the X11 and Wayland backends, which also
# drags in Go's own cgo runtime (linux_syscall.c, gcc_amd64.S). A macOS clang
# compiles those against the macOS SDK and dies in a wall of assembler errors,
# so this recipe asks the compiler what it targets before starting, and names
# the supported alternatives when the answer is not Linux. Point CC at a
# Linux-targeting cross compiler and the build runs.
#
# Two deliberate soft spots: the arch half of the triple is only checked for the
# arches Neru ships (amd64, arm64), and a compiler that cannot answer
# -dumpmachine gets the benefit of the doubt. Both fail open, so the guard never
# blocks a build that would have worked.
[doc('Build a Linux binary with CGO; needs a Linux-targeting C compiler.')]
build-linux ARCH="amd64":
    #!/usr/bin/env bash
    set -euo pipefail
    # CC is what cgo actually runs, and it may carry flags ("clang --target=...",
    # "ccache gcc"), so it stays unquoted on the -dumpmachine call.
    cc=${CC:-$(go env CC)}
    triple=$($cc -dumpmachine 2>/dev/null || true)
    case "{{ ARCH }}" in
        amd64) linux_triple='^(x86_64|amd64)-.*linux' ;;
        arm64) linux_triple='^(aarch64|arm64)-.*linux' ;;
        *) linux_triple='linux' ;;
    esac
    if [[ -n "$triple" ]] && ! echo "$triple" | grep -Eq "$linux_triple"; then
        echo "error: cannot build linux/{{ ARCH }} with CGO here — the C compiler ($cc) targets $triple." >&2
        echo "       Neru's Linux build needs CGO for the X11 and Wayland backends, so Go's own cgo" >&2
        echo "       runtime is compiled too, against the host SDK, and fails." >&2
        echo "" >&2
        echo "Supported alternatives:" >&2
        echo "  just lint-cross    compiles and lints the Linux build (amd64) with CGO on, in Docker" >&2
        echo "  just check-cross   fast type-check of the Linux and Windows builds, CGO off, no Docker" >&2
        echo "  CGO_ENABLED=0 GOOS=linux GOARCH={{ ARCH }} go build ./cmd/neru" >&2
        echo "                     pure-Go Linux binary; the CGO-only backends compile out, so it is" >&2
        echo "                     not the shipped product (docs/CROSS_PLATFORM.md#cgo-guidance)" >&2
        echo "" >&2
        echo "Have a Linux cross toolchain? Re-run with CC=<linux-targeting-compiler>." >&2
        exit 1
    fi
    echo "Building Neru for linux/{{ ARCH }}..."
    mkdir -p bin
    CGO_ENABLED=1 GOOS=linux GOARCH={{ ARCH }} go build -ldflags="{{ LDFLAGS }}" -o bin/neru-linux-{{ ARCH }} ./cmd/neru
    echo "✓ Build complete: bin/neru-linux-{{ ARCH }}"

# Generate Windows resource files (.syso) for embedding the app icon and manifest.
#
# Must be run before go build on/for Windows.  The .syso files are written into
# cmd/neru/ so go build picks them up automatically.
[doc('Generate the Windows icon and manifest .syso files into cmd/neru/.')]
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
[doc('Build a Windows binary from any host, with CGO off.')]
build-windows ARCH="amd64":
    @echo "Building Neru for windows/{{ ARCH }}..."
    mkdir -p bin
    just generate-winres {{ ARCH }}
    CGO_ENABLED=0 GOOS=windows GOARCH={{ ARCH }} go build -ldflags="{{ LDFLAGS }}" -o bin/neru-windows-{{ ARCH }}.exe ./cmd/neru
    @echo "✓ Build complete: bin/neru-windows-{{ ARCH }}.exe"

# Build a macOS binary for the current host.

# macOS requires CGO because the native bridge is part of the real product.
[doc('Build a macOS binary for this host, native bridge included.')]
build-darwin:
    @echo "Building Neru for macOS..."
    mkdir -p bin
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -o bin/neru-darwin ./cmd/neru
    @echo "✓ Build complete: bin/neru-darwin"

# Build with optimizations for release
[doc('Build an optimized, trimpath release binary into bin/neru.')]
release:
    @echo "Building release version..."
    @echo "Version: {{ VERSION }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Release build complete: bin/neru"

# Build with custom version
[doc('Build a release binary stamped with the version you pass.')]
build-version VERSION_OVERRIDE:
    @echo "Building Neru with custom version..."
    CGO_ENABLED=1 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/buildinfo.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/buildinfo.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Build complete: bin/neru (version: {{ VERSION_OVERRIDE }})"

# Build a macOS release artifact for CI on a native macOS host.

# Usage: just release-ci-darwin arm64 v1.2.3
[doc('Build the macOS release artifact, on a native macOS host.')]
release-ci-darwin ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (darwin/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    CGO_ENABLED=1 GOOS=darwin GOARCH={{ ARCH }} MACOSX_DEPLOYMENT_TARGET={{ MACOSX_DEPLOYMENT_TARGET }} CGO_LDFLAGS_ALLOW='-Wl,.*' CGO_LDFLAGS='-Wl,-macosx_version_min,{{ MACOSX_DEPLOYMENT_TARGET }}' go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/buildinfo.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/buildinfo.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-darwin-{{ ARCH }} ./cmd/neru
    @echo "✓ Release artifact for darwin/{{ ARCH }} built successfully"

# Build a Linux release artifact for CI on a native Linux host.

# Usage: just release-ci-linux amd64 v1.2.3
[doc('Build the Linux release artifact, on a native Linux host.')]
release-ci-linux ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (linux/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    CGO_ENABLED=1 GOOS=linux GOARCH={{ ARCH }} go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/buildinfo.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/buildinfo.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-linux-{{ ARCH }} ./cmd/neru
    @echo "✓ Release artifact for linux/{{ ARCH }} built successfully"

# Build a Windows release artifact for CI.

# Usage: just release-ci-windows amd64 v1.2.3
[doc('Build the Windows release artifact, from any host.')]
release-ci-windows ARCH VERSION_OVERRIDE:
    @echo "Building release artifact (windows/{{ ARCH }}) for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    just generate-winres {{ ARCH }}
    CGO_ENABLED=0 GOOS=windows GOARCH={{ ARCH }} go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/buildinfo.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/buildinfo.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-windows-{{ ARCH }}.exe ./cmd/neru
    @echo "✓ Release artifact for windows/{{ ARCH }} built successfully"

# Bundle the application
[doc('Build, then package and ad-hoc sign build/Neru.app.')]
bundle: release
    @echo "Bundling Neru..."
    mkdir -p build/Neru.app/Contents/{MacOS,Resources}

    cp -r bin/neru build/Neru.app/Contents/MacOS/neru

    cp resources/icon.icns build/Neru.app/Contents/Resources/icon.icns
    cp resources/Neru.entitlements build/Neru.app/Contents/Resources/Neru.entitlements

    sed "s/VERSION/{{ VERSION }}/g" resources/Info.plist.template > build/Neru.app/Contents/Info.plist

    codesign --force --deep --sign - --entitlements resources/Neru.entitlements --options runtime build/Neru.app

    @echo "✓ Bundle complete: build/Neru.app"

# Install a built Neru. Dispatches to the per-platform script in scripts/. Build
# first: `just bundle` (macOS), `just build` (Linux), `just build-windows`
# (Windows). Pass -y to accept every prompt non-interactively (`just install -y`).
# On Windows this runs under a bash such as Git Bash.
[doc('Install a built Neru with the platform script in scripts/.')]
install *ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    script="{{ justfile_directory() }}/scripts/install-{{ os() }}.sh"
    if [ ! -f "$script" ]; then
        echo "just install: unsupported platform '{{ os() }}'" >&2
        exit 1
    fi
    exec bash "$script" {{ ARGS }}

# Remove a Neru installed by `just install`, undoing each of its steps in
# reverse. Dispatches to the per-platform script in scripts/. Pass -y to accept
# every prompt non-interactively, and --purge to also remove your config and
# logs (they are kept otherwise, so -y alone can never delete your config.toml).
# On Windows this runs under a bash such as Git Bash.
[doc('Undo `just install`; your config survives unless you pass --purge.')]
uninstall *ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    script="{{ justfile_directory() }}/scripts/uninstall-{{ os() }}.sh"
    if [ ! -f "$script" ]; then
        echo "just uninstall: unsupported platform '{{ os() }}'" >&2
        exit 1
    fi
    exec bash "$script" {{ ARGS }}

# Run tests

# Run all tests (unit + integration). Desktop-safe: tests that would drive the
# real cursor, keyboard or overlays skip themselves here — run `just
# test-desktop` to include them when you can hand the machine over.
[doc('Run the unit and integration tests, leaving the desktop alone.')]
test: test-unit test-integration
    @echo "Running all tests..."

# Run unit tests
[doc('Run the unit tests.')]
test-unit:
    @echo "Running unit tests..."
    go test -v ./...

# Run the cross-platform-safe test slice: every package that contains no
# platform-tagged source at all, so its behavior is identical on macOS, Linux
# and Windows. Useful as a fast confidence check before or during Linux/Windows
# work — a failure here is a real cross-platform regression, not a
# host-specific one.
#
# This list is kept by hand but not on trust: TestFoundationSliceMatchesTheRecipe
# in internal/architecture parses it and fails when it stops matching the
# packages that qualify, naming each one. `just list-foundation-packages` prints
# the corrected list to paste back in.
#
# CI runs this recipe too, via test-ci, so a list that no longer resolves fails
# the build rather than only the next person to type it.
[doc('Run the test slice whose behavior is identical on all three platforms.')]
test-foundation:
    @echo "Running cross-platform foundation tests..."
    go test ./internal/config ./internal/config/loader \
        ./internal/app/components ./internal/app/components/scroll \
        ./internal/app/heldrepeat ./internal/app/ipcctrl \
        ./internal/app/keybinding \
        ./internal/app/services ./internal/app/services/indicator \
        ./internal/app/services/modeindicator \
        ./internal/app/services/stickyindicator \
        ./internal/app/services/virtualpointer \
        ./internal/architecture ./internal/cli/cliutil \
        ./internal/domain ./internal/domain/action \
        ./internal/domain/element ./internal/domain/grid \
        ./internal/domain/hint ./internal/domain/keyvocab \
        ./internal/domain/modecmd ./internal/domain/recursivegrid \
        ./internal/domain/state ./internal/derrors \
        ./internal/flagref \
        ./internal/adapter/logger \
        ./internal/adapter/overlay/render/badge \
        ./internal/adapter/platform/fontcache \
        ./internal/adapter/platform/fontgeneric \
        ./internal/adapter/platform/mousestate \
        ./internal/ports ./internal/ports/mocks \
        ./internal/domain/geometry
    @echo "✓ Cross-platform foundation tests passed"

# Print the packages test-foundation should be running. Use it to fix the recipe
# after TestFoundationSliceMatchesTheRecipe reports drift.
#
# The rule this applies — a package compiles to the same files on darwin, linux
# and windows, and has tests — lives in that test and only there. Deciding it a
# second time here is what let the old version of this recipe report packages
# that were platform-specific by directory or by a //go:build line no filename
# check could see.
[doc('Print the package list the test-foundation recipe should be running.')]
list-foundation-packages:
    @NERU_LIST_FOUNDATION=1 go test -count=1 -v \
        -run TestFoundationSliceMatchesTheRecipe ./internal/architecture | grep '^\./'

# Run integration tests (desktop-safe subset)
# Tests that drive the real cursor, keyboard or overlays gate themselves on
# NERU_DESKTOP_TESTS (see requireDesktop in the test files) and skip here, so
# this target never commandeers the machine; `just test-desktop` opts in.
#
# -p 1 runs one package binary at a time. The desktop-driving tests share one
# physical input device, and even the passive half shares daemon sockets and
# log files; concurrent package binaries fight over them.
#
# -count=1 disables the test cache. Go keys that cache on the inputs it can see
# — sources, env, files read — and none of those change when the thing an
# integration test actually depends on does: whether Accessibility permission is
# granted, whether a daemon holds the socket, whether the screen is locked. A
# result cached from a run under different conditions is then replayed as a
# pass, which is worse than no result at all: it reports green for a run that
# never happened.
[doc('Run the integration tests, minus the ones that drive the desktop.')]
test-integration:
    @echo "Running integration tests (desktop-safe)..."
    go test -tags=integration -p 1 -count=1 -v ./...

# Run the full integration suite INCLUDING the tests that drive the real
# desktop: the cursor moves, real clicks and scrolls land, overlays flash, and
# an event tap briefly intercepts the keyboard. Hand the machine over while it
# runs. Needs Accessibility permission (System Settings > Privacy & Security).
[doc('Run every integration test, including the ones that drive the desktop.')]
test-desktop:
    @echo "Running integration tests (including desktop-driving tests)..."
    NERU_DESKTOP_TESTS=1 go test -tags=integration -p 1 -count=1 -v ./...

# Run with race detection
[doc('Run the unit and integration tests under the race detector.')]
test-race: test-race-unit test-race-integration
    @echo "Running tests with race detection..."

# Run unit tests with race detection
[doc('Run the unit tests under the race detector.')]
test-race-unit:
    @echo "Running unit tests with race detection..."
    go test -race -v ./...

# Run integration tests with race detection
# See test-integration for why -p 1 and -count=1 are required here.
[doc('Run the integration tests under the race detector.')]
test-race-integration:
    @echo "Running integration tests with race detection..."
    go test -tags=integration -race -p 1 -count=1 -v ./...

# The full test suite at maximum depth: everything plain and again under
# -race (four passes: unit, integration, race-unit, race-integration),
# including the desktop-driving tests. This is the deep local bar for a real
# desktop session with Accessibility granted — it takes over the cursor and
# keyboard while it runs. CI gates on test-ci below, which is the same minus
# the passes that are meaningless on a headless runner.
[doc('Run every suite, plain and under -race, desktop-driving tests included.')]
test-all:
    NERU_DESKTOP_TESTS=1 just test test-race

# Integration tests as CI runs them: -short makes tests that drive the real
# cursor/keyboard or need OS permissions skip *explicitly* (via their
# testing.Short guards) instead of passing for reasons nobody wrote down —
# GitHub runners have no Accessibility grant and no interactive session.
# Tests that touch real input or permissions must guard themselves with
# testing.Short(); permission-free integration tests (config, logger, IPC,
# CLI) still run fully.
[doc('Run the integration tests the way CI does, under -short.')]
test-integration-ci:
    @echo "Running integration tests (CI profile: -short)..."
    go test -tags=integration -short -p 1 -count=1 ./...

# Everything CI's test job runs: the foundation slice, unit, unit under -race,
# and the CI profile of the integration suite. Race coverage on the integration
# half is left to test-all on real machines — on a permission-less runner it
# doubles the runtime of the least meaningful pass.
#
# test-foundation runs first and is otherwise pure duplication: every package it
# names, test-unit runs again seconds later. It is here for the recipe itself.
# Nothing gated on it until #1267, and it spent weeks aborting before it ran a
# single test — a package deleted in #1221 stayed in its list, so every
# invocation died on "directory not found" and reported that to nobody.
# TestFoundationSliceRunsInCI keeps it reachable from here; the workflow invokes
# this recipe on macOS, Linux and Windows alike, which is where a slice claiming
# to be cross-platform-safe should be proven.
[doc('Run every test suite CI runs: foundation, unit, unit -race, integration.')]
test-ci: test-foundation test-unit test-race-unit test-integration-ci

# Run the set of checks CI gates a pull request on, in the same order, on this
# host. CI runs the same recipes on macOS, Linux and Windows, so a green run
# here is one leg of three. This is the real pre-push bar — `just test` alone
# is a subset of it. For the deepest local verification (full integration +
# race passes on a real desktop session) run `just test-all` as well.
[doc('Run the checks CI gates a pull request on, on this host only.')]
ci: fmt-check lint vet build test-ci vuln
    @echo "✓ All CI checks passed"

# Check if files are formatted correctly
[doc('Check that the Objective-C sources are clang-format clean.')]
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
    done < <(find internal/adapter \( -name "*.h" -o -name "*.m" -o -name "*.c" \) -print0)
    if [ $EXIT_CODE -ne 0 ]; then
        echo "Some Objective-C files are not properly formatted. Run 'just fmt' to fix them."
        exit 1
    fi
    echo "✓ All Objective-C files are properly formatted"

# Generate man pages
[doc('Generate the man pages into OUTPUT_DIR.')]
genman OUTPUT_DIR="build/man":
    @echo "Generating man pages..."
    go run ./cmd/genman {{ OUTPUT_DIR }}
    @echo "✓ Man pages generated in {{ OUTPUT_DIR }}/"

# Rewrite the mode-flag reference from the grammar's descriptor table.
# Run after adding, removing or re-wording a mode flag; the architecture
# guardrail fails while the page is out of date.
[doc('Rewrite the mode-flag reference in docs/CLI.md from the grammar.')]
genflagref:
    @echo "Generating the mode-flag reference..."
    go run ./cmd/genflagref docs/CLI.md

# Clean build artifacts
[doc('Delete the build outputs, the app bundles and the lint cache.')]
clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/
    rm -rf build/
    rm -rf *.app
    rm -f cmd/neru/rsrc_windows_*.syso
    rm -rf "{{ GOLANGCI_LINT_CACHE }}"
    @echo "✓ Clean complete"

# Where golangci-lint keeps its cache, scoped to this checkout.
#
# Its default is one host-level directory shared by every checkout on the
# machine, and the agent worktrees parked under .claude/worktrees/ are separate
# checkouts of this repository. They all wrote to that one cache, so removing a
# worktree left entries behind naming files under a directory that no longer
# exists, and the next `just fmt` in another checkout reported them as a lint
# failure against a real-looking path — nothing wrong with the code, and no hint
# in the message that the cache was the problem.
#
# Keeping the cache in the checkout gives each one its own, keeps repeat lints
# warm (it persists between runs, and `just clean` is what removes it), and lets
# a removed checkout take its cache with it rather than leaving one behind to go
# stale. It is gitignored. The containerized lint-cross recipe passes its own
# value to the container and is unaffected.
export GOLANGCI_LINT_CACHE := justfile_directory() / ".golangci-cache"

# Format code
[doc('Format the Go sources with golangci-lint and the Objective-C ones.')]
fmt:
    @echo "Formatting Go files..."
    golangci-lint fmt
    golangci-lint run --fix
    @echo "Formatting Objective-C files..."
    @find internal/adapter \( -name "*.h" -o -name "*.m" -o -name "*.c" \) -exec sh -c 'case "$1" in *.c) af=file.c;; *) af=file.m;; esac; clang-format -i --style=file --assume-filename="$af" "$1"' _ {} \;
    @echo "✓ Format complete"

# Lint code (Go via golangci-lint, Objective-C via clang-tidy)
[doc('Lint the Go and the Objective-C sources.')]
lint: lint-go lint-objc

# Lint Go code
[doc('Lint the Go sources with golangci-lint.')]
lint-go:
    @echo "Linting Go code..."
    golangci-lint run
    @echo "✓ Go lint complete"

# Lint Objective-C with the clang static analyzer (via clang-tidy, which
# devbox's clang-tools provides). macOS only — the .m files need the macOS SDK.
#
# Excluded checks, each for a reason:
#  - optin.osx.cocoa.localizability: Neru is not localized; user-facing
#    strings are intentionally plain literals.
#  - deadcode.DeadStores: the eventtap bridge deliberately copies old table
#    refs to locals and nils them after unlocking so ARC releases the old
#    tables outside the lock — the analyzer sees those stores as dead.
#  - optin.performance.GCDAntipattern: permission prompts are called from Go
#    through a synchronous bridge, so blocking on a semaphore is the contract.
[doc('Lint the Objective-C sources with the clang analyzer; macOS only.')]
lint-objc:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "$(uname -s)" != "Darwin" ]; then
        echo "Skipping Objective-C lint: requires the macOS SDK"
        exit 0
    fi
    if ! command -v clang-tidy >/dev/null 2>&1; then
        echo "clang-tidy not found — run inside 'devbox shell' (clang-tools provides it)" >&2
        exit 1
    fi
    echo "Linting Objective-C files (clang-tidy)..."
    SDK=$(xcrun --show-sdk-path)
    CHECKS='-*,clang-analyzer-*,-clang-analyzer-optin.osx.cocoa.localizability*,-clang-analyzer-deadcode.DeadStores,-clang-analyzer-optin.performance.GCDAntipattern'
    export SDK CHECKS
    find internal -name '*.m' -print0 | xargs -0 -P 4 -n 1 bash -c '
        out=$(clang-tidy --quiet --checks="$CHECKS" "$1" -- \
            -x objective-c -fobjc-arc -fmodules -mmacosx-version-min=14.0 \
            -isysroot "$SDK" 2>/dev/null)
        if echo "$out" | grep -q "warning:"; then
            echo "$out"
            exit 1
        fi
    ' _
    echo "✓ Objective-C lint complete"

# Vet
[doc('Run go vet over the host build.')]
vet:
    @echo "Vetting code..."
    go vet ./...
    @echo "✓ Vet complete"

# Vet the code as the other platforms see it.
#
# `just vet` only type-checks the host's build tags, so a change to shared code
# that breaks the Linux or Windows build is invisible locally and only fails in
# CI. This compiles every package *and its tests* for both other targets.
#
# CGO is off, so this covers the pure-Go and no-cgo backends. The cgo-only Linux
# backends (X11, wlroots) still need a cross toolchain or CI to type-check.
[doc('Run go vet over the Linux and Windows builds too, with CGO off.')]
vet-cross:
    @echo "Vetting for linux/amd64 (CGO off)..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go vet ./...
    @echo "Vetting for windows/amd64 (CGO off)..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
    @echo "✓ Cross-platform vet complete"

# Actually run the Linux test suite locally, in a container.
#
# This is the strongest cross-platform check available without pushing: it
# executes the linux-tagged tests — the stub contracts, the backend dispatch,
# the linux-only packages — rather than only type-checking them.
#
# The image mirrors the Linux CI job: same native dev packages, CGO enabled, and
# headless (no DISPLAY / WAYLAND_DISPLAY), so what passes here is what CI sees.
# Docker caches the image after the first run.
#
# Requires a running Docker daemon.
[doc('Run the Linux test suite in a CI-equivalent container; needs Docker.')]
test-linux:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building the Linux CI-equivalent image (cached after first run)..."
    docker build -q -t neru-linux-ci - >/dev/null <<'DOCKERFILE'
    FROM golang:1.26
    RUN apt-get update -qq && apt-get install -y -qq \
        libcairo2-dev libwayland-dev libx11-dev libxtst-dev libxrandr-dev \
        libxinerama-dev libxfixes-dev libxkbcommon-dev wayland-protocols \
        libei-dev liboeffis-dev libxi-dev libxrender-dev libfontconfig1-dev \
        pkg-config >/dev/null 2>&1
    DOCKERFILE
    echo "Running the Linux test suite (CGO on, headless)..."
    docker run --rm -v "$PWD":/src -w /src \
        -e CGO_ENABLED=1 -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomod \
        neru-linux-ci go test -count=1 -timeout 900s ./...
    echo "✓ Linux test suite passed"

# Same as test-linux but with CGO off, exercising the no-cgo Linux backends.
[doc('Run the Linux test suite in a container with CGO off; needs Docker.')]
test-linux-nocgo:
    @echo "Running the Linux test suite in a container (CGO off)..."
    docker run --rm -v "$PWD":/src -w /src \
        -e CGO_ENABLED=0 -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomod \
        golang:1.26 go test -count=1 -timeout 600s ./...
    @echo "✓ Linux no-cgo test suite passed"

# Compile every package *and its test binary* for Windows.
#
# Windows cannot be executed from a macOS or Linux host, so this is the
# strongest available check: it catches everything `go vet` does plus anything
# that only fails when the test binary is linked.
[doc('Compile every package and its test binary for Windows.')]
test-windows-compile:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Compiling Windows test binaries..."
    failed=0
    for pkg in $(go list ./...); do
        if ! GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /dev/null "$pkg" >/tmp/neru-win.log 2>&1; then
            if ! grep -q "no test files\|no non-test Go files\|build constraints exclude all Go files" /tmp/neru-win.log; then
                echo "FAIL: $pkg"; cat /tmp/neru-win.log; failed=1
            fi
        fi
    done
    [ "$failed" = "0" ] || exit 1
    echo "✓ Windows test binaries compile"

# Lint the code as the other platforms see it.
#
# `just lint` only lints the host's build tags, so a linter complaint in
# linux- or windows-tagged source is invisible locally and first appears as a
# red CI job. Reproducing it needs the Linux toolchain (CGO on, native dev
# headers), so this runs in the same container image the Linux CI job uses, with
# the golangci-lint version pinned in .github/workflows/ci.yml.
#
# Requires a running Docker daemon.
[doc('Lint the Linux and Windows builds in a container; needs Docker.')]
lint-cross:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building the Linux lint image (cached after first run)..."
    docker build -q -t neru-linux-ci - >/dev/null <<'BASE'
    FROM golang:1.26
    RUN apt-get update -qq && apt-get install -y -qq \
        libcairo2-dev libwayland-dev libx11-dev libxtst-dev libxrandr-dev \
        libxinerama-dev libxfixes-dev libxkbcommon-dev wayland-protocols \
        libei-dev liboeffis-dev libxi-dev libxrender-dev libfontconfig1-dev \
        pkg-config >/dev/null 2>&1
    BASE
    docker build -q -t neru-linux-lint - >/dev/null <<'LINT'
    FROM neru-linux-ci
    RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
    LINT
    echo "Linting for linux/amd64 (CGO on)..."
    docker run --rm -v "$PWD":/src -w /src \
        -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomod -e GOLANGCI_LINT_CACHE=/tmp/glcache \
        neru-linux-lint golangci-lint run
    echo "Linting for windows/amd64 (CGO off)..."
    docker run --rm -v "$PWD":/src -w /src \
        -e GOOS=windows -e GOARCH=amd64 -e CGO_ENABLED=0 \
        -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/gomod -e GOLANGCI_LINT_CACHE=/tmp/glcache \
        neru-linux-lint golangci-lint run
    echo "✓ Cross-platform lint complete"

# Everything that can be checked for other platforms without pushing.
#
# Deliberately excludes lint-cross and test-linux, which need Docker — run those
# separately for a real Linux lint and execution rather than a type-check.
[doc('Type-check the Linux and Windows builds; no Docker, no cross toolchain.')]
check-cross: vet-cross test-windows-compile
    @echo "✓ Cross-platform checks complete (run 'just lint-cross' and 'just test-linux' for real Linux runs)"

# Scan dependencies for known vulnerabilities.
#
# govulncheck is call-graph aware: it only reports a CVE when the vulnerable
# symbol is actually reachable from this module, so a finding here is a finding
# that ships. It respects GOOS/GOARCH, which matters for a tree this
# build-tagged — a vulnerable Windows-only dependency is invisible from a macOS
# run, so CI runs this natively on all three platforms.
#
# The tool is fetched at @latest rather than pinned: its value is knowing about
# vulnerabilities published after the pin would have been written, and the
# vulnerability database is fetched at run time regardless.
[doc('Scan the dependencies for known vulnerabilities with govulncheck.')]
vuln:
    @echo "Scanning for known vulnerabilities..."
    go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
    @echo "✓ No known vulnerabilities"

# Run unit tests with coverage and print the total.
#
# Unit tests only: integration tests need real permissions, a real screen and a
# free socket, so including them would make the number depend on the machine
# rather than on the code.
#
# -coverpkg=./... is what makes the number honest. Without it Go credits a
# package only for the statements its own tests execute, so code that is
# thoroughly exercised from a neighbouring package reads as zero. The action
# sequence executor measured 0% that way and 77% this way; nothing about the
# tests changed, only which package was asked.
[doc('Run the unit tests with coverage and print the total.')]
coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running unit tests with coverage..."
    go test -coverprofile=coverage.txt -covermode=atomic -coverpkg=./... ./...
    go tool cover -func=coverage.txt | tail -1
    echo "✓ Coverage profile written to coverage.txt"

# Render the coverage profile as a browsable HTML report.
[doc('Render the coverage profile as a browsable HTML report.')]
coverage-html: coverage
    go tool cover -html=coverage.txt -o coverage.html
    @echo "✓ Coverage report written to coverage.html"

# Download dependencies
[doc('Download the module dependencies and tidy go.mod.')]
deps:
    @echo "Downloading dependencies..."
    go mod download
    go mod tidy
    @echo "✓ Dependencies updated"

# Verify dependencies
[doc('Verify the downloaded module dependencies against go.sum.')]
verify:
    @echo "Verifying dependencies..."
    go mod verify
    @echo "✓ Dependencies verified"

# Generate icon.icns from a source PNG (e.g., just generate-icns icon-1024.png)
[doc('Generate resources/icon.icns from a source PNG.')]
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
[doc('Generate the two 44x44 systray icons from source PNGs.')]
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
[doc('Generate the app icon and both systray icons from source PNGs.')]
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
WLR_PROTOCOL_DIR := "internal/adapter/platform/linux/wlr_protocol"

# Download Wayland protocol XMLs from canonical upstream repositories
[doc('Download the Wayland protocol XMLs from their upstream repositories.')]
fetch-protocols:
    @echo "Fetching Wayland protocol XMLs..."
    mkdir -p {{ PROTOCOL_DIR }}
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlroots/-/raw/master/protocol/wlr-layer-shell-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/wlr-layer-shell-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlroots/-/raw/master/protocol/virtual-keyboard-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/virtual-keyboard-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlr-protocols/-/raw/master/unstable/wlr-virtual-pointer-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/wlr-virtual-pointer-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/unstable/xdg-output/xdg-output-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/xdg-output-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/stable/xdg-shell/xdg-shell.xml" -o {{ PROTOCOL_DIR }}/xdg-shell.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/unstable/relative-pointer/relative-pointer-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/relative-pointer-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wlroots/wlr-protocols/-/raw/master/unstable/wlr-foreign-toplevel-management-unstable-v1.xml" -o {{ PROTOCOL_DIR }}/wlr-foreign-toplevel-management-unstable-v1.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/stable/viewporter/viewporter.xml" -o {{ PROTOCOL_DIR }}/viewporter.xml
    curl -fsSL "https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/master/staging/fractional-scale/fractional-scale-v1.xml" -o {{ PROTOCOL_DIR }}/fractional-scale-v1.xml
    @echo "✓ Protocol XMLs downloaded to {{ PROTOCOL_DIR }}/"

# Generate wayland-scanner files from XMLs
[doc('Generate the wayland-scanner headers and code from the XMLs.')]
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

    # wlr-foreign-toplevel-management (unstable) — focused-app app_id tracking
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/wlr-foreign-toplevel-management-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/foreign-toplevel.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/wlr-foreign-toplevel-management-unstable-v1.xml > {{ WLR_PROTOCOL_DIR }}/foreign-toplevel.c

    # viewporter (stable) — maps an over-rendered buffer down to logical size for fractional scaling
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/viewporter.xml > {{ WLR_PROTOCOL_DIR }}/viewporter.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/viewporter.xml > {{ WLR_PROTOCOL_DIR }}/viewporter.c

    # fractional-scale (staging) — compositor's preferred per-surface fractional scale
    wayland-scanner client-header < {{ PROTOCOL_DIR }}/fractional-scale-v1.xml > {{ WLR_PROTOCOL_DIR }}/fractional-scale-v1.h
    wayland-scanner private-code < {{ PROTOCOL_DIR }}/fractional-scale-v1.xml > {{ WLR_PROTOCOL_DIR }}/fractional-scale-v1.c
    @echo "✓ Protocol files generated in {{ WLR_PROTOCOL_DIR }}/"

# Download and generate all Wayland protocols
[doc('Download the Wayland protocol XMLs and generate their code.')]
generate-all-protocols: fetch-protocols generate-protocols
    @echo "✓ All Wayland protocols downloaded and generated"
