# Cross-Platform Architecture

Neru is designed to be a cross-platform keyboard-driven navigation tool. This document outlines the architecture used to support multiple operating systems.

## Overview

The codebase is split into three main layers:

1.  **Ports (Interfaces)**: Defined in `internal/core/ports`, these are platform-agnostic interfaces that the rest of the application uses to interact with the system.
2.  **Infrastructure (Adapters)**: Implementation of the ports for specific platforms, located in `internal/core/infra`.
3.  **Application Logic**: The core logic of the application, which remains platform-agnostic by only interacting with the ports.

## Platform Abstraction

### OS Detection and Selection

Platform-specific code is isolated using Go's build tags (`//go:build`). Each platform-specific implementation is typically named with a suffix like `_darwin.go`, `_linux.go`, or `_windows.go`. 

For packages that rely on CGo (like those using macOS native APIs), we provide a clean separation:
- `*_darwin.go`: Contains the actual macOS implementation using CGo and Objective-C.
- `*_linux.go`: Contains the Linux implementation (currently stubs for most components).
- `*_windows.go`: Contains the Windows implementation (currently stubs for most components).

This structure ensures that the project can be built for any supported platform without CGo, which is critical for CI and cross-compilation.

### Stubs and Future Implementations

To support a new platform, you should:
1. Identify the relevant package (e.g., `internal/core/infra/accessibility`).
2. Locate the existing `*_stub.go` or platform-specific file (e.g., `element_windows.go`).
3. Replace the stub implementation with actual platform-specific logic.
4. Ensure all required methods from the package's internal interface or global functions are implemented to match the Darwin version.

### Key Abstraction Layers

-   **Accessibility**: Abstracted via `ports.AccessibilityPort`. On macOS, this uses the `AXUIElement` APIs. On Linux, this will use AT-SPI.
-   **Hotkeys**: Abstracted via `ports.HotkeyPort`.
-   **Event Tap**: Abstracted via `ports.EventTapPort` for global keyboard interception.
-   **Overlay**: Abstracted via `ports.OverlayPort` for drawing on screen.
-   **System**: Abstracted via `ports.SystemPort` for file paths, process management, and screen information.

## Future Linux Support

To implement Linux support, the following steps are required:

1.  **Accessibility**: Implement `ports.AccessibilityPort` using AT-SPI (likely via DBus).
2.  **Hotkeys**: Implement `ports.HotkeyPort` using X11 (`XGrabKey`) or a Wayland-specific protocol.
3.  **Event Tap**: Implement `ports.EventTapPort`. On X11, this can use `XRecord`. On Wayland, this may require a compositor-specific extension or `input-method` protocol.
4.  **Overlay**: Implement `ports.OverlayPort` using a library like `GTK`, `Qt`, or raw `X11`/`Wayland` surfaces.

## Configuration

Platform-specific default settings are handled in the `internal/config` package using `applyPlatformDefaults` functions in platform-specific files (`config_darwin.go`, `config_linux.go`).

## Build System

The `justfile` includes targets for building for different platforms:

-   `just build`: Builds for the current platform.
-   `just release-ci`: Builds binaries for all supported platforms (macOS, Linux, Windows).
