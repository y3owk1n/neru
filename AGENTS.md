# AGENTS.md - Neru Development Guide

Neru is a keyboard-driven navigation tool written in Go, with native bridges per platform (Objective-C on macOS, X11/Wayland/evdev on Linux, Win32 on Windows). macOS is the primary platform; Linux and Windows are partially supported.

## Domain Concepts

- **Mode**: Navigation context (hints, grid, recursive_grid, scroll, monitor_select; idle when none is active)
- **Bridge**: Objective-C macOS integration layer
- **Adapter**: Port implementation for external systems
- **Port**: Interface definition for system capabilities (e.g., [accessibility.go](internal/ports/accessibility.go))
- **Semantic role**: Platform-neutral role name written in `hints.clickable_roles` (`button`, `text_field`). Resolved to each platform's native accessibility vocabulary — AX on macOS, AT-SPI on Linux, UI Automation on Windows — at config load. Native roles are addressed by prefix (`ax:`, `atspi:`, `uia:`). See [vocabulary.go](internal/domain/element/vocabulary.go)

## Architecture & Cross-Platform

Neru follows a **Hexagonal Architecture (Ports and Adapters)**. All OS-specific code is strictly isolated.

### The "One Rule"

**Non-darwin-tagged code must never import `internal/adapter/platform/darwin`.** This is enforced twice: `depguard` in `.golangci.yml` and `internal/architecture/dependency_boundary_test.go` — `just lint` alone is not the only gate.

### File Organization for Platforms

- **Ports**: [internal/ports/](internal/ports/)
- **Adapters**: [internal/adapter/](internal/adapter/)
- **Platform Factory**: [internal/adapter/platform/factory.go](internal/adapter/platform/factory.go) and build-tagged siblings.
- **Platform Implementations**: [internal/adapter/platform/darwin/](internal/adapter/platform/darwin/), `linux/`, `windows/`.

## AI Assistant Exploration Tips

### Finding the "Source of Truth"

- **App Startup**: [app_initialization.go](internal/app/app_initialization.go)
- **Navigation Logic**: [internal/app/modes/](internal/app/modes/)
- **Coordinate Conversion**: [conversion.go](internal/ui/coordinates/conversion.go)
- **Error Definitions**: [errors.go](internal/derrors/errors.go)
- **Role Vocabulary**: [vocabulary.go](internal/domain/element/vocabulary.go) — adapters emit native role names; only config resolution translates
- **Native macOS Logic**: [internal/adapter/platform/darwin/](internal/adapter/platform/darwin/)

### Contextual Shortcuts

- To understand **Mode** behavior: Read `internal/app/modes/base.go` and `handler.go`.
- To understand **Accessibility**: Read `internal/ports/accessibility.go` (Port) and `internal/adapter/accessibility/adapter.go` (Adapter).
- To understand **Overlay** rendering: Read `internal/ports/overlay.go` and `internal/adapter/overlay/render/overlayutil/factory_darwin.go`.

## Documentation

Documentation is progressively disclosed. Start here, then navigate to detailed docs:

- [System Architecture](./docs/ARCHITECTURE.md) - Comprehensive architecture overview
- [Development Guide](./docs/DEVELOPMENT.md) - Build, testing, architecture
- [Coding Standards](./docs/CODING_STANDARDS.md) - Go & Objective-C conventions
- [CLI Usage](./docs/CLI.md) - Command-line interface
- [Configuration](./docs/CONFIGURATION.md) - Configuration reference

## Resources

- [Go](https://golang.org/doc/) | [Just](https://github.com/casey/just) | [Cobra](https://github.com/spf13/cobra)

> **Tip**: Docs may become outdated. When in doubt, read the code directly.
