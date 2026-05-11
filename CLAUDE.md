# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Neru** (練る — "to refine through practice") is a keyboard-driven navigation tool for macOS, Linux, and Windows. It lets users navigate the screen and interact with UI elements (click, scroll, drag) using only keyboard input.

- **Language:** Go 1.26.1+ with Objective-C for macOS native APIs
- **Build tool:** [Just](https://github.com/casey/just) (`justfile` at root)
- **macOS:** Fully stable; **Linux:** X11 and Wayland (wlroots); **Windows:** Stub/foundations only

## Common Commands

```bash
# Build
just build              # Development build → ./bin/neru
just release            # Release build (optimized, stripped)

# Test
just test               # All unit tests
just test-unit          # Unit tests only
just test-integration   # Integration tests (platform-specific)
just test-foundation    # Cross-platform foundation tests
just test-race          # Tests with race detector

# Lint & Format
just lint               # golangci-lint + clang-format check
just fmt                # Format Go (goimports, gofumpt, golines) + Objective-C
just fmt-check          # Check formatting without modifying
just vet                # go vet

# Pre-commit checklist
just fmt && just lint && just test && just build

# Other
just clean              # Remove build artifacts
just deps               # Download and tidy dependencies
just bundle             # Create macOS .app bundle
just generate-all-protocols  # Fetch Wayland protocol XMLs and generate bindings
```

### Running a single test

```bash
go test ./internal/core/domain/grid/... -run TestGridSubdivide
```

## Architecture

Neru uses **Hexagonal (Ports & Adapters) architecture** with strict layer separation:

```
cmd/neru/             → Entry point
internal/
  app/                → Application layer: orchestration, use cases
    modes/            → Navigation mode implementations (Hints, Grid, Scroll, RecursiveGrid)
    services/         → Business logic services (HintService, GridService, etc.)
    components/       → UI components per mode
  core/
    domain/           → Pure business logic, no external deps
      action/         → Action definitions
      element/        → UI element representation
      grid/           → Grid subdivision algorithms
      hint/           → Hint generation
      state/          → App state, cursor state, modifier state
    ports/            → Interface contracts (AccessibilityPort, OverlayPort, etc.)
    infra/            → Concrete platform implementations
      platform/
        darwin/       → macOS (Accessibility API, EventTap, Hotkeys, Overlay)
        linux/        → X11 / Wayland adapters
        windows/      → Stubs
      ipc/            → Unix socket IPC
      eventtap/       → Global keyboard capture
  ui/overlay/         → Overlay renderer and coordinate system
  config/             → TOML config loading and validation
  cli/                → Cobra-based CLI commands
```

### Navigation Modes

All four modes implement `Mode` interface (`HandleKey`, `HandleActionKey`, `Activate`, `Exit`, `ToggleActionMode`, `ModeType`). The central orchestrator is `internal/app/modes/handler.go`.

1. **Hints** — Overlay labels on clickable elements via macOS Accessibility APIs
2. **Grid** — Universal coordinate grid, subdivide by typing letters
3. **Scroll** — Vim-style scrolling (`j`/`k`, `gg`/`G`, `d`/`u`)
4. **Recursive Grid** — Iteratively subdivides a grid cell (recommended default)

### IPC

Daemon and CLI communicate via Unix sockets. `neru launch` starts the daemon; other commands send messages over IPC with a 5-second timeout.

## Cross-Platform Rules

- **Darwin isolation:** Non-darwin code must never import `internal/core/infra/platform/darwin`. Enforced by `golangci-lint` depguard.
- **Build tags:** All OS-specific files use `//go:build darwin`, `//go:build linux`, or `//go:build windows`.
- **Modifier naming:** Use "Primary" (Cmd on macOS, Ctrl on Linux/Windows) in cross-platform contexts.
- **Unimplemented paths:** Return `errors.CodeNotSupported` for features not yet implemented on a platform.
- **CGO:** Required on macOS (Objective-C bridge) and Linux native backends; disabled by default on Windows.

## Testing Conventions

- Unit tests: `*_test.go` with no build tags
- Integration tests: `*_integration_darwin_test.go` / `*_integration_linux_test.go` tagged `//go:build integration && <os>`
- Core domain logic uses table-driven tests

## Code Standards

- Godoc comments on all exported symbols
- Use the custom error package in `internal/core/errors/` with proper wrapping
- Objective-C files formatted with clang-format (enforced in CI on macOS)
- Full standards in `docs/CODING_STANDARDS.md`

## Key Docs

- `docs/DEVELOPMENT.md` — Build, debug, contribute
- `docs/ARCHITECTURE.md` — System design and cross-platform patterns
- `docs/CROSS_PLATFORM.md` — Linux/Windows contributor guide
- `docs/CONFIGURATION.md` — Full TOML config reference
- `docs/CODING_STANDARDS.md` — Code style requirements
