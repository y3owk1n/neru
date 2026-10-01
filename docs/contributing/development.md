# Development guide

How to set up a development environment, build, test and debug Neru, and where
to put new code. The contribution process lives in
[CONTRIBUTING.md](../../CONTRIBUTING.md), the system shape in
[architecture](architecture.md), platform work in the
[porting guide](porting.md), and code conventions in the root
[AGENTS.md](../../AGENTS.md).

## Quick start

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

oku sync && oku allow  # or: brew install go just golangci-lint llvm
just build
./bin/neru launch       # runs in the foreground
```

Then from a second terminal:

```bash
./bin/neru hints        # should show hint overlays
```

> [!IMPORTANT]
> On macOS this needs Accessibility permission granted to whichever app starts
> the daemon, which is your terminal when you run `./bin/neru launch` by hand.
> Without it the smoke test reports an accessibility error instead of drawing
> overlays.

There is no `just run` recipe. Build first, then launch the daemon directly.
The CLI talks to the running daemon over a socket, so both halves come from the
same `./bin/neru` binary.

On Linux, install the [build dependencies](#build-dependencies) before your
first build. To install Neru rather than develop it, see
[installation](../guide/installation.md).

## Development setup

### Prerequisites

- **Go 1.26+**: [install Go](https://golang.org/dl/)
- **Xcode Command Line Tools** (macOS): `xcode-select --install`
- **Build dependencies** (Linux): the system `-dev`/`-devel` packages a CGO
  build links against, listed under [Building on Linux](#building-on-linux).
  Install them before your first build, including under oku.
- **Just**, the command runner: [install](https://github.com/casey/just)
- **golangci-lint**: [install](https://golangci-lint.run/usage/install/)

### Option A: oku (recommended)

[oku](https://github.com/y3owk1n/oku) installs the toolchain that `oku.toml`
lists, at the versions `oku.lock` pins, into a profile that belongs to this
repo:

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/oku/main/install.sh | sh

oku sync               # install the toolchain
oku allow              # let the shell hook put it on PATH inside this repo
```

The shell hook that oku's installer prints does the rest whenever you `cd` in.
With [direnv](https://direnv.net/), `eval "$(oku env --shell bash)"` in an
`.envrc` does the same.

oku manages Go, gopls, goimports, gofumpt, golines, golangci-lint, just, and
clang-format and clang-tidy (for the Objective-C sources). CI installs the same
versions from `oku.lock` with the oku action, on macOS, Linux and Windows.
`oku update` moves each tool to the newest version that `oku.toml` allows, and
`oku outdated` shows what is behind.

oku does not provide the libraries a Linux CGO build links against. See
[Building on Linux](#building-on-linux).

### Option B: manual installation

```bash
brew install go just golangci-lint llvm
```

`llvm` supplies `clang-format` for Objective-C formatting. oku's extra tools
are optional and installable on their own:

```bash
go install golang.org/x/tools/gopls@latest
go install mvdan.cc/gofumpt@latest
go install github.com/segmentio/golines@latest
```

Match the golangci-lint version that `oku.lock` pins, which is the one CI uses.

### Verify

```bash
go version              # 1.26+
just --version
golangci-lint --version
just --list             # all available recipes
```

An EditorConfig plugin is worth installing. `.editorconfig` carries the tab and
line-ending rules that CI enforces.

## Common tasks

Every build, test and lint entry point goes through `just`. `just --list` shows
the full set, including the Wayland protocol generation and icon recipes.

| Task    | Command                      | Description                                     |
| ------- | ---------------------------- | ----------------------------------------------- |
| Build   | `just build`                 | Compile for the current platform                |
| Build   | `just build-darwin`          | Build a macOS binary (on macOS)                 |
| Build   | `just build-linux [ARCH]`    | Build a Linux binary (defaults to amd64)        |
| Build   | `just build-windows [ARCH]`  | Build a Windows binary                          |
| Build   | `just build-version v1.0.0`  | Build with an explicit version string           |
| Build   | `just release`               | Optimized, stripped release build               |
| Dist    | `just dist`                  | Assemble the release layout (bin, man, `Neru.app`) in `build/dist` |
| Install | `just install [-y]`          | Build, `dist`, then run the release installer, `-y` auto-accepts |
| Test    | `just test`                  | Unit + integration, desktop-safe                |
| Test    | `just test-unit`             | Unit tests only                                 |
| Test    | `just test-integration`      | Integration tests only, desktop-safe            |
| Test    | `just test-desktop`          | Integration including the tests that drive the real cursor, keyboard and overlays |
| Test    | `just test-foundation`       | Fast cross-platform-safe slice, CI runs it too  |
| Test    | `just test-race`             | Unit + integration with `-race`                 |
| Test    | `just test-race-unit`        | Unit tests with `-race`                         |
| Test    | `just test-race-integration` | Integration tests with `-race`                  |
| Test    | `just test-all`              | `test` and `test-race`, desktop tests included: the deepest sweep |
| Test    | `just test-ci`               | What CI gates on: foundation, unit, race-unit, short integration |
| Test    | `just coverage`              | Unit tests with coverage, prints the total      |
| Test    | `just coverage-html`         | Coverage as a browsable `coverage.html`         |
| Lint    | `just lint`                  | golangci-lint, plus clang-tidy on `.m` files (macOS) |
| Lint    | `just vet`                   | `go vet`                                        |
| Lint    | `just vuln`                  | `govulncheck`: reachable CVEs in dependencies   |
| Lint    | `just check-cross`           | CGO-off type-check of the Linux and Windows builds |
| Lint    | `just lint-cross`            | Lint the Linux build with CGO on, in Docker     |
| Test    | `just test-linux`            | Run the Linux test suite in a CI-equivalent container (Docker) |
| Gate    | `just ci`                    | The pre-push gate: the checks CI gates on, on this host |
| Format  | `just fmt`                   | Format Go and Objective-C                       |
| Format  | `just fmt-check`             | Check Objective-C formatting                    |
| Docs    | `just genman`                | Generate man pages                              |
| Docs    | `just genflagref`            | Rewrite the mode-flag reference in `docs/reference/cli.md` |
| Docs    | `just gensupportref`         | Rewrite the platform-support table in `docs/reference/platform-support.md` |
| Clean   | `just clean`                 | Remove build artifacts                          |

Targeting a single package or test:

```bash
go test ./internal/domain/hint/
go test -run TestScrollMode_HandleKey_DoesNothing ./internal/app/modes/
go test -tags=integration ./internal/adapter/accessibility/
```

Watch mode, if you have [entr](https://eradman.com/entrproject/):

```bash
find . -name "*.go" | entr -r just test
```

### What `just ci` covers, and what it does not

`just ci` runs `fmt-check lint vet build check-cross test-ci vuln`, in that
order, on this host. CI runs the same recipes on macOS, Linux and Windows, so a
green run here is one leg of three. For the deepest local check on a real
desktop session, run `just test-all` as well.

`just check-cross` is the only member that looks at the other two targets. It
type-checks the Linux and Windows builds with CGO off, which catches a build
break in a plain `//go:build linux` or `//go:build windows` file. `just lint`
and `just vet` cannot see those, since both compile for the host. It does not
catch a break in a cgo-tagged file, which CGO-off skips entirely. It costs
seconds and needs no Docker.

The cgo-only Linux paths need a real Linux toolchain. `just lint-cross` and
`just test-linux` provide one in a container, and CI checks them on every push.
Neither is part of `just ci`, on purpose. A documented pre-push gate that fails
because no Docker daemon is running is worse than a gate that documents what it
does not check ([ADR 0012](../adr/0012-the-first-hour-must-not-lie.md)).

### What the Linux desktop legs cover, and what they do not

CI has two jobs `just ci` has no counterpart for. Both run `just test-desktop`,
the tier that drives the real cursor, keyboard and overlays, inside a
disposable Linux session with a session D-Bus and AT-SPI on it. A disposable
runner is the one place that tier is safe, since locally it takes over the
machine. The two legs share every step but the display server.

| Job                          | Session                                  | Bar under [ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md) |
| ---------------------------- | ---------------------------------------- | ------------------------------- |
| `desktop (ubuntu-latest)`     | sway on the wlroots headless backend     | behavioral parity (the blessed stack) |
| `desktop-x11 (ubuntu-latest)` | Xvfb with openbox                        | capability parity               |

Each leg asserts its session before running anything, so a broken environment
fails as a broken environment rather than as test failures that look like
Neru's. The sway leg requires `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_manager_v1`. The X11 leg requires the XTEST, XFIXES and
RANDR extensions and an EWMH window manager owning the root window. A bare Xvfb
has no `_NET_ACTIVE_WINDOW`, which is why openbox runs. Both require
`CGO_ENABLED=1` and an answering accessibility bus.

The X11 leg exercises the X11 half of every subsystem with two
implementations: XTest injection, `XGrabKey` hotkeys, `XGrabKeyboard` capture,
XRandR screens, `XGetImage` capture, the XFixes overlay shape,
`_NET_ACTIVE_WINDOW` and `WM_CLASS` identity, and `Xft.dpi` scaling. Both legs
run with nothing focused, which is the only place CI reaches that state for the
capability contract tests in `internal/adapter/platform`.

What neither leg covers:

- **Most desktop-driving behavior on Linux.** Most tests that read
  `NERU_DESKTOP_TESTS` are macOS tests. The Linux ones are the smooth-scroll
  measurement, which maps a real xdg-shell window and measures what the
  compositor delivers to it, the X11 keyboard-grab and layout tests, and the
  `neru services` tests, which also need a reachable systemd user manager.
- **KDE.** Nothing in CI runs KWin. Xwayland is disabled in the sway session,
  so X11 is covered only by its own leg.
- **Device access.** The runners have no `/dev/dri` and no `input` group, and
  the headless compositor reads no input devices, so the evdev and uinput paths
  cannot run.
- **A focused window.** Neither session runs a client, so which application is
  focused is never exercised.
- **Behavioral parity on X11.** The X11 leg is not evidence of it.

Each leg writes a job summary counting executed tests, failures, packages
reporting `FAIL`, and every skip with its reason. The step also enforces the
counts. It fails if nothing executed, or if a test skipped because it could not
see the display server. A leg that passes by skipping its tests reports green
without testing anything, which is worse than having no leg.

**Both legs block merges.** They were made required after 40 clean runs each on
pull requests and `main`. Those runs were counted from step conclusions,
because job-level `continue-on-error` reports a failed job as successful.

## Building

### Without Just

```bash
VERSION=$(git describe --tags --always --dirty)

go build \
  -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version=$VERSION" \
  -trimpath \
  -o bin/neru \
  ./cmd/neru
```

- `-ldflags="-s -w"` strips debug info and the symbol table
- `-trimpath` removes filesystem paths from the binary
- `-X pkg.Var=value` injects the version at build time

### Cross-platform checks

Starting Linux or Windows work? The minimum smoke test is:

```bash
just build
just test-foundation
```

`just test-foundation` runs every package whose behavior is identical on all
three platforms, so a failure there is a real cross-platform regression rather
than a host-specific one. Only the target OS can run `just test` meaningfully,
since integration tests are tagged per OS.

- `just build-windows` cross-compiles from any host, with CGO off.
- `just build-linux` does not. Linux needs CGO, and a macOS C compiler builds
  the cgo runtime against the wrong SDK, so the recipe refuses when the
  compiler does not target Linux. From macOS, use `just check-cross` for a fast
  CGO-off type-check, or `just lint-cross` for the CGO-on Linux build in
  Docker. Tagged Linux release binaries are built by CI on a native runner.
- `just lint` only sees your own platform, because golangci-lint honours build
  tags. Reproduce Linux findings with
  `CGO_ENABLED=0 GOOS=linux golangci-lint run ./internal/...`, ignoring the
  `unused` and `unparam` reports that come only from the excluded `*_cgo.go`
  files. The cgo paths need `just lint-cross` or CI.

Backend, CGO and modifier expectations are not per-OS constants. Start from
[profile.go](../../internal/adapter/platform/profile.go) and
[CGO guidance](porting.md#cgo-guidance).

## Building on Linux

### Build dependencies

A Linux build uses CGO and links against distribution libraries that oku does
not provide. `just linux-deps` installs them with apt, dnf or pacman, together
with a C compiler and pkg-config. The libraries are listed below per
distribution. What each library is for at run time is in the
[Linux setup guide](../guide/linux.md).

`libei` and `liboeffis` are linked at build time for the KDE input path, so
install them even if you only test on wlroots compositors. `fontconfig` is
required at build time. DejaVu fonts are the defaults when `font_family` is
unset.

#### Debian / Ubuntu

```bash
sudo apt-get install -y \
  libcairo2-dev \
  libwayland-dev \
  libx11-dev \
  libxtst-dev \
  libxrandr-dev \
  libxrender-dev \
  libxext-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libei-dev \
  liboeffis-dev \
  libfontconfig-dev \
  libtesseract-dev \
  tesseract-ocr-eng \
  libpipewire-0.3-dev \
  wayland-protocols \
  fonts-dejavu-core
```

#### Fedora

```bash
sudo dnf install -y \
  cairo-devel \
  wayland-devel \
  libX11-devel \
  libXtst-devel \
  libXrandr-devel \
  libXrender-devel \
  libXext-devel \
  libXfixes-devel \
  libxkbcommon-devel \
  libei-devel \
  liboeffis-devel \
  fontconfig-devel \
  tesseract-devel \
  tesseract-langpack-eng \
  pipewire-devel \
  wayland-protocols-devel \
  dejavu-sans-fonts dejavu-serif-fonts dejavu-sans-mono-fonts
```

Fedora ships `liboeffis-devel` on its own from Fedora 42 on.

#### Arch Linux

```bash
sudo pacman -S \
  cairo \
  wayland \
  libx11 \
  libxtst \
  libxrandr \
  libxrender \
  libxext \
  libxfixes \
  libxkbcommon \
  libei \
  fontconfig \
  tesseract \
  tesseract-data-eng \
  libpipewire \
  wayland-protocols \
  ttf-dejavu
```

On Arch, `liboeffis` is part of the `libei` package.

### Native and cross builds

```bash
# Native build on the host (recommended for local dev and testing)
just build

# Build for a named Linux GOARCH (recipe defaults to amd64)
just build-linux          # amd64
just build-linux arm64    # arm64
```

Building for an arch other than the host's needs a C compiler that targets it,
passed as `CC`. Cross-compiling from macOS to Linux is not supported. See
[Cross-platform checks](#cross-platform-checks). Verify the binary matches your
target:

```bash
go env GOARCH
file bin/neru
```

## Testing

Neru has four testing layers:

1. **Unit tests**: shared Go logic with no native OS dependency, using mocks
   from `internal/ports/mocks`.
2. **Contract tests**: ports and adapters agreeing on error semantics such as
   `CodeNotSupported`.
3. **Integration tests**: real OS behavior behind the `integration` build tag.
4. **Architecture tests**: guardrails protecting package boundaries and
   platform isolation (`internal/architecture/`).

Contract tests check that stubs are not silent no-ops, per subsystem rather
than per stub. When you
add a stubbed platform feature, update the subsystem's existing contract test if
it has one, and write a new one when a caller could read the stub's `nil` as
success. `internal/adapter/platform/AGENTS.md` states the rule and names the
tests that exist.

### Organization

| Type            | File pattern                 | Build tag             | Command                 |
| --------------- | ---------------------------- | --------------------- | ----------------------- |
| **Unit**        | `*_test.go`                  | none                  | `just test-unit`        |
| **Integration** | `*_integration_<os>_test.go` | `integration && <os>` | `just test-integration` |

Naming, mocks and build-tag conventions are in the root
[AGENTS.md](../../AGENTS.md). The macOS main-run-loop test harness is
documented in
[darwin/AGENTS.md](../../internal/adapter/platform/darwin/AGENTS.md).

### What each layer covers

**Unit**: hint generation, grid calculations, element filtering, action
processing, mode transitions, config parsing, validation and defaults, and CLI
argument handling. These run everywhere.

**Integration**: most of these are macOS, covering real Accessibility and event
tap APIs, global hotkey registration, overlay and window management, Unix
socket IPC, config loading and reloading, and service-to-adapter coordination.
Linux has a handful under `internal/adapter/platform/linux/` (fontconfig, X11,
screen capture, OCR, notifications) plus the evdev probe, the smooth-scroll
measurement and `neru services`. Windows has the services command, the overlay
transition and the UIA tree walk. Everything else on both is pinned by unit and
contract tests, so adding real integration tests there is one of the more
valuable contributions available.

No `just` recipe runs the Linux integration tests from a macOS host.
`just test-linux` runs the container without the `integration` tag, and
`just test-integration` runs on the host. Run them the way the container recipe
does and add the tag:

```bash
docker run --rm -v "$PWD":/src -w /src -e CGO_ENABLED=1 neru-linux-ci \
  go test -tags=integration ./...
```

CI covers them on `ubuntu-latest`, where `just test-ci` runs the integration
suite natively, and again on the
[Linux desktop legs](#what-the-linux-desktop-legs-cover-and-what-they-do-not).

### Running integration tests

The integration suite comes in two tiers:

- **`just test` and `just test-integration` are desktop-safe.** Tests that
  would drive the real cursor, keyboard or overlays skip themselves, so you can
  keep working while they run.
- **`just test-desktop` includes those tests.** It sets `NERU_DESKTOP_TESTS=1`,
  and the cursor moves, real clicks and scrolls land, overlays flash, and an
  event tap briefly intercepts the keyboard. Hand the machine over while it
  runs. On macOS your terminal needs Accessibility permission (System Settings,
  Privacy & Security, Accessibility). `just test-all` includes them too.

For either tier:

- **Quit any running `neru` daemon first.** A live daemon holds the IPC socket,
  which makes the IPC integration tests skip, so a green run did not test
  IPC.
- The integration recipes run with `-p 1`, one package at a time, because the
  tests share one input device, daemon sockets and log files. They also run
  with `-count=1`, because Go's test cache cannot see whether Accessibility was
  granted or a daemon held the socket, so a cached pass may come from a run
  under different conditions.

## Debugging

Debug logging and where the log file is written are described in
[Log file locations](../guide/troubleshooting.md#log-file-locations). File
logging is off by default, so turn it on before looking for a log.

For a step debugger:

```bash
dlv debug ./cmd/neru
```

What belongs at which log level, and what must never be logged, is in the root
[AGENTS.md](../../AGENTS.md) under Conventions.

## Adding code

### Where things go

| Directory                  | Role                                               |
| -------------------------- | -------------------------------------------------- |
| `internal/domain/`         | Pure business logic, entities, value objects       |
| `internal/ports/`          | Interface contracts (Accessibility, Overlay, Font) |
| `internal/adapter/`        | Platform-specific adapter implementations          |
| `internal/app/`            | Application orchestration, services, modes         |
| `internal/app/components/` | Mode-specific overlay rendering                    |
| `internal/app/modes/`      | Navigation mode implementations                    |
| `internal/cli/`            | Cobra CLI commands, IPC dispatch                   |
| `internal/config/`         | TOML parsing, validation, defaults                 |

Layer responsibilities and the boundaries between them are in
[Component architecture](architecture.md#component-architecture). Platform
file-slot naming is in [File Layout Rules](porting.md#file-layout-rules).

**Configuration options**: the full chain (schema, defaults, platform
overrides, validation, examples, docs) is documented in
[internal/config/AGENTS.md](../../internal/config/AGENTS.md). The
`neru-add-config-option` skill in `.agents/skills/` walks it step by step.

**Actions**

1. Define the action in `internal/domain/action/action.go`
2. Implement logic in `internal/app/services/action_service.go`
3. Wire pending-action dispatch in `internal/app/modes/mode_handlers.go` (the
   per-mode files set it via `Context.SetPendingAction`)
4. Update config and documentation

**UI components**

1. Create the component in `internal/app/components/`
2. Implement drawing in `internal/adapter/overlay/render/`
3. macOS Objective-C goes in `internal/adapter/platform/darwin/` behind
   `//go:build darwin`, with a no-op stub elsewhere
4. Build the render overlay in
   `internal/adapter/overlay/manager/components.go`, since the overlay
   constructs what it draws, and assemble the app-side component in
   `internal/app/component_factory.go`

**CLI commands**: a cobra command in `internal/cli/` (registered in an
`init()`), the matching IPC handler in `internal/app/ipcctrl/`, `just genman`,
and the [CLI reference](../reference/cli.md). The `neru-add-cli-command` skill
walks it step by step.

**Mode flags**: one entry in the descriptor table in
`internal/domain/modecmd`, then `just genflagref`. The entry registers the flag
on every command that accepts it and writes its row in the
[CLI reference](../reference/cli.md). An architecture test fails while either
is missing. A flag also declares which platforms writing it does anything on,
in `platform_support.go` beside that table, and so do the config options and
the action names in their own packages. Then run `just gensupportref`. An
architecture test fails while a word has no column, and the daemon warns once
at load about the inert ones a configuration writes.

### Dependency injection

Wiring is manual and explicit. Constructors take their dependencies, and
`internal/app/new.go` assembles everything in numbered phases that unwind in
reverse on failure.

`app.New` takes functional options ([options.go](../../internal/app/options.go)),
which is how tests substitute doubles for the ports they need: `WithSystemPort`,
`WithAccessibility`, `WithEventTap`, `WithIPCServer`, `WithOverlayPort`,
`WithHotkeyService`, `WithWatcher`, `WithTextInput`, plus `WithConfig`,
`WithConfigPath`, `WithLogger` and the config-load carriers `WithWrittenConfig`
and `WithConfigWarnings`. An option that is not supplied falls back to the real
adapter built during initialization.

```go
hintService := services.NewHintService(accAdapter, overlayAdapter, systemPort, hintGen, cfg.Hints, logger, visionPort)
gridService := services.NewGridService(overlayAdapter)
actionService := services.NewActionService(accAdapter, overlayAdapter, systemPort, logger)
```

### Mode interface contract

Every navigation mode implements `Mode` (`Activate(modecmd.Activation)`,
`HandleKey(string)`, `Exit()`, `ModeType()`,
`RefreshForMonitorMove(context.Context, image.Rectangle)`), defined in
[handler.go](../../internal/app/modes/handler.go). Each mode is its own type
with its own bodies for the four behavioural methods. The shape a new mode has
to follow is stated in
[internal/app/modes/AGENTS.md](../../internal/app/modes/AGENTS.md). A new CLI
flag that varies a mode's activation means a new flag descriptor in
[internal/domain/modecmd](../../internal/domain/modecmd) and the `Activation`
field it writes, not a new interface method.

All four run with the handler lock already held. The full locking contract
lives in [internal/app/modes/AGENTS.md](../../internal/app/modes/AGENTS.md).
Read it before touching anything that calls back into the handler.

## Release process

Releases are automated by
[Release Please](https://github.com/googleapis/release-please). Merging the
release PR builds and publishes the binaries on GitHub.

Versioning is semantic, `vMAJOR.MINOR.PATCH`, for breaking changes,
backward-compatible features and bug fixes. Release Please derives the
changelog from the commit subjects on `main`, and because pull requests
squash-merge, each of those is a PR title. So the
[conventional commit format](../../CONTRIBUTING.md#commit-messages) applied to
the title is what ships to users.

> [!NOTE]
> The Homebrew version bump happens separately, in its own repo.
