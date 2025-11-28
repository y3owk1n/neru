# Infrastructure Layer

The `internal/core/infra` package contains the concrete implementations of the interfaces defined in the Application layer. This is where the application interacts with the outside world (macOS APIs, UI frameworks, Filesystem).

## Infrastructure

### Accessibility (`internal/core/infra/accessibility`)

Implements `ports.AccessibilityPort`.

- Wraps the low-level accessibility infrastructure.
- Handles CGo calls to macOS Accessibility APIs.
- Converts raw accessibility trees into domain `Element` objects.
- Manages permissions and filtering.

### Overlay (`internal/core/infra/overlay`)

Implements `ports.OverlayPort`.

- Wraps the `internal/ui` package (Electron/Frontend).
- Sends commands to the UI process to draw hints, grids, and highlights.
- Handles coordinate conversion between screen space and UI space.

### Config (`internal/core/infra/config`)

Implements `ports.ConfigPort`.

- Manages loading and saving configuration from disk.
- Provides thread-safe access to configuration values.
- Handles configuration validation.

### EventTap (`internal/core/infra/eventtap`)

- Listens for global keyboard and mouse events.
- Intercepts input for Hint and Grid modes.
- Maps raw events to application commands.

### IPC (`internal/core/infra/ipc`)

- Manages Inter-Process Communication between the Go backend and the UI frontend.
