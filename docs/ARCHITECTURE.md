# Neru System Architecture

Neru is a keyboard-driven navigation tool for macOS (with Linux and Windows support in progress) built with Go and Objective-C. It enhances productivity by allowing users to quickly navigate and interact with UI elements using keyboard shortcuts.

---

## Table of Contents

- [System Overview](#system-overview)
- [Cross-Platform Design Principles](#cross-platform-design-principles)
- [Platform Status](#platform-status)
- [Platform Contribution Model](#platform-contribution-model)
- [CLI Layer Cross-Platform Notes](#cli-layer-cross-platform-notes)
- [Application Identifier Terminology](#application-identifier-terminology)
- [Codebase Navigation Guide](#codebase-navigation-guide)
- [Coordinate Systems and Units](#coordinate-systems-and-units)
- [Runtime Capability Reporting](#runtime-capability-reporting)
- [Error Handling and Graceful Degradation](#error-handling-and-graceful-degradation)
- [Technology Stack](#technology-stack)
- [Component Architecture](#component-architecture)
- [Platform-Specific Implementations](#platform-specific-implementations)
- [Data Flow Diagrams](#data-flow-diagrams)
- [API Integration Patterns](#api-integration-patterns)
- [Build and Deployment Processes](#build-and-deployment-processes)
- [Performance Considerations](#performance-considerations)
- [Security Architecture](#security-architecture)
- [References](#references)

---

## System Overview

Neru operates as a background daemon that listens for global hotkeys and keyboard events. When activated, it provides several navigation modes:

- **Hints Mode**: Overlays unique character labels on clickable UI elements.
- **Grid Mode**: Divides the screen into a coordinate-based grid system.
- **Scroll Mode**: Provides Vim-style scrolling at the current cursor position.
- **Recursive Grid Mode**: Recursive cell navigation with center preview and backtracking.

The architecture is designed for high performance, low latency, and cross-platform extensibility while maintaining a deep integration with macOS native APIs.

This document focuses on system shape and architectural rationale.
For contributor workflow, file-slot selection, Linux backend guidance, and
practical porting steps, use [CROSS_PLATFORM.md](./CROSS_PLATFORM.md).

---

## Cross-Platform Design Principles

Neru follows a layered architecture inspired by the **Hexagonal Architecture (Ports and Adapters)** pattern.

### Core Principles

1. **Shared Business Logic**: All core logic (hint generation, grid calculations, mode transitions) is written in pure Go and resides in `internal/core/domain` and `internal/app/services`.
2. **Platform Isolation**: OS-specific code is strictly isolated. Non-darwin code must never import macOS-specific packages.
3. **Ports and Adapters**: System capabilities (Accessibility, Hotkeys, Overlays) are defined as interfaces (Ports) in `internal/core/ports`. Concrete implementations (Adapters) reside in `internal/core/infra`.
4. **Build Tag Separation**: OS-specific files use Go build tags (e.g., `//go:build darwin`) to ensure they are only compiled for the target platform.
5. **Platform Roles Over Brand Names**: Shared code should talk about roles like "primary modifier", "display server", and "accessibility backend" instead of assuming macOS terms such as `Cmd` or a single display stack.
6. **Build Strategy Follows Backend Choice**: CGO requirements should be modeled per backend family, not assumed per OS. macOS currently requires CGO; Linux and Windows may mix pure-Go and CGO-backed implementations depending on subsystem/backend.

### The "One Rule"

> **Non-darwin-tagged code must never import `internal/core/infra/platform/darwin`.**

Violation of this rule is caught by `golangci-lint` using `depguard`.

---

## Platform Status

| Capability                         | macOS | Linux            | Windows       |
| ---------------------------------- | ----- | ---------------- | ------------- |
| Screen bounds / cursor             | ✅    | 🔲 TODO          | 🔲 TODO       |
| Global hotkeys                     | ✅    | 🔲 TODO          | 🔲 TODO       |
| Keyboard event tap                 | ✅    | 🔲 TODO          | 🔲 TODO       |
| Accessibility (clickable elements) | ✅    | 🔲 TODO (AT-SPI) | 🔲 TODO (UIA) |
| UI overlays                        | ✅    | 🔲 TODO          | 🔲 TODO       |
| App watcher                        | ✅    | 🔲 TODO          | 🔲 TODO       |
| Dark mode detection                | ✅    | 🔲 TODO          | 🔲 TODO       |
| Notifications / alerts             | ✅    | 🔲 TODO          | 🔲 TODO       |
| Config / log directories           | ✅    | ⚠️ Partial       | ✅ (AppData)  |

🔲 = stub returns `CodeNotSupported`. Replace with real implementation.

---

## Platform Contribution Model

This section explains how platform work fits into the architecture.
It is intentionally high-level. For the contributor playbook, see
[CROSS_PLATFORM.md](./CROSS_PLATFORM.md).

### Source Of Truth

The main architecture-level source of truth for platform expectations is:

- [profile.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/profile.go)

It describes:

- primary modifier expectations
- display server family
- backend family per subsystem
- expected build mode per backend family

### Layer Placement

Platform work should stay in these layers:

- `internal/core/infra/platform/`: broad system capabilities
- `internal/core/infra/hotkeys/`: global hotkey registration
- `internal/core/infra/eventtap/`: keyboard event capture
- `internal/core/infra/accessibility/`: platform accessibility integration
- `internal/ui/overlay/`: overlay manager orchestration
- `internal/app/components/*/overlay_*.go`: mode-specific overlay rendering

Shared logic should stay in:

- `internal/core/domain/`
- `internal/app/services/`
- `internal/app/modes/`

### Linux Backend Shape

Linux is modeled as a backend family rather than a single implementation.

At the architecture level, the expected split is:

- `*_linux_common.go`: shared wrapper/fallback layer
- `*_linux_x11.go`: X11 implementation slot
- `*_linux_wayland.go`: Wayland/compositor implementation slot

This keeps backend choice out of shared app code and makes implementation slots
obvious before real Linux work lands.

### Fallback And Unsupported Paths

Dispatch-style packages may use:

- `platform_darwin.go`
- `platform_other.go`

Unimplemented behavior should remain explicit through `CodeNotSupported` and
capability reporting rather than silent failure.

### Build Strategy

Build strategy follows backend choice, not just OS choice.

Current high-level model:

- macOS: CGO-backed native bridge
- Linux: backend-dependent
- Windows: prefer pure Go first

The practical decisions and file-by-file guidance live in
[CROSS_PLATFORM.md](./CROSS_PLATFORM.md).

---

## CLI Layer Cross-Platform Notes

### `neru services` — service management

`internal/cli/services.go` carries `//go:build darwin` because it uses `launchctl` and macOS `.plist` files. On non-darwin platforms, the `services` command is simply not registered.
**To add Linux service management:**

1. Create `internal/cli/services_linux.go` with `//go:build linux`.
2. Implement install/uninstall/start/stop using `systemctl` (systemd) or a cross-distro approach.
3. Register the subcommands in `init()` just like the darwin version does.

### `IsRunningFromAppBundle`

`internal/cli/root.go` defines `IsRunningFromAppBundle()` which delegates to a build-tagged `isRunningFromAppBundle()` helper. On macOS it detects `.app/Contents/MacOS` paths so the daemon auto-starts when double-clicked in Finder. On other platforms it always returns false.

### `cmd/neru/main.go` — main thread locking

On macOS, the entry point calls `runtime.LockOSThread()` before anything else — required for Cocoa's main-thread requirement. Non-macOS builds omit this. Never add `LockOSThread` to shared code.

## Application Identifier Terminology

The codebase uses "bundle ID" as a generic term for the platform application identifier. The mapping per platform is:

| Platform | Term                        | Example                          |
| -------- | --------------------------- | -------------------------------- |
| macOS    | Bundle ID                   | `com.apple.Safari`               |
| Linux    | Desktop ID / executable     | `firefox.desktop` or `firefox`   |
| Windows  | AppUserModelID / executable | `Microsoft.Edge` or `msedge.exe` |

The `FocusedAppBundleID` method in `ports.AccessibilityPort` returns whatever the platform uses. The exclusion list in config (`general.excluded_apps`) should use the same format for the target platform.

---

## Codebase Navigation Guide

To understand how Neru works, follow the path of an event from the OS to the user action.

### 1. Entry Points

- [main_darwin.go](file:///Users/kylewong/Dev/neru/cmd/neru/main_darwin.go): Bootstraps the application, locking the main thread for Cocoa.
- [root.go](file:///Users/kylewong/Dev/neru/internal/cli/root.go): The Cobra root command for the CLI.

### 2. Application Wiring

- [app_initialization.go](file:///Users/kylewong/Dev/neru/internal/app/app_initialization.go): Orchestrates the startup phases.
- [app_initialization_steps.go](file:///Users/kylewong/Dev/neru/internal/app/app_initialization_steps.go): Detailed steps for initializing infrastructure, services, and UI.

### 3. The Platform Factory

The [factory.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/factory.go) and its build-tagged siblings (e.g., [factory_darwin.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/factory_darwin.go)) are the gatekeepers for OS-specific code. They return the correct `ports.SystemPort` implementation without polluting shared code with OS-specific imports.

## Runtime Capability Reporting

Neru now reports a runtime capability matrix through the platform adapters.
This is intentionally stricter than "it compiles":

- Supported features report `supported`
- Stubbed or incomplete features report `stub`

The main user-facing entry point is `neru doctor`, which surfaces platform
gaps instead of letting unsupported behavior fail silently.

### 4. Input Processing Flow

1. **OS Level**: [eventtap_darwin.m](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/darwin/eventtap_darwin.m) captures low-level keyboard events.
2. **Infrastructure Level**: [adapter.go](file:///Users/kylewong/Dev/neru/internal/core/infra/eventtap/adapter.go) receives events and dispatches them to the app.
3. **Application Level**: [handler.go](file:///Users/kylewong/Dev/neru/internal/app/modes/handler.go) receives the key and routes it to the active [Mode](file:///Users/kylewong/Dev/neru/internal/app/modes/base.go).
4. **Service Level**: The mode calls into services like [hint_service.go](file:///Users/kylewong/Dev/neru/internal/app/services/hint_service.go) to perform business logic.

---

## Coordinate Systems and Units

Neru uses a **global top-left (0,0) coordinate system** for all shared logic.

- **Origin**: (0,0) is the top-left corner of the primary display.
- **Y-Axis**: Increases downwards.
- **Units**: Screen pixels (unscaled).

### macOS Coordinate Inversion

macOS Cocoa APIs use a bottom-left (0,0) coordinate system where Y increases upwards. The [darwin platform adapter](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/darwin/accessibility_screen_darwin.m) is responsible for inverting the Y coordinate before passing it to shared Go code.

---

## Error Handling and Graceful Degradation

Neru uses a custom error package [derrors](file:///Users/kylewong/Dev/neru/internal/core/errors/errors.go) for structured error handling.

### The `CodeNotSupported` Policy

When a platform-specific feature is not yet implemented, the adapter must return an error with the `CodeNotSupported` code.

```go
return derrors.New(derrors.CodeNotSupported, "feature X not yet implemented on linux")
```

### Graceful Degradation

Callers in the service layer should use the `IsNotSupported(err)` helper to handle missing features gracefully (e.g., by logging a warning instead of returning an error to the user).

For features that are intentionally unavailable on a platform, prefer returning
`CodeNotSupported` over silent no-ops unless the operation is explicitly
documented as best-effort.

---

## Technology Stack

- **Core Language**: [Go](https://golang.org/) (1.26+)
- **Native Integration**: [CGo](https://pkg.go.dev/cmd/cgo) + Objective-C (macOS Bridge)
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **Configuration**: [TOML](https://toml.io/)
- **IPC**: Unix Domain Sockets
- **Build System**: [Just](https://github.com/casey/just)
- **CI/CD**: GitHub Actions + [Release Please](https://github.com/googleapis/release-please)

---

## Component Architecture

```mermaid
graph TD
    subgraph "Presentation Layer"
        CLI[internal/cli]
        UI[internal/ui]
    end

    subgraph "Application Layer"
        App[internal/app/app.go]
        Modes[internal/app/modes]
        Services[internal/app/services]
    end

    subgraph "Core Layer"
        Ports[internal/core/ports]
        Domain[internal/core/domain]
    end

    subgraph "Infrastructure Layer"
        Infra[internal/core/infra]
        Platform[internal/core/infra/platform]
    end

    CLI -->|IPC| App
    App --> Services
    Modes --> Services
    Services --> Ports
    Ports --> Domain
    Infra -.->|Implements| Ports
    Platform -.->|Implements| Ports
    UI --> Platform
```

### Layer Responsibilities

- **Domain (`internal/core/domain`)**: Pure business logic and entities (e.g., [hint.go](file:///Users/kylewong/Dev/neru/internal/core/domain/hint/hint.go), [grid.go](file:///Users/kylewong/Dev/neru/internal/core/domain/grid/grid.go)). No external dependencies.
- **Ports (`internal/core/ports`)**: Interface contracts defining system capabilities (e.g., [accessibility.go](file:///Users/kylewong/Dev/neru/internal/core/ports/accessibility.go), [overlay.go](file:///Users/kylewong/Dev/neru/internal/core/ports/overlay.go)).
- **Application (`internal/app`)**: Orchestrates domain entities and services. Manages application lifecycle and navigation modes (e.g., [hints.go](file:///Users/kylewong/Dev/neru/internal/app/modes/hints.go)).
- **Infrastructure (`internal/core/infra`)**: Concrete implementations of ports using platform-specific APIs (e.g., [accessibility/adapter.go](file:///Users/kylewong/Dev/neru/internal/core/infra/accessibility/adapter.go)).
- **UI (`internal/ui`)**: Handles coordinate transformations and abstract rendering logic.
- **CLI (`internal/cli`)**: Handles user commands, configuration loading, and IPC communication with the daemon.

---

## Platform-Specific Implementations

### macOS (Primary)

- **Accessibility**: Uses `AXUIElement` to query UI hierarchies and perform actions (click, focus).
- **Event Tap**: Uses `CGEventTap` for global keyboard interception.
- **Hotkeys**: Uses native system APIs for global hotkey registration.
- **Overlays**: Native Cocoa windows managed via Objective-C bridge.

### Linux/Windows (In Progress)

- **Linux**: Planned integration with AT-SPI for accessibility and X11/Wayland for events.
- **Windows**: Planned integration with UI Automation (UIA) and Windows Hooks.
- Currently, most non-macOS implementations are stubs returning `derrors.CodeNotSupported`.

---

## Data Flow Diagrams

### Input Event Propagation

```mermaid
sequenceDiagram
    participant OS as Operating System
    participant ET as Event Tap (Infra)
    participant H as Handler (App)
    participant M as Active Mode (App)
    participant S as Service (App)
    participant A as Adapter (Infra)

    OS->>ET: Key Down Event
    ET->>H: Dispatch Key
    H->>M: HandleKey(key)
    M->>S: Process Logic
    S->>A: Perform Action (e.g., Click)
    A->>OS: Native API Call
```

### Overlay Rendering Flow

```mermaid
sequenceDiagram
    participant M as Mode (App)
    participant S as Service (App)
    participant OA as Overlay Adapter (Infra)
    participant B as Bridge (CGo)
    participant C as Cocoa (macOS)

    M->>S: Request Display
    S->>OA: ShowOverlay(elements)
    OA->>B: DrawLabels(rects)
    B->>C: Render Native Windows
```

---

## API Integration Patterns

### CGo Bridge (macOS)

Neru uses a sophisticated bridge between Go and Objective-C. Native macOS classes are wrapped in CGo, allowing Go to call into Cocoa APIs while maintaining type safety.

- **Location**: `internal/core/infra/platform/darwin/`
- **Key Files**: `bridge.go`, `overlay_darwin.m`, `accessibility_element_darwin.m`.

### IPC Controller

The CLI communicates with the background daemon using Unix Domain Sockets. The [ipc_controller.go](file:///Users/kylewong/Dev/neru/internal/app/ipc_controller.go) manages this communication, routing commands like `neru hints` or `neru stop` to the running application instance.

---

## Build and Deployment Processes

### Build System

Neru uses `just` for build automation.

- `just build`: Compiles the binary for the current platform.
- `just release`: Optimized build with stripped symbols.
- `just bundle`: Creates a macOS `.app` bundle.

### CI/CD

- **GitHub Actions**: Runs linting, unit tests, and integration tests on every PR.
- **Release Please**: Automatically manages versioning and generates GitHub releases upon merging to `main`.
- **Cross-Compilation**: Go's native cross-compilation is used for Linux and Windows binaries (with `CGO_ENABLED=0`).

---

## Performance Considerations

1. **Event Tap Latency**: The event tap callback is kept extremely lean to prevent system-wide keyboard lag. Heavy processing is deferred to Go routines.
2. **Accessibility Caching**: Querying the macOS Accessibility API is expensive. Neru implements intelligent caching in the [accessibility/cache.go](file:///Users/kylewong/Dev/neru/internal/core/infra/accessibility/cache.go) to minimize IPC overhead.
3. **Native Rendering**: Overlays are rendered using native Cocoa APIs for GPU-accelerated, flicker-free UI.

---

## Security Architecture

1. **Secure Input Detection**: Neru detects when "Secure Input" is enabled (e.g., focusing a password field) and automatically suspends the event tap to prevent unintended key logging.
2. **Permissions**: Neru requires Accessibility permissions on macOS. It only requests the minimum set of permissions needed for UI interaction.
3. **IPC Security**: Unix domain sockets are created with restricted file permissions, ensuring only the current user can communicate with the daemon.

---

## References

- [Cross-Platform Contributor Guide](CROSS_PLATFORM.md)
- [Development Guide](DEVELOPMENT.md)
- [Coding Standards](CODING_STANDARDS.md)
- [Configuration Reference](CONFIGURATION.md)
- [macOS Accessibility API](https://developer.apple.com/documentation/applicationservices/ax_ui_element_ref)
