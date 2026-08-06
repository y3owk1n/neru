# AGENTS.md — Neru Agent Guide

This file is the shared source of truth for any AI agent working on this repo (Claude Code, Codex, Copilot, Cursor, etc.). `CLAUDE.md` is a symlink to this file — edit here, never the symlink. Area-specific contracts live in nested `AGENTS.md` files — read the nearest one (and its parents) before editing there: `internal/app/modes/`, `internal/config/`, `internal/adapter/overlay/`, `internal/adapter/platform/` and its `darwin/` and `linux/` subdirectories. Personal overrides go in gitignored `AGENTS.local.md` / `CLAUDE.local.md`.

Neru is a keyboard-driven, mouse-free navigation tool (hints / grid / recursive grid / scroll modes) written in Go, with native bridges per platform (Objective-C on macOS, X11/Wayland/evdev on Linux, Win32 on Windows). macOS is the primary platform; Linux and Windows are partially supported.

## Product Direction

Neru is a free, open-source mouse replacement: every pointer action reachable from the keyboard, instantly. Judge features and fixes against these properties (origin story: `HOW-I-USE-NERU.md`; current intent: `docs/ROADMAP.md`):

- **Latency is the product.** The event tap sits on every keystroke; a correct feature that makes activation or key handling feel slower is a regression.
- **Good defaults over per-app hacks.** No hardcoded per-application workarounds — the answer to "app X doesn't work" is better defaults plus user-configurable roles/filters.
- **Grid modes stay accessibility-independent.** They exist precisely because accessibility trees are unreliable; never make them depend on element detection.
- **Privacy is absolute.** Neru observes every keystroke and on-screen element. No telemetry, no accounts; nothing logged beyond what Conventions allows.
- **Config is the interface, but every option is a cost.** A new option passes only when no single default is right for everyone. Prefer fixing the default.
- **Reliability beats features.** Startup, reload, and mode transitions failing loudly and recovering cleanly outranks any new capability.

When a feature doesn't clearly fit, park it in a GitHub Discussion instead of implementing — narrowing scope is a maintainer decision.

## Domain Concepts

- **Mode**: navigation context (hints, grid, recursive_grid, scroll, monitor_select; idle when none is active)
- **Port / Adapter**: interface in `internal/ports` / its implementation in `internal/adapter`
- **Bridge**: Objective-C macOS integration layer
- **Semantic role**: platform-neutral role name in `hints.clickable_roles` (`button`, `text_field`), resolved to native vocabulary (AX / AT-SPI / UIA) at config load; native roles use prefixes (`ax:`, `atspi:`, `uia:`). See `internal/domain/element/vocabulary.go`

## Commands

Everything goes through `just` (`just --list` for the full set); `devbox shell` provides the toolchain.

```bash
just build              # dev build -> bin/neru; no `just run` — build then ./bin/neru launch
just build-darwin       # or build-linux / build-windows [ARCH]
just test               # unit + integration, desktop-safe; test-desktop adds the cursor/keyboard-driving tests
just test-foundation    # fast cross-platform-safe slice (config, action, ports)
just lint               # golangci-lint + clang-tidy on .m files (macOS)
just fmt                # golangci-lint fmt, then clang-format on .h/.m/.c
just genman             # man pages via ./cmd/genman
just genflagref         # mode-flag reference in docs/CLI.md via ./cmd/genflagref
```

Pre-commit gate: `just fmt && just lint && just test && just build`. Before pushing: `just ci` — exactly what CI gates on (adds `vet`, `-race`, `vuln`).

Single test: `go test -run TestHandler_HandleKey ./internal/app/modes/`; integration tests need `-tags=integration`.

## Architecture

Hexagonal (ports and adapters). Domain and application logic are pure Go; every OS capability is an interface in `internal/ports` implemented by an adapter in `internal/adapter`.

```
cmd/neru            entry point (main_darwin.go calls runtime.LockOSThread for Cocoa)
internal/cli        Cobra commands; most just send an IPC request to the daemon
internal/app        wiring, lifecycle, modes, services, IPC controller
internal/domain     pure logic: hint, grid, recursivegrid, element, action, state, modecmd
internal/ports      interface contracts + `mocks/`
internal/derrors    the shared error vocabulary
internal/flagref    renders the mode-flag reference in docs/CLI.md from the modecmd descriptor table
internal/adapter    adapters: accessibility, eventtap, hotkeys, overlay, ipc, vision, systray, platform/*
internal/config     TOML schema, defaults, validators, theme (+ loader)
internal/architecture   guardrail tests for package boundaries
```

Hard rules that apply everywhere:

- **The One Rule**: non-darwin-tagged code must never import `internal/adapter/platform/darwin`. Enforced by `depguard` and `internal/architecture/dependency_boundary_test.go`. Details and platform file slots: `internal/adapter/platform/AGENTS.md`.
- **Coordinates**: all shared code uses global top-left origin, Y down, unscaled pixels; Cocoa's flip stays inside the darwin adapter (`internal/domain/geometry` owns conversions).
- **Errors**: use `derrors` (`derrors.New` / `derrors.Wrap`). Unsupported platform behavior returns `derrors.CodeNotSupported` explicitly — never a silent no-op; callers degrade via `IsNotSupported`.
- **`modes.Handler` has a strict locking contract** — read `internal/app/modes/AGENTS.md` before touching modes or anything that calls back into the handler.

Runtime shape: a daemon plus a thin CLI. `neru launch` starts the daemon; other commands dial a Unix socket (`$TMPDIR/neru.sock`, 0600) or Windows named pipe — transport in `internal/adapter/ipc`, handlers in `internal/app/ipcctrl`. New user-facing behavior usually needs a CLI command, an IPC handler, and the service/mode work behind it (the `add-cli-command` skill walks it). Startup is a numbered, individually-unwound phase sequence in `internal/app/new.go`. Input flow: native event tap → `adapter/eventtap` → `app/modes/handler.go` → active `Mode` → `app/services/*` → adapter → native API.

Configuration is hot-reloadable TOML; adding an option touches a five-link chain — read `internal/config/AGENTS.md` or use the `add-config-option` skill.

## Conventions

Formatting and lint mechanics are fully enforced by `just fmt` + `just lint` — run them rather than memorizing rules. What tooling cannot enforce:

- Logging: named zap loggers per subsystem; constructors accept nil (`zap.NewNop()` fallback). `info` is for lifecycle/config/mode-activation only; per-keypress internals go to `debug`. **Never log UI text, element titles/values, hint search terms, keystreams, exec output, or raw config subtrees** — log counts, durations, IDs, booleans instead.
- Tests: unit tests use port mocks from `internal/ports/mocks`; real-OS tests are `*_integration_<os>_test.go` tagged `//go:build integration && <os>`. Table-driven, `TestType_Method_EdgeCase` naming. Every platform stub gets a contract test pinning its `CodeNotSupported` behavior. Full user journeys (hotkey → overlay draw → cursor/click) run as plain unit tests through the simulation harness in `internal/app/simulation_harness_test.go` — extend those journeys when changing user-visible mode behavior.
- Commits are conventional commits; Release Please ships the subject verbatim in the changelog, so write it for users (the `create-pr` skill covers this).
- Each documented fact has exactly one home — ownership table in `docs/CROSS_PLATFORM.md`. Capability *status* goes there, never in `docs/ARCHITECTURE.md` (shape, not status). Update docs in the same change as platform work.

## Agent Resources

- `.agents/skills/` is the canonical home for project skills (`add-config-option`, `add-cli-command`, `add-platform-feature`, `create-pr`, `file-issue`); `.claude/skills` is a directory symlink to it — never add skill bodies there. Each skill may carry an `agents/openai.yaml` overlay for Codex.
- `.claude/agents/` contains review profiles (`platform-boundary-reviewer`, `deadlock-reviewer`) that read the current contract from these guide files.
- `.claude/settings.json` wires a non-blocking format-on-edit hook.
- The layout is pinned by `internal/architecture/agent_contract_test.go`: every `AGENTS.md` (root and nested) keeps a sibling `CLAUDE.md` symlink, and `.claude/skills` stays a symlink to `.agents/skills`.

## Documentation

Start here, then navigate: `docs/ARCHITECTURE.md` (shape) · `docs/DEVELOPMENT.md` · `docs/CLI.md` · `docs/CONFIGURATION.md` · `docs/CROSS_PLATFORM.md` (status). Docs drift in places; when they disagree with code, read the code.

Keep this file lean — it loads into every agent session. Add only contracts an agent cannot infer from the code; area-specific depth goes in the nested guides, workflow depth in a skill.
