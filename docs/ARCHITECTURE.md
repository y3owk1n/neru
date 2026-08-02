# Architecture

How Neru is structured internally: layers, boundaries, data flow, and the rules
that keep platform code isolated.

Neru is a keyboard-driven navigation tool written in Go with an Objective-C
bridge on macOS. It runs as a daemon with a thin CLI client.

This document owns **system shape and rationale**. What actually works on each
platform lives in [CROSS_PLATFORM.md](CROSS_PLATFORM.md); how to build and test
lives in [DEVELOPMENT.md](DEVELOPMENT.md).

**Related:** [Cross-Platform Guide](CROSS_PLATFORM.md) ·
[Development Guide](DEVELOPMENT.md) · [Coding Standards](CODING_STANDARDS.md)

---

## Table of Contents

- [System Overview](#system-overview)
- [Runtime Shape](#runtime-shape)
- [Design Principles](#design-principles)
- [The "One Rule"](#the-one-rule)
- [Component Architecture](#component-architecture)
- [Codebase Navigation Guide](#codebase-navigation-guide)
- [Data Flow](#data-flow)
- [Mode Handler Locking](#mode-handler-locking)
- [Coordinate Systems and Units](#coordinate-systems-and-units)
- [Error Handling and Graceful Degradation](#error-handling-and-graceful-degradation)
- [Runtime Capability Reporting](#runtime-capability-reporting)
- [Platform Boundaries in the CLI Layer](#platform-boundaries-in-the-cli-layer)
- [Application Identifier Terminology](#application-identifier-terminology)
- [Technology Stack](#technology-stack)
- [Performance Considerations](#performance-considerations)
- [Security Architecture](#security-architecture)
- [References](#references)

---

## System Overview

Neru runs as a background daemon that listens for global hotkeys and keyboard
events. When activated it offers several navigation modes:

- **Hints** — overlays unique character labels on clickable UI elements
- **Grid** — divides the screen into a coordinate-based grid
- **Recursive grid** — recursive cell navigation with center preview and backtracking
- **Scroll** — Vim-style scrolling at the cursor position

The architecture targets low latency and cross-platform extensibility while
integrating deeply with native APIs. macOS is the reference implementation;
current per-platform support is tracked in
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#platform-status).

---

## Runtime Shape

Neru is a **daemon plus a thin CLI**. `neru launch` starts the daemon;
`neru hints`, `neru action left_click`, `neru config reload` and friends dial a
Unix domain socket (`$TMPDIR/neru.sock`, mode 0600) or a Windows named pipe —
see `internal/adapter/ipc` and `internal/app/ipc_controller.go` /
`ipc_handlers.go`.

New user-facing behavior therefore usually needs three pieces: a CLI command
(`internal/cli/`, registered in an `init()`), an IPC handler, and the
service/mode work behind it.

Startup is a numbered, individually-unwound phase sequence in
[app_initialization.go](../internal/app/app_initialization.go), with the
individual steps in
[app_initialization_steps.go](../internal/app/app_initialization_steps.go):

```
1. infrastructure   4. UI components        7. IPC controller
2. services         4.5 systray             8. event tap + IPC server
3. application state 5. renderer/overlays   9. shutdown channel
                    6. mode handler
```

Dependency injection is manual and explicit. Each phase that allocates
something appends a cleanup closure; on failure the app records `failurePhase`
and runs those closures in reverse (`slices.Backward`), so a half-built daemon
never lingers.

---

## Design Principles

Neru follows a layered **Hexagonal Architecture (Ports and Adapters)**:

1. **Shared business logic** — hint generation, grid calculations, mode
   transitions are pure Go in `internal/domain` and `internal/app/services`.
2. **Platform isolation** — OS-specific code is strictly quarantined.
3. **Ports and adapters** — every system capability (Accessibility, Hotkeys,
   Overlays) is an interface in `internal/ports`, implemented by an adapter
   in `internal/adapter`.
4. **Build tag separation** — OS-specific files carry build tags (`//go:build
   darwin`) so they compile only for their target.
5. **Platform roles over brand names** — shared code says "primary modifier",
   "display server", "accessibility backend", never `Cmd` or a single display
   stack.
6. **Build strategy follows backend choice** — CGO is a per-backend-family
   decision, not a per-OS one. macOS requires it; Linux and Windows mix pure-Go
   and CGO-backed implementations by subsystem.

Where platform code physically goes, which file slot to use, and how the Linux
backend family is organized are contributor concerns owned by
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#contributor-guide). The architectural
source of truth for per-subsystem backend family, primary-modifier
expectations, and build mode is
[profile.go](../internal/adapter/platform/profile.go).

---

## The "One Rule"

> **Non-darwin-tagged code must never import
> `internal/adapter/platform/darwin`.**

Enforced twice: `depguard` in `.golangci.yml`, and
[dependency_boundary_test.go](../internal/architecture/dependency_boundary_test.go).
The only exemptions are `platform/darwin/**`, `*_darwin.go`, and
`*integration_darwin_test.go`.

Cross the boundary through `ports.SystemPort` or a build-tagged dispatch pair
(`platform_darwin.go` / `platform_other.go`).

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

    subgraph "Domain Layer"
        Ports[internal/ports]
        Domain[internal/domain]
    end

    subgraph "Adapters Layer"
        Adapters[internal/adapter]
        Platform[internal/adapter/platform]
    end

    CLI -->|IPC| App
    App --> Services
    Modes --> Services
    Services --> Ports
    Ports --> Domain
    Adapters -.->|Implements| Ports
    Platform -.->|Implements| Ports
    UI --> Adapters
```

### Layer responsibilities

- **Domain** (`internal/domain`) — pure business logic and entities
  ([hint.go](../internal/domain/hint/hint.go),
  [grid.go](../internal/domain/grid/grid.go)). No external dependencies.
- **Ports** (`internal/ports`) — interface contracts defining system
  capabilities ([accessibility.go](../internal/ports/accessibility.go),
  [overlay.go](../internal/ports/overlay.go),
  [font.go](../internal/ports/font.go)).
- **Application** (`internal/app`) — orchestrates domain entities and services;
  owns lifecycle and navigation modes.
- **Adapters** (`internal/adapter`) — concrete port implementations on
  platform APIs.
- **UI** (`internal/ui`) — coordinate transformation and the renderer facade
  over the overlay adapter. The native overlay backends live in
  `internal/adapter/overlay`, not here.
- **CLI** (`internal/cli`) — user commands, config loading, IPC to the daemon.

A directory-by-directory map for placing new code is in
[DEVELOPMENT.md](DEVELOPMENT.md#where-things-go).

---

## Codebase Navigation Guide

The fastest way to understand Neru is to follow one event from the OS to the
user-visible action.

**1. Entry points**

- [main_darwin.go](../cmd/neru/main_darwin.go) — bootstraps the app, locking the
  main thread for Cocoa
- [root.go](../internal/cli/root.go) — the Cobra root command

**2. Application wiring**

- [app_initialization.go](../internal/app/app_initialization.go) — startup phases
- [app_initialization_steps.go](../internal/app/app_initialization_steps.go) —
  the individual infrastructure, service, and UI steps

**3. The platform factory**

[factory.go](../internal/adapter/platform/factory.go) and its build-tagged
siblings are the only place that picks a `ports.SystemPort` implementation. On
Linux there is a second, *runtime* axis on top of build tags:
[backend_linux.go](../internal/adapter/platform/backend_linux.go) detects the
live compositor (wlroots / KDE / GNOME / other) and the factory routes to it.

**4. Input processing**

1. **OS** — [eventtap_darwin.m](../internal/adapter/platform/darwin/eventtap_darwin.m)
   captures low-level keyboard events (Linux/Windows have equivalents)
2. **Adapters** — [adapter.go](../internal/adapter/eventtap/adapter.go)
   receives and dispatches them
3. **Application** — [handler.go](../internal/app/modes/handler.go) routes the
   key to the active [Mode](../internal/app/modes/base.go)
4. **Service** — the mode calls into
   [hint_service.go](../internal/app/services/hint_service.go) and friends
5. **Keyboard layout changes** — on macOS the mode-level CGEventTap rebuilds its
   key-name lookup tables at runtime (`NeruSetKeymapLayoutChangeCallback` in
   [keymap_darwin.m](../internal/adapter/platform/darwin/keymap_darwin.m)) so
   navigation keys survive layout switches. Per-hotkey CGEventTaps re-register
   too (`NeruSetKeymapLayoutChangeCallback2`), because `NeruKeyNameToCode` maps
   key names to layout-aware keycodes.

---

## Data Flow

### Input event propagation

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

### Overlay rendering

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

On macOS each component owns its own NSPanel and calls the Objective-C bridge
directly. On Linux and Windows the overlay **manager** does all drawing into one
shared surface, and the per-component files are style-only stubs — see
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#overlay-rendering).

### The CGo bridge (macOS)

Native macOS classes are wrapped in CGo so Go can call Cocoa while keeping type
safety. Location: `internal/adapter/platform/darwin/`; key files `bridge.go`,
`overlay_darwin.m`, `accessibility_element_darwin.m`.

---

## Mode Handler Locking

`modes.Handler` has a single `mu sync.Mutex` serializing the event-tap thread
against timer goroutines. The public entry points (`HandleKeyPress`,
`ActivateMode`, `ExitMode`, …) take the lock and then call into modes.

**Consequently `Mode.Activate` / `HandleKey` / `Exit` all run with `h.mu`
already held.** Inside them, use only the `*Locked` helpers —
`setModeLocked`, `exitModeLocked`, `refreshGridVirtualPointerLocked`, … — never
the public `SetMode*` / `ExitMode` methods, which would self-deadlock.

There is one documented lock order: **`moveMonitorMu` → `h.mu`**, never the
reverse.

The interface itself and the `baseMode` / `GenericMode` building blocks are
documented in
[DEVELOPMENT.md](DEVELOPMENT.md#mode-interface-contract).

---

## Coordinate Systems and Units

All shared code uses a **global top-left (0,0)** coordinate system.

- **Origin** — (0,0) is the top-left corner of the primary display
- **Y-axis** — increases downwards
- **Units** — screen pixels, unscaled

macOS Cocoa uses a bottom-left origin with Y increasing upwards. The inversion
happens inside the darwin adapter
([accessibility_screen_darwin.m](../internal/adapter/platform/darwin/accessibility_screen_darwin.m))
— flipped coordinates must never leak into shared Go. Conversions live in
`internal/ui/coordinates`.

---

## Error Handling and Graceful Degradation

Neru uses the custom [derrors](../internal/derrors/errors.go) package:
`derrors.New(code, msg)` and `derrors.Wrap(err, code, msg)`.

### The `CodeNotSupported` policy

Unimplemented platform behavior must return `CodeNotSupported` explicitly rather
than silently no-oping:

```go
return derrors.New(derrors.CodeNotSupported, "feature X not yet implemented on linux")
```

Callers in the service layer degrade gracefully via `derrors.IsNotSupported(err)`
— typically logging a warning instead of surfacing an error. Prefer
`CodeNotSupported` over a silent no-op unless the operation is explicitly
documented as best-effort.

---

## Runtime Capability Reporting

Neru reports a runtime capability matrix through the platform adapters,
deliberately stricter than "it compiles":

- supported features report `supported`
- stubbed or incomplete features report `stub`

`neru doctor` is the user-facing entry point, so the matrix
([capabilities.go](../internal/ports/capabilities.go),
[capability_presets.go](../internal/ports/capability_presets.go)) must stay
in sync with reality — a stub reporting `supported` is a bug.

---

## Platform Boundaries in the CLI Layer

**`neru services`** — `internal/cli/services.go` carries `//go:build darwin`
because it drives `launchctl` and macOS `.plist` files. On other platforms the
command is simply never registered. Adding Linux service management means a new
`services_linux.go` with `//go:build linux` implementing install/uninstall/
start/stop over `systemctl`, registered in its own `init()`.

**`IsRunningFromAppBundle`** — [root.go](../internal/cli/root.go) delegates to a
build-tagged `isRunningFromAppBundle()`. On macOS it detects
`.app/Contents/MacOS` paths so the daemon auto-starts when double-clicked in
Finder; elsewhere it returns false.

**Main-thread locking** — on macOS `cmd/neru/main.go` calls
`runtime.LockOSThread()` before anything else, required by Cocoa. Non-macOS
builds omit it. Never add `LockOSThread` to shared code.

---

## Application Identifier Terminology

The codebase says "bundle ID" generically for the platform application
identifier:

| Platform | Term                        | Example                          |
| -------- | --------------------------- | -------------------------------- |
| macOS    | Bundle ID                   | `com.apple.Safari`               |
| Linux    | Desktop ID / executable     | `firefox.desktop` or `firefox`   |
| Windows  | AppUserModelID / executable | `Microsoft.Edge` or `msedge.exe` |

`ports.AccessibilityPort.FocusedAppBundleID` returns whatever the platform uses,
and `general.excluded_apps` in the config should use the same format for the
target platform.

---

## Technology Stack

- **Core language** — [Go](https://golang.org/) 1.26+
- **Native integration** — [CGo](https://pkg.go.dev/cmd/cgo) + Objective-C (macOS)
- **CLI framework** — [Cobra](https://github.com/spf13/cobra)
- **Configuration** — [TOML](https://toml.io/)
- **IPC** — Unix domain sockets (Windows named pipes)
- **Build system** — [Just](https://github.com/casey/just)
- **CI/CD** — GitHub Actions + [Release Please](https://github.com/googleapis/release-please)

GitHub Actions runs lint, unit, and integration tests on every PR. Windows
binaries cross-compile with `CGO_ENABLED=0`; Linux builds need `CGO_ENABLED=1`
(X11/Wayland native backends) and must run on a Linux host, as macOS does for
its own.

---

## Performance Considerations

1. **Event tap latency** — the event tap callback stays extremely lean to avoid
   system-wide keyboard lag; heavy processing is deferred to goroutines.
2. **Bounded accessibility walks** — querying accessibility APIs is expensive, so
   traversal is bounded rather than exhaustive: `maxDepth` on the macOS walk
   ([client.go](../internal/adapter/accessibility/client.go)), and
   `atspiMaxDepth` / `atspiMaxNodes` on the Linux AT-SPI walk
   ([atspi_linux.go](../internal/adapter/accessibility/atspi_linux.go)).
3. **Caching** — a TTL/LRU cache for computed grid layouts
   ([grid/cache.go](../internal/domain/grid/cache.go)) and a cache of C
   string pointers for overlay styles
   ([style_cache.go](../internal/adapter/overlay/render/overlayutil/style_cache.go))
   keep repeated activations off the hot path.
4. **Native rendering** — GPU-accelerated CoreAnimation on macOS, Cairo on
   Linux, GDI on Windows.

---

## Security Architecture

1. **Secure input detection** — Neru detects when Secure Input is enabled (e.g.
   a focused password field) and suspends the event tap, preventing unintended
   key logging.
2. **Permissions** — Accessibility permission is required on macOS; Neru requests
   only the minimum needed for UI interaction.
3. **IPC security** — the Unix domain socket is created with restricted file
   permissions (0600), so only the current user can talk to the daemon.

---

## References

- [CROSS_PLATFORM.md](CROSS_PLATFORM.md) — per-platform support and contributor guide
- [DEVELOPMENT.md](DEVELOPMENT.md) — build, test, debug, add code
- [CODING_STANDARDS.md](CODING_STANDARDS.md) — formatting, logging, documentation
- [CONFIGURATION.md](CONFIGURATION.md) — configuration reference
- [macOS Accessibility API](https://developer.apple.com/documentation/applicationservices/ax_ui_element_ref)
