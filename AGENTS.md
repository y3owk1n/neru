# AGENTS.md - Neru Development Guide

Neru is a keyboard-driven navigation tool for macOS built with Go and Objective-C.

## Domain Concepts

- **Mode**: Navigation context (hints, grid, scroll, action)
- **Bridge**: Objective-C macOS integration layer
- **Adapter**: Port implementation for external systems
- **Port**: Interface definition for system capabilities (e.g., [accessibility.go](file:///Users/kylewong/Dev/neru/internal/core/ports/accessibility.go))

## Architecture & Cross-Platform

Neru follows a **Hexagonal Architecture (Ports and Adapters)**. All OS-specific code is strictly isolated.

### The "One Rule"

**Non-darwin-tagged code must never import `internal/core/infra/platform/darwin`.** This is enforced by `golangci-lint` using `depguard`.

### File Organization for Platforms

- **Ports**: [internal/core/ports/](file:///Users/kylewong/Dev/neru/internal/core/ports/)
- **Infrastructure**: [internal/core/infra/](file:///Users/kylewong/Dev/neru/internal/core/infra/)
- **Platform Factory**: [internal/core/infra/platform/factory.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/factory.go) and build-tagged siblings.
- **Platform Implementations**: [internal/core/infra/platform/darwin/](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/darwin/), `linux/`, `windows/`.

## AI Assistant Exploration Tips

### Finding the "Source of Truth"

- **App Startup**: [app_initialization.go](file:///Users/kylewong/Dev/neru/internal/app/app_initialization.go)
- **Navigation Logic**: [internal/app/modes/](file:///Users/kylewong/Dev/neru/internal/app/modes/)
- **Coordinate Conversion**: [conversion.go](file:///Users/kylewong/Dev/neru/internal/ui/coordinates/conversion.go)
- **Error Definitions**: [errors.go](file:///Users/kylewong/Dev/neru/internal/core/errors/errors.go)
- **Native macOS Logic**: [internal/core/infra/platform/darwin/](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/darwin/)

### Contextual Shortcuts

- To understand **Mode** behavior: Read `internal/app/modes/base.go` and `handler.go`.
- To understand **Accessibility**: Read `internal/core/ports/accessibility.go` (Port) and `internal/core/infra/accessibility/adapter.go` (Adapter).
- To understand **Overlay** rendering: Read `internal/core/ports/overlay.go` and `internal/app/components/overlayutil/factory.go`.

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
