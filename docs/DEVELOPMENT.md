# Development Guide

How to set up a development environment, build, test, and debug Neru, and where
to put new code.

This guide owns the **local workflow**. Neighbouring documents own the rest:
contribution process and commit conventions in
[CONTRIBUTING.md](../CONTRIBUTING.md), the architectural reference in
[ARCHITECTURE.md](ARCHITECTURE.md), per-platform support and platform file
layout in [CROSS_PLATFORM.md](CROSS_PLATFORM.md), and conventions in the root
[AGENTS.md](../AGENTS.md). None of those are repeated here.

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

> [!IMPORTANT]
> On macOS this needs Accessibility permission granted to whichever app starts
> the daemon — your terminal, when you run `./bin/neru launch` by hand. Without
> it the smoke test reports an accessibility error instead of drawing overlays.
> Same grant, same place as for the integration tests: see
> [Running integration tests](#running-integration-tests).

There is no `just run` recipe — build first, then launch the daemon directly.
The CLI talks to the running daemon over a socket, so both halves come from the
same `./bin/neru` binary.

For end-user installation (Homebrew, Nix, prebuilt binaries) see
[INSTALLATION.md](INSTALLATION.md); on Linux,
[LINUX_SETUP.md](LINUX_SETUP.md) covers both preparing the host and the
[build dependencies](LINUX_SETUP.md#build-dependencies) a source build needs.

---

## Development Setup

### Prerequisites

- **Go 1.26+** — [Install Go](https://golang.org/dl/)
- **Xcode Command Line Tools** (macOS) — `xcode-select --install`
- **Build dependencies** (Linux) — the system `-dev`/`-devel` packages a CGO
  build links against, listed for apt, dnf and pacman in
  [LINUX_SETUP.md](LINUX_SETUP.md#build-dependencies). Install them before your
  first build, including under Devbox.
- **Just** — command runner — [install](https://github.com/casey/just)
- **golangci-lint** — linter — [install](https://golangci-lint.run/usage/install/)

### Option A: Devbox (recommended)

[Devbox](https://www.jetify.com/devbox) provides an isolated environment with
the toolchain below pre-configured:

```bash
curl -fsSL https://get.jetify.com/devbox | bash

devbox shell            # enter the shell manually
```

Or let [direnv](https://direnv.net/) activate it automatically — install direnv,
add `eval "$(direnv hook bash)"` (or zsh/fish) to your shell, and the `.envrc`
in the repo root takes over whenever you `cd` in.

Devbox manages Go 1.26+, gopls, gotools, gofumpt, golines, golangci-lint, just,
and clang-tools (for CGo).

On Linux it is not enough on its own: Devbox does not pull the `-dev` outputs a
CGO build links against
([jetify-com/devbox#2761](https://github.com/jetify-com/devbox/issues/2761)),
so install the system packages listed in
[LINUX_SETUP.md](LINUX_SETUP.md#build-dependencies) first — `just build` fails
in the compiler without them.

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
| Test    | `just test`                  | Unit + integration (desktop-safe: never drives your cursor) |
| Test    | `just test-unit`             | Unit tests only                                 |
| Test    | `just test-integration`      | Integration tests only (desktop-safe)           |
| Test    | `just test-desktop`          | Integration incl. tests that drive the real cursor/keyboard/overlays |
| Test    | `just test-foundation`       | Fast cross-platform-safe slice; CI runs it too  |
| Test    | `just test-race`             | Unit + integration with `-race`                 |
| Test    | `just test-race-unit`        | Unit tests with `-race`                         |
| Test    | `just test-race-integration` | Integration tests with `-race`                  |
| Test    | `just test-all`              | `test` **and** `test-race`, desktop tests included — the deepest sweep |
| Test    | `just test-ci`               | What CI gates on: foundation, unit, race-unit, short-integration |
| Test    | `just coverage`              | Unit tests with coverage; prints the total      |
| Test    | `just coverage-html`         | Coverage as a browsable `coverage.html`         |
| Lint    | `just lint`                  | golangci-lint + clang-tidy on `.m` files (macOS) |
| Lint    | `just vet`                   | `go vet`                                        |
| Lint    | `just vuln`                  | `govulncheck` — reachable CVEs in dependencies  |
| Lint    | `just check-cross`           | CGO-off type-check of the Linux and Windows builds |
| Gate    | `just ci`                    | The pre-push gate: the checks CI gates on, on this host |
| Format  | `just fmt`                   | Format Go and Objective-C                       |
| Format  | `just fmt-check`             | Check Objective-C formatting                    |
| Docs    | `just genman`                | Generate man pages                              |
| Docs    | `just genflagref`            | Rewrite the mode-flag reference in `docs/CLI.md` |
| Clean   | `just clean`                 | Remove build artifacts                          |

### What `just ci` covers, and what it does not

`just ci` runs `fmt-check lint vet build check-cross test-ci vuln`, in that
order, on this host. CI runs the same recipes on macOS, Linux and Windows, so a
green run here is one leg of three.

`just check-cross` is the only member that looks at the other two targets. It
type-checks the Linux and Windows builds with CGO off, which catches a build
break in a plain `//go:build linux` or `//go:build windows` file — invisible to
`just lint` and `just vet`, since both compile for the host — but not one in a
cgo-tagged file, which CGO-off skips entirely. It costs seconds and needs no
Docker.

The cgo-only Linux paths need a real Linux toolchain. `just lint-cross` and
`just test-linux` provide one in a container, and CI checks them on every push.
Neither is part of `just ci`, deliberately: a documented pre-push gate that
fails for want of a running Docker daemon is worse than one that admits its
limits ([ADR 0012](adr/0012-the-first-hour-must-not-lie.md)).

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

Contract tests pin stub loudness per subsystem rather than per stub. When you
add a stubbed platform feature, update the subsystem's existing contract test if
it has one, and write a new one when a caller could read the stub's `nil` as
success — `internal/adapter/platform/AGENTS.md` states the rule and names the
tests that exist.

### Organization

| Type            | File pattern                 | Build tag             | Command                 |
| --------------- | ---------------------------- | --------------------- | ----------------------- |
| **Unit**        | `*_test.go`                  | —                     | `just test-unit`        |
| **Integration** | `*_integration_<os>_test.go` | `integration && <os>` | `just test-integration` |

Naming, mocks, and build-tag conventions are in the root
[AGENTS.md](../AGENTS.md), which is their single home; the macOS main-run-loop
test harness is documented in
[darwin/AGENTS.md](../internal/adapter/platform/darwin/AGENTS.md).

### What each layer covers

**Unit** — hint generation, grid calculations, element filtering, action
processing, mode transitions, config parsing/validation/defaults, and CLI
argument handling. These run everywhere.

**Integration** — almost all of these are **macOS**: real Accessibility and
event tap APIs, global hotkey registration, overlay and window management, Unix
socket IPC, config file loading and reloading, and service-to-adapter
coordination. Linux has exactly one so far — the fontconfig font resolver
(`internal/adapter/platform/linux/font_integration_linux_test.go`, which reads
what is installed with `fc-list` and skips where fontconfig has nothing to
report) — and Windows has none, so behavior on both is otherwise pinned by unit
and contract tests alone. Adding real ones is one of the more valuable
contributions available.

No `just` recipe runs the Linux ones: `just test-linux` runs the container
without the `integration` tag, and `just test-integration` runs on the host.
Until there is a recipe, run them the way the container recipe does and add the
tag — `docker run --rm -v "$PWD":/src -w /src -e CGO_ENABLED=1 neru-linux-ci go
test -tags=integration ./...` — on an image with fonts and `fc-list` installed
(`fontconfig fonts-dejavu-core`). CI covers them on `ubuntu-latest`, where
`just test-ci` runs the integration suite natively.

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

What belongs at which log level — and what must never be logged — is in the
root [AGENTS.md](../AGENTS.md) under Conventions.

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
| `internal/cli/`            | Cobra CLI commands, IPC dispatch                   |
| `internal/config/`         | TOML parsing, validation, defaults                 |

Layer responsibilities and the boundaries between them are in
[ARCHITECTURE.md](ARCHITECTURE.md#component-architecture); platform file-slot
naming is in [CROSS_PLATFORM.md](CROSS_PLATFORM.md#file-layout-rules).

**Configuration options** — the full chain (schema → defaults → platform
overrides → validation → examples → docs) is documented in
[internal/config/AGENTS.md](../internal/config/AGENTS.md); the
`add-config-option` skill in `.agents/skills/` walks it step by step.

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
   `internal/adapter/overlay/manager/components.go` — the overlay constructs
   what it draws — and assemble the app-side component in
   `internal/app/component_factory.go`

**CLI commands** — cobra command in `internal/cli/` (registered in an
`init()`), the matching IPC handler in `internal/app/ipcctrl/`, `just genman`,
and [CLI.md](CLI.md); the `add-cli-command` skill walks it step by step.

**Mode flags** — one entry in the descriptor table in
`internal/domain/modecmd`, then `just genflagref`. The entry is what registers
the flag on every command that accepts it and what writes its row in
[CLI.md](CLI.md); an architecture test fails while either is missing.

### Dependency injection

Wiring is manual and explicit — constructors take their dependencies, and
`internal/app/new.go` assembles everything in numbered phases
that unwind in reverse on failure.

`app.New` takes functional options ([options.go](../internal/app/options.go)),
which is how tests substitute doubles for the ports they need — `WithSystemPort`,
`WithEventTap`, `WithIPCServer`, `WithOverlayPort`, `WithHotkeyService`,
`WithWatcher`, plus `WithConfig` / `WithConfigPath` / `WithLogger`. An option
that is not supplied falls back to the real adapter built during initialization.

```go
hintService := services.NewHintService(accAdapter, overlayAdapter, systemPort, hintGen, cfg.Hints, logger, visionPort)
gridService := services.NewGridService(overlayAdapter, systemPort, logger)
actionService := services.NewActionService(accAdapter, overlayAdapter, systemPort, logger)
```

### Mode interface contract

Every navigation mode implements `Mode` (`Activate(modecmd.Activation)` /
`HandleKey(string)` / `Exit()` / `ModeType()` /
`RefreshForMonitorMove(context.Context, image.Rectangle)`), defined in
[handler.go](../internal/app/modes/handler.go). Each mode is its own type with
its own bodies for the four behavioural methods — the shape a new mode has to
follow is stated in
[internal/app/modes/AGENTS.md](../internal/app/modes/AGENTS.md).
A new CLI flag that varies a mode's activation means a new flag descriptor in
[internal/domain/modecmd](../internal/domain/modecmd) and the `Activation`
field it writes, not a new interface method.

All four run with the handler lock already held — the full locking contract
lives in [internal/app/modes/AGENTS.md](../internal/app/modes/AGENTS.md); read
it before touching anything that calls back into the handler.

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
