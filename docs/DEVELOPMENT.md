# Development Guide

How to set up a development environment, build, test, and debug Neru, and where
to put new code.

This guide owns the **local workflow**. Neighbouring documents own the rest:
contribution process and commit conventions in
[CONTRIBUTING.md](../CONTRIBUTING.md), the architectural reference in
[ARCHITECTURE.md](ARCHITECTURE.md), per-platform support and platform file
layout in [CROSS_PLATFORM.md](CROSS_PLATFORM.md), and style rules in
[CODING_STANDARDS.md](CODING_STANDARDS.md). None of those are repeated here.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Development Setup](#development-setup)
- [Common Tasks](#common-tasks)
- [Building](#building)
- [Testing](#testing)
- [Debugging](#debugging)
- [Adding Code](#adding-code)
- [Release Process](#release-process)
- [Resources](#resources)

---

## Quick Start

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

devbox shell            # or: brew install go just golangci-lint llvm
just build
./bin/neru launch       # runs in the foreground
```

Then from a second terminal:

```bash
./bin/neru hints        # should show hint overlays
```

There is no `just run` recipe — build first, then launch the daemon directly.
The CLI talks to the running daemon over a socket, so both halves come from the
same `./bin/neru` binary.

For end-user installation (Homebrew, Nix, prebuilt binaries) see
[INSTALLATION.md](INSTALLATION.md); for preparing a Linux host see
[LINUX_SETUP.md](LINUX_SETUP.md).

---

## Development Setup

### Prerequisites

- **Go 1.26+** — [Install Go](https://golang.org/dl/)
- **Xcode Command Line Tools** (macOS) — `xcode-select --install`
- **Just** — command runner — [install](https://github.com/casey/just)
- **golangci-lint** — linter — [install](https://golangci-lint.run/usage/install/)

### Option A: Devbox (recommended)

[Devbox](https://www.jetify.com/devbox) provides an isolated environment with
every tool pre-configured:

```bash
curl -fsSL https://get.jetify.com/devbox | bash

devbox shell            # enter the shell manually
```

Or let [direnv](https://direnv.net/) activate it automatically — install direnv,
add `eval "$(direnv hook bash)"` (or zsh/fish) to your shell, and the `.envrc`
in the repo root takes over whenever you `cd` in.

Devbox manages Go 1.26+, gopls, gotools, gofumpt, golines, golangci-lint, just,
and clang-tools (for CGo).

### Option B: Manual installation

```bash
brew install go just golangci-lint llvm
```

`llvm` supplies `clang-format` for Objective-C formatting. Devbox's extra tools
are optional and installable on their own:

```bash
go install golang.org/x/tools/gopls@latest
go install mvdan.cc/gofumpt@latest
go install github.com/segmentio/golines@latest
```

### Verify

```bash
go version              # 1.26+
just --version
golangci-lint --version
just --list             # all available recipes
```

An EditorConfig plugin is worth installing — `.editorconfig` carries the tab and
line-ending rules that CI enforces.

---

## Common Tasks

Every build, test, and lint entry point goes through `just`. This table is the
single reference for them; `just --list` shows the full set including the
Wayland protocol generation and icon recipes.

| Task    | Command                      | Description                                     |
| ------- | ---------------------------- | ----------------------------------------------- |
| Build   | `just build`                 | Compile for the current platform                |
| Build   | `just build-darwin`          | Build a macOS binary (on macOS)                 |
| Build   | `just build-linux [ARCH]`    | Build a Linux binary (defaults to amd64)        |
| Build   | `just build-windows [ARCH]`  | Build a Windows binary                          |
| Build   | `just build-version v1.0.0`  | Build with an explicit version string           |
| Build   | `just release`               | Optimized, stripped release build               |
| Bundle  | `just bundle`                | Release build + macOS `Neru.app` (ad-hoc signed) |
| Install | `just install [-y]`          | Install an already-built Neru; `-y` auto-accepts |
| Test    | `just test`                  | Unit + integration                              |
| Test    | `just test-unit`             | Unit tests only                                 |
| Test    | `just test-integration`      | Integration tests only                          |
| Test    | `just test-foundation`       | Fast cross-platform-safe slice                  |
| Test    | `just test-race`             | Unit + integration with `-race`                 |
| Test    | `just test-race-unit`        | Unit tests with `-race`                         |
| Test    | `just test-race-integration` | Integration tests with `-race`                  |
| Test    | `just test-all`              | `test` **and** `test-race` — the full sweep     |
| Test    | `just coverage`              | Unit tests with coverage; prints the total      |
| Test    | `just coverage-html`         | Coverage as a browsable `coverage.html`         |
| Lint    | `just lint`                  | `golangci-lint run`                             |
| Lint    | `just vet`                   | `go vet`                                        |
| Lint    | `just vuln`                  | `govulncheck` — reachable CVEs in dependencies  |
| Format  | `just fmt`                   | Format Go and Objective-C                       |
| Format  | `just fmt-check`             | Check Objective-C formatting                    |
| Docs    | `just genman`                | Generate man pages                              |
| Clean   | `just clean`                 | Remove build artifacts                          |

Targeting a single package or test:

```bash
go test ./internal/domain/hint/
go test -run TestHandler_HandleKey ./internal/app/modes/
go test -tags=integration ./internal/adapter/accessibility/
```

Watch mode, if you have [entr](https://eradman.com/entrproject/):

```bash
find . -name "*.go" | entr -r just test
```

---

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

- `-ldflags="-s -w"` — strip debug info and symbol table (smaller binary)
- `-trimpath` — remove filesystem paths from the binary
- `-X pkg.Var=value` — inject the version at build time

### Cross-platform baseline

Starting Linux or Windows work? The minimum smoke test is:

```bash
just build
just test-foundation
```

then `just build-linux` / `just build-windows` / `just build-darwin` for your
target. Cross-compiled binaries build from any host, but only the target OS can
run `just test` meaningfully — integration tests are tagged per-OS, and
cross-compiling to Linux from macOS is not supported (CGO plus Linux headers).

Backend, CGO, and modifier expectations are **not** per-OS constants; start from
[profile.go](../internal/adapter/platform/profile.go) and
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#cgo-guidance).

---

## Testing

Neru has four testing layers:

1. **Unit tests** — shared Go logic with no native OS dependency, using mocks
   from `internal/ports/mocks`.
2. **Contract tests** — ports and adapters agreeing on error semantics such as
   `CodeNotSupported`.
3. **Integration tests** — real OS/native behavior behind the `integration`
   build tag.
4. **Architecture tests** — guardrails protecting package boundaries and
   platform isolation (`internal/architecture/`).

When you add a stubbed platform feature, add or update a contract test so the
unsupported behavior is explicit and stable until the real implementation lands.

### Organization

| Type            | File pattern                 | Build tag             | Command                 |
| --------------- | ---------------------------- | --------------------- | ----------------------- |
| **Unit**        | `*_test.go`                  | —                     | `just test-unit`        |
| **Integration** | `*_integration_<os>_test.go` | `integration && <os>` | `just test-integration` |

Tests are table-driven and named `TestType_Method_EdgeCase`. Detailed patterns
live in [TESTING_PATTERNS.md](testing/TESTING_PATTERNS.md).

### What each layer covers

**Unit** — hint generation, grid calculations, element filtering, action
processing, mode transitions, config parsing/validation/defaults, and CLI
argument handling. These run everywhere.

**Integration** — today these are **macOS only**: real Accessibility and event
tap APIs, global hotkey registration, overlay and window management, Unix socket
IPC, config file loading and reloading, and service-to-adapter coordination.
`*_integration_linux_test.go` and `*_integration_windows_test.go` are the
reserved slots — no such tests exist yet, so Linux and Windows behavior is
currently pinned by unit and contract tests alone. Adding real ones is one of
the more valuable contributions available.

### Running integration tests

Integration tests exercise the **real OS**, and on macOS that has consequences
worth knowing before your first run:

- **They move your cursor and type keystrokes.** Don't run them while you're
  typing in another window; the tests and you are sharing one physical input
  device.
- **Your terminal needs Accessibility permission** (System Settings → Privacy &
  Security → Accessibility). Without it, accessibility- and event-tap-backed
  tests fail with permission errors rather than skipping.
- **Quit any running `neru` daemon first.** A live daemon holds the IPC socket,
  which makes the IPC integration tests silently skip — a green run that tested
  less than you think.
- `just test-integration` runs with `-p 1` (one package at a time — concurrent
  packages would fight over the one physical cursor) and `-count=1` (no test
  cache — Go's cache can't see whether Accessibility was granted or a daemon
  held the socket, so a cached pass may be from a run under different
  conditions).

---

## Debugging

Enable debug logging in `~/.config/neru/config.toml`:

```toml
[logging]
log_level = "debug"
```

Then follow the log:

```bash
tail -f ~/Library/Logs/neru/app.log       # macOS
tail -f ~/.local/state/neru/log/app.log   # Linux
```

For a step debugger:

```bash
dlv debug ./cmd/neru
```

What belongs at which log level — and what must never be logged — is in
[CODING_STANDARDS.md](CODING_STANDARDS.md#logging-standards).

---

## Adding Code

### Where things go

| Directory                  | Role                                               |
| -------------------------- | -------------------------------------------------- |
| `internal/domain/`    | Pure business logic, entities, value objects       |
| `internal/ports/`     | Interface contracts (Accessibility, Overlay, Font) |
| `internal/adapter/`     | Platform-specific adapter implementations          |
| `internal/app/`            | Application orchestration, services, modes         |
| `internal/app/components/` | Mode-specific overlay rendering                    |
| `internal/app/modes/`      | Navigation mode implementations                    |
| `internal/ui/`             | Coordinate conversion, abstract rendering          |
| `internal/cli/`            | Cobra CLI commands, IPC dispatch                   |
| `internal/config/`         | TOML parsing, validation, defaults                 |

Layer responsibilities and the boundaries between them are in
[ARCHITECTURE.md](ARCHITECTURE.md#component-architecture); platform file-slot
naming is in [CROSS_PLATFORM.md](CROSS_PLATFORM.md#file-layout-rules).

**Configuration options**

1. Add fields to the structs in `internal/config/config.go`
2. Shared defaults in `newDefaultConfig()` (`config_defaults.go`); platform
   overrides in `applyPlatformDefaults()` (`config_<os>.go`)
3. Validation in the `Validate*()` methods
4. Update `configs/` examples and [CONFIGURATION.md](CONFIGURATION.md)

**Actions**

1. Define the action in `internal/domain/action/action.go`
2. Implement logic in `internal/app/services/action_service.go`
3. Wire pending-action dispatch in `internal/app/modes/mode_handlers.go` (the
   per-mode files set it via `Context.SetPendingAction`)
4. Update config and documentation

**UI components**

1. Create the component in `internal/app/components/`
2. Implement rendering in `internal/ui/`
3. macOS Objective-C goes in `internal/adapter/platform/darwin/` behind
   `//go:build darwin`, with a no-op stub elsewhere
4. Register in `internal/app/component_factory.go` or
   `internal/app/new.go`

**CLI commands**

1. Create the command file in `internal/cli/`
2. Register it in an `init()` (see `internal/cli/root.go`)
3. Add the matching IPC handler in `internal/app/ipcctrl/`
4. Document in [CLI.md](CLI.md)

### Dependency injection

Wiring is manual and explicit — constructors take their dependencies, and
`internal/app/new.go` assembles everything in numbered phases
that unwind in reverse on failure.

`app.New` takes functional options ([options.go](../internal/app/options.go)),
which is how tests substitute doubles for the ports they need — `WithSystemPort`,
`WithEventTap`, `WithIPCServer`, `WithOverlayManager`, `WithHotkeyService`,
`WithWatcher`, plus `WithConfig` / `WithConfigPath` / `WithLogger`. An option
that is not supplied falls back to the real adapter built during initialization.

```go
hintService := services.NewHintService(accAdapter, overlayAdapter, systemPort, hintGen, cfg.Hints, logger, visionPort)
gridService := services.NewGridService(overlayAdapter, systemPort, logger)
actionService := services.NewActionService(accAdapter, overlayAdapter, systemPort, logger)
```

### Mode interface contract

Every navigation mode implements `Mode`, defined in
[handler.go](../internal/app/modes/handler.go):

```go
type Mode interface {
    // Activate activates the mode with an optional pending action.
    Activate(opts ModeActivationOptions)

    // HandleKey processes a key press within the mode's context.
    HandleKey(key string)

    // Exit performs mode-specific cleanup and deactivation.
    Exit()

    // ModeType returns the domain mode type this implementation represents.
    ModeType() domain.Mode
}
```

`ModeActivationOptions` ([hints.go](../internal/app/modes/hints.go)) carries
every per-activation override — `Action`, `Modifier`, `OnExit`, `Repeat`,
`CursorFollowSelection`, `ZoomToDepth`, `FilterRoles`, `FilterTextContains`,
`Search`, `HideOnEmptySearch`, `Strategy`, `LabelDirection`, `Toggle`,
`SplitWord`. A new CLI flag that varies a mode's activation usually means a new
field here rather than a new interface method.

> **Locking contract.** `Activate`, `HandleKey`, and `Exit` are all called with
> the handler lock **already held** by the public entry point (`ActivateMode`,
> `HandleKeyPress`, `ExitMode`). Modes are built on the inner `*handlerState`,
> which has no mutex and no access to the locking entry points, so calling one
> from locked context is a compile error. Deferred callbacks (timers,
> goroutines) reach the lock through `handlerState.outer` — see
> [ARCHITECTURE.md](ARCHITECTURE.md#mode-handler-locking).

**Method contracts**

- `Activate(opts)` — call `handler.setMode()` to change app state, show
  mode-specific overlays, initialize state from `opts`. Log activation at
  `info`; keep routing and redraw detail at `debug`.
- `HandleKey(key)` — route a single key string (`"a"`, `"j"`, `"escape"`) to the
  right handler and update mode state.
- `Exit()` — hide overlays, reset mode-specific state. Common cleanup is already
  handled by `exitMode`.
- `ModeType()` — return the `domain.Mode` enum value (e.g. `domain.ModeHints`).

**Implementation pattern**

Modes are not written as bare structs implementing four methods by hand. Two
shared building blocks cover almost every case:

- **`baseMode`** ([base.go](../internal/app/modes/base.go)) — holds the handler
  and mode type, supplies no-op `Activate` / `HandleKey` / `Exit` plus a real
  `ModeType`. Embed it and override only what differs.
- **`GenericMode`** ([generic_mode.go](../internal/app/modes/generic_mode.go)) —
  embeds `baseMode` and takes a `ModeBehavior` of optional `ActivateFunc` /
  `HandleKeyFunc` / `ExitFunc` callbacks. A nil callback falls back to the
  handler's standard flow for that mode type.

So a mode is usually just a constructor supplying behavior. `ScrollMode` in
full:

```go
type ScrollMode struct {
    *GenericMode
}

func NewScrollMode(handler *handlerState) *ScrollMode {
    behavior := ModeBehavior{
        ActivateFunc: func(handler *handlerState, _ ModeActivationOptions) {
            // Runs with the lock held; handlerState methods are safe to call.
            handler.startInteractiveScroll()
            handler.startIndicatorPolling(domain.ModeScroll)
        },
        ExitFunc: func(handler *handlerState) {
            handler.stopIndicatorPolling()
            handler.stopHeldRepeat()
            handler.clearAndHideOverlay()
            // ... reset mode-specific state
        },
    }

    return &ScrollMode{
        GenericMode: NewGenericMode(handler, domain.ModeScroll, "ScrollMode", behavior),
    }
}
```

Reach for a hand-written struct embedding `baseMode` only when the mode needs
its own fields or a `HandleKey` too involved for a callback.

**Adding a new mode**

1. Add the `Mode` constant to `internal/domain/domain_constants.go`
2. Implement the `Mode` interface in `internal/app/modes/` — name it `XXXMode`
   with a `NewXXXMode` constructor
3. Register it in the map built by `NewHandler`:

    ```go
    handler.modes = map[domain.Mode]Mode{
        domain.ModeHints:         NewHintsMode(handler),
        domain.ModeGrid:          NewGridMode(handler),
        domain.ModeScroll:        NewScrollMode(handler),
        domain.ModeRecursiveGrid: NewRecursiveGridMode(handler),
        domain.ModeMonitorSelect: NewMonitorSelectMode(handler),
        // Add new modes here
    }
    ```

4. Add the CLI command and IPC handler (see [CLI commands](#where-things-go))
5. Add hotkey defaults to the config
6. Add unit and integration tests
7. Document in [CLI.md](CLI.md) and [CONFIGURATION.md](CONFIGURATION.md)

---

## Release Process

Releases are automated by
[Release Please](https://github.com/googleapis/release-please). Merging the
release PR builds and publishes the binaries on GitHub.

Versioning is semantic — `vMAJOR.MINOR.PATCH`: breaking changes, backward-
compatible features, bug fixes. Because Release Please derives the changelog
from commit subjects, the [conventional commit
format](../CONTRIBUTING.md#commit-messages) is what ships to users.

> [!NOTE]
> The Homebrew version bump lives in a separate repo and is updated separately.

---

## Resources

- [Go Documentation](https://golang.org/doc/)
- [macOS Accessibility API](https://developer.apple.com/documentation/applicationservices/ax_ui_element_ref)
- [TOML Spec](https://toml.io/)
- [Cobra](https://github.com/spf13/cobra)
- [Just](https://github.com/casey/just)
