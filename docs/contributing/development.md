# Development guide

Set up, build, test and debug Neru, and decide where new code goes. The
contribution process is in [CONTRIBUTING.md](../../CONTRIBUTING.md) and code
conventions are in the root [AGENTS.md](../../AGENTS.md).

## Quick start

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru
oku sync && oku allow   # or: brew install go just golangci-lint llvm
just build
./bin/neru launch       # runs in the foreground
./bin/neru hints        # from a second terminal, should show hint overlays
```

> [!IMPORTANT]
> On macOS the app that starts the daemon needs Accessibility permission. When
> you run `./bin/neru launch` by hand, that app is your terminal. Without it the
> smoke test reports an accessibility error instead of drawing overlays.

There is no `just run` recipe. The daemon and the CLI are the same
`./bin/neru` binary. On Linux, install the
[build dependencies](#build-dependencies) before your first build. To install
Neru rather than develop it, see [installation](../guide/installation.md).

## Development setup

### Prerequisites

- **Go 1.26+**, **Just** and **golangci-lint**, from oku or by hand (below)
- **Xcode Command Line Tools** (macOS): `xcode-select --install`
- **Build dependencies** (Linux): see [Build dependencies](#build-dependencies)
- An EditorConfig plugin, for the `.editorconfig` rules CI enforces

### Option A: oku (recommended)

[oku](https://github.com/y3owk1n/oku) installs the tools `oku.toml` lists, at
the versions `oku.lock` pins, into a profile for this repo:

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/oku/main/install.sh | sh
oku sync               # install the toolchain
oku allow              # let the shell hook put it on PATH inside this repo
```

The installer's shell hook activates the profile when you `cd` in, or put
`eval "$(oku env --shell bash)"` in a [direnv](https://direnv.net/) `.envrc`.
CI installs the same versions from `oku.lock`. `oku update` moves each tool to
the newest version `oku.toml` allows, and `oku outdated` shows what is behind.

### Option B: manual installation

```bash
brew install go just golangci-lint llvm   # llvm supplies clang-format
```

Match the golangci-lint version that `oku.lock` pins, which CI uses. gopls,
gofumpt and golines are optional, through `go install`.

## Common tasks

`just --list` shows every recipe, including the Wayland protocol and icon ones.

| Command | Description |
| ------- | ----------- |
| `just build` | Compile for the current platform |
| `just build-darwin`, `build-linux [ARCH]`, `build-windows [ARCH]` | Build for one OS. `ARCH` defaults to amd64 |
| `just build-version v1.0.0` | Build with an explicit version string |
| `just release` | Optimized, stripped release build |
| `just dist` | Assemble the release layout (bin, man, `Neru.app`) in `build/dist` |
| `just install [-y]` | Build, `dist`, then run the release installer, `-y` auto-accepts |
| `just test` | Unit + integration, desktop-safe |
| `just test-unit`, `test-integration` | One tier only, desktop-safe |
| `just test-desktop` | Integration including the tests that drive the real cursor, keyboard and overlays |
| `just test-foundation` | Fast cross-platform-safe slice, CI runs it too |
| `just test-race`, `test-race-unit`, `test-race-integration` | The same tiers with `-race` |
| `just test-all` | `test` and `test-race`, desktop tests included: the deepest sweep |
| `just test-ci` | What CI gates on: foundation, unit, race-unit, short integration |
| `just test-linux` | Run the Linux test suite in a CI-equivalent container (Docker) |
| `just coverage`, `coverage-html` | Unit coverage as a total, or as a browsable `coverage.html` |
| `just lint` | golangci-lint, plus clang-tidy on `.m` files (macOS) |
| `just vet`, `vuln` | `go vet`, and `govulncheck` for reachable CVEs in dependencies |
| `just check-cross` | CGO-off type-check of the Linux and Windows builds |
| `just lint-cross` | Lint the Linux build with CGO on, in Docker |
| `just ci` | The pre-push gate: the checks CI gates on, on this host |
| `just fmt`, `fmt-check` | Format Go and Objective-C, or check Objective-C formatting |
| `just genman` | Generate man pages |
| `just genflagref` | Rewrite the mode-flag reference in `docs/reference/cli.md` |
| `just gensupportref` | Rewrite the platform-support table in `docs/reference/platform-support.md` |
| `just clean` | Remove build artifacts |

Targeting a single package or test:

```bash
go test ./internal/domain/hint/
go test -run TestScrollMode_HandleKey_DoesNothing ./internal/app/modes/
go test -tags=integration ./internal/adapter/accessibility/
```

### What `just ci` covers, and what it does not

`just ci` runs `fmt-check lint vet build check-cross test-ci vuln`, in that
order, on this host. CI runs the same recipes on macOS, Linux and Windows, so a
green local run covers one leg of three. For the deepest local check on a real
desktop session, also run `just test-all`.

| Check | Covers | Misses |
| ----- | ------ | ------ |
| `just lint`, `just vet` | the host build | every other GOOS |
| `just check-cross` (in `just ci`, no Docker) | plain `//go:build linux` and `//go:build windows` files, CGO off | cgo-tagged files |
| `just lint-cross`, `just test-linux` (Docker) | the cgo Linux paths | left out of `just ci` so it runs without Docker ([ADR 0012](../adr/0012-the-first-hour-must-not-lie.md)). CI runs both on every push |

### What the Linux desktop legs cover, and what they do not

Two required CI jobs run `just test-desktop`, which takes over a local
machine, in a disposable Linux session with D-Bus and AT-SPI. Both need
`CGO_ENABLED=1` and an answering accessibility bus, and check their session
before any test runs.

| Job | Session | Required | Bar under [ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md) |
| --- | ------- | -------- | --- |
| `desktop (ubuntu-latest)` | sway on the wlroots headless backend, Xwayland off | `zwlr_layer_shell_v1`, `zwlr_virtual_pointer_manager_v1` | behavioral parity (the blessed stack) |
| `desktop-x11 (ubuntu-latest)` | Xvfb with openbox, for `_NET_ACTIVE_WINDOW` | XTEST, XFIXES, RANDR, an EWMH window manager | capability parity |

The X11 leg exercises XTest, `XGrabKey`, `XGrabKeyboard`, XRandR,
`XGetImage`, the XFixes overlay shape, `_NET_ACTIVE_WINDOW` and `WM_CLASS`
identity, and `Xft.dpi` scaling. Both legs run with nothing focused, the only
place CI covers that state for the capability contract tests in
`internal/adapter/platform`. A leg fails if nothing executed or a test skipped
because it could not see the display server.

Neither leg covers:

- **Most desktop-driving tests**, which are macOS tests. The Linux ones are
  the smooth-scroll measurement, the X11 keyboard-grab and layout tests, and
  `neru services`, which also needs a systemd user manager.
- **KDE**, **a focused window**, and **behavioral parity on X11**.
- **Device access.** No `/dev/dri` and no `input` group, so the evdev and
  uinput paths cannot run.

## Building

### Without Just

```bash
VERSION=$(git describe --tags --always --dirty)
go build -trimpath -o bin/neru \
  -ldflags="-s -w -X github.com/y3owk1n/neru/internal/buildinfo.Version=$VERSION" ./cmd/neru
```

### Cross-platform checks

Before or during Linux or Windows work, run `just build && just test-foundation`.
`just test-foundation` runs every package whose behavior is identical on all
three platforms, so a failure there is a real cross-platform regression. Only
the target OS can run `just test` meaningfully, because integration tests are
tagged per OS.

- `just build-windows` cross-compiles from any host, with CGO off.
- `just build-linux` refuses unless the C compiler targets Linux, because Linux
  needs CGO. From macOS, use `just check-cross` or `just lint-cross`. CI builds
  tagged Linux release binaries on a native runner.
- `just build-linux arm64` on another architecture needs an arm64 C compiler
  as `CC`. Check the result with `file bin/neru`.
- `just lint` sees only the host platform. Reproduce Linux findings with
  `CGO_ENABLED=0 GOOS=linux golangci-lint run ./internal/...` and ignore the
  `unused` and `unparam` reports caused by the excluded `*_cgo.go` files.

CGO and modifier expectations are per backend, not per OS. Start from
[profile.go](../../internal/adapter/platform/profile.go) and
[CGO guidance](porting.md#cgo-guidance).

## Building on Linux

### Build dependencies

A Linux build uses CGO and links against distribution libraries that oku does
not provide. `just linux-deps` installs them, with a C compiler and pkg-config,
through apt, dnf or pacman.

- On Fedora 42 and later, also install `liboeffis-devel`, which ships
  separately there and which the recipe does not list.
- On another distribution, install the equivalents of: cairo, wayland and
  wayland-protocols, libX11, libXtst, libXrandr, libXrender, libXext,
  libXfixes, libxkbcommon, libei and liboeffis, fontconfig, tesseract with
  English data, pipewire, and the DejaVu fonts.

The build links libei and liboeffis even if you only test on wlroots. DejaVu
is the default font when `font_family` is unset. Runtime roles of each library
are in the [Linux setup guide](../guide/linux.md).

## Testing

| Layer | What it checks | File pattern, tag | Command |
| ----- | -------------- | ----------------- | ------- |
| Unit | shared Go logic, using mocks from `internal/ports/mocks` | `*_test.go`, none | `just test-unit` |
| Contract | ports and adapters agree on error semantics such as `CodeNotSupported` | `*_test.go`, an OS tag where needed | `just test-unit` |
| Integration | real OS behavior | `*_integration_<os>_test.go`, `integration && <os>` | `just test-integration` |
| Architecture | package boundaries and platform isolation in `internal/architecture/` | `*_test.go`, none | `just test-unit` |

When you stub a platform feature, update the subsystem's contract test, or
write one if a caller could read the stub's `nil` as success
([platform/AGENTS.md](../../internal/adapter/platform/AGENTS.md) names them).
Test naming and mocks are in the root [AGENTS.md](../../AGENTS.md), and the
macOS main-run-loop harness in
[darwin/AGENTS.md](../../internal/adapter/platform/darwin/AGENTS.md).

Integration tests are mostly macOS. Linux has a few in
`internal/adapter/platform/linux/` plus the evdev probe, smooth-scroll and
`neru services` tests. Windows has services, the overlay transition and the
UIA tree walk. From a macOS host, run the Linux ones in the container:

```bash
docker run --rm -v "$PWD":/src -w /src -e CGO_ENABLED=1 neru-linux-ci \
  go test -tags=integration ./...
```

CI runs them on `ubuntu-latest` through `just test-ci`, and on the
[Linux desktop legs](#what-the-linux-desktop-legs-cover-and-what-they-do-not).

### Running integration tests

- **`just test` and `just test-integration` are desktop-safe.** Tests that
  would drive the real cursor, keyboard or overlays skip themselves.
- **`just test-desktop` and `just test-all` include them.** They set
  `NERU_DESKTOP_TESTS=1` and take over the cursor and keyboard while they run.
  On macOS your terminal needs Accessibility permission.
- **Quit any running `neru` daemon first.** A live daemon holds the IPC socket,
  which makes the IPC integration tests skip.
- The recipes run with `-p 1` because the tests share one input device, daemon
  sockets and log files, and with `-count=1` because Go's test cache cannot see
  whether Accessibility was granted or a daemon held the socket.

## Debugging

File logging is off by default. To turn it on, see
[Log file locations](../guide/troubleshooting.md#log-file-locations). For a step
debugger, run `dlv debug ./cmd/neru`.

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

Layer boundaries are in
[Component architecture](architecture.md#component-architecture), and platform
file names in [File Layout Rules](porting.md#file-layout-rules).

**Configuration options**: follow [config/AGENTS.md](../../internal/config/AGENTS.md)
or the `neru-add-config-option` skill.

**Actions**:

1. Define the action in `internal/domain/action/action.go`
2. Implement it in `internal/app/services/action_service.go`
3. Wire pending-action dispatch in `internal/app/modes/mode_handlers.go` (the
   per-mode files set it via `Context.SetPendingAction`)
4. Update config and documentation

**UI components**:

1. Create the component in `internal/app/components/`
2. Implement drawing in `internal/adapter/overlay/render/`
3. Put macOS Objective-C in `internal/adapter/platform/darwin/` behind
   `//go:build darwin`, with a no-op stub elsewhere
4. Build the render overlay in `internal/adapter/overlay/manager/components.go`
   and the app-side component in `internal/app/component_factory.go`

**CLI commands**: a cobra command in `internal/cli/` (registered in an
`init()`), the IPC handler in `internal/app/ipcctrl/`, `just genman`, and the
[CLI reference](../reference/cli.md). The `neru-add-cli-command` skill walks it.

**Mode flags**: add one entry to the descriptor table in
`internal/domain/modecmd`, then run `just genflagref`. The entry registers the
flag on every command that accepts it and writes its
[CLI reference](../reference/cli.md) row. Declare its platforms in
`platform_support.go` beside the table, as config options and actions do in
their own packages, then run `just gensupportref`. Architecture tests fail
while either is missing, and the daemon warns once at load about inert words a
configuration writes.

### Dependency injection

Constructors take their dependencies, and `internal/app/new.go` assembles them
in numbered phases that unwind in reverse on failure. `app.New` takes
functional options ([options.go](../../internal/app/options.go)), which tests
use to substitute doubles: `WithSystemPort`, `WithAccessibility`,
`WithEventTap`, `WithIPCServer`, `WithOverlayPort`, `WithHotkeyService`,
`WithWatcher`, `WithTextInput`, `WithConfig`, `WithConfigPath`, `WithLogger`,
`WithWrittenConfig` and `WithConfigWarnings`. An option left out falls back to
the real adapter.

### Mode interface contract

Every navigation mode implements `Mode` (`Activate(modecmd.Activation)`,
`HandleKey(string)`, `Exit()`, `ModeType()`,
`RefreshForMonitorMove(context.Context, image.Rectangle)`) from
[handler.go](../../internal/app/modes/handler.go). A flag that varies
activation is a [modecmd](../../internal/domain/modecmd) descriptor plus an
`Activation` field, not a new method. The shape of a new mode and its locking
contract are in [modes/AGENTS.md](../../internal/app/modes/AGENTS.md).

## Release process

Merging the [Release Please](https://github.com/googleapis/release-please) PR
publishes a semantic version and its binaries on GitHub, with a changelog built
from the [PR titles](../../CONTRIBUTING.md#commit-messages) on `main`. The
Homebrew bump happens in its own repo.
