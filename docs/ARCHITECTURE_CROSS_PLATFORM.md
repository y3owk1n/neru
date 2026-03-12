# Cross-Platform Architecture

Neru is designed to be a cross-platform keyboard-driven navigation tool.
This document is the primary reference for contributors adding or improving
platform support. **Read it before touching any platform-specific code.**

---

## The one rule every contributor must know

> **Non-darwin-tagged code must never import `internal/core/infra/platform/darwin`.**

If you violate this rule, `golangci-lint` will fail with a `depguard` error
pointing you back to this document.

The only code that may import `platform/darwin` is:
- Files inside `internal/core/infra/platform/darwin/` (all carry `//go:build darwin`)
- Files named `*_darwin.go` in any package (they carry `//go:build darwin`)
- Integration test files named `*integration_darwin_test.go`

---

## Architecture layers

```
┌─────────────────────────────────────────────────────────┐
│  cmd/neru  (entry point, build-tag split per OS)        │
├─────────────────────────────────────────────────────────┤
│  internal/app  (orchestration, services, modes)         │
│  internal/config  (configuration, validation)           │
│  ── no OS imports allowed here ──────────────────────── │
├─────────────────────────────────────────────────────────┤
│  internal/core/ports  (interfaces only, no OS code)     │
├─────────────────────────────────────────────────────────┤
│  internal/core/infra/platform/                          │
│    factory.go          ← runtime.GOOS switch            │
│    darwin/             ← //go:build darwin on all files │
│    linux/              ← pure Go, no CGo                │
│    windows/            ← pure Go, no CGo                │
├─────────────────────────────────────────────────────────┤
│  internal/core/infra/  (other infra: hotkeys, eventtap) │
│    package_darwin.go   ← platform dispatch (darwin)     │
│    package_stub.go     ← platform dispatch (!darwin)    │
└─────────────────────────────────────────────────────────┘
```

OS selection happens **only** at:
1. `internal/core/infra/platform/factory.go` — `NewSystemPort()` picks the right adapter.
2. Build-tagged `*_darwin.go` / `*_stub.go` pairs inside infra packages.

---

## How to add a feature to an existing platform

### Step 1 — Find the right adapter

| What you want to implement | Where to implement it |
|---|---|
| Screen bounds, cursor, dark mode, notifications | `internal/core/infra/platform/<os>/system.go` |
| Global hotkeys | `internal/core/infra/hotkeys/manager_<os>.go` |
| Global keyboard event tap | `internal/core/infra/eventtap/eventtap_<os>.go` |
| Application watcher (launch/activate events) | `internal/core/infra/appwatcher/platform_<os>.go` |
| UI overlays | `internal/app/components/*/overlay_<os>.go` |

### Step 2 — Replace the `CodeNotSupported` stub

Most unimplemented methods currently return:
```go
return derrors.New(derrors.CodeNotSupported, "X not yet implemented on linux")
```
Replace that with a real implementation. Remove the `TODO` comment when done.

### Step 3 — Add a test

Unit tests go next to the implementation file. Integration tests (that require
a real display/OS) go in `*_integration_<os>_test.go` files with the matching
build tag (`//go:build integration && linux`).

### Step 4 — Run CI locally

```bash
# On the target OS:
just lint
just vet
just test
just build
```

---

## How to add a new OS-specific operation

If you need a new OS-specific operation that doesn't exist in `ports.SystemPort`:

### Option A — Add it to `SystemPort` (for broadly applicable operations)

1. Add the method to `internal/core/ports/system.go`.
2. Implement it in `internal/core/infra/platform/darwin/system.go`.
3. Add a `CodeNotSupported` stub in `internal/core/infra/platform/linux/system.go`
   and `internal/core/infra/platform/windows/system.go`.
4. Add the method to `internal/core/ports/mocks/system.go`.
5. Inject `SystemPort` into the service/handler that needs it.

### Option B — Use a dispatch pair (for isolated operations)

If the operation is only needed in one infra package (e.g., `appwatcher`):

1. Create `internal/core/infra/<package>/platform_darwin.go`:
   ```go
   //go:build darwin
   package <package>
   import "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
   func platformDoThing() { darwin.DoThing() }
   ```
2. Create `internal/core/infra/<package>/platform_stub.go`:
   ```go
   //go:build !darwin
   package <package>
   func platformDoThing() {}
   ```
3. Call `platformDoThing()` from the package's shared code (no build tag needed there).

**Never import `platform/darwin` from shared (untagged) code.** Use Option A or B.

---

## The `overlayutil/native` sub-package

Overlay rendering on macOS passes C-heap pointers through CGo callbacks.
These helpers live in `internal/app/components/overlayutil/native/`:

- `native_darwin.go` — real CGo-backed implementation
- `native_stub.go` — no-op stubs for non-darwin

`overlayutil` itself has no build tags and no OS imports. It calls `native.*`
functions which resolve to the right implementation per platform.

---

## Platform status

| Capability | macOS | Linux | Windows |
|---|---|---|---|
| Screen bounds / cursor | ✅ | 🔲 TODO | 🔲 TODO |
| Global hotkeys | ✅ | 🔲 TODO | 🔲 TODO |
| Keyboard event tap | ✅ | 🔲 TODO | 🔲 TODO |
| Accessibility (clickable elements) | ✅ | 🔲 TODO (AT-SPI) | 🔲 TODO (UIA) |
| UI overlays | ✅ | 🔲 TODO | 🔲 TODO |
| App watcher | ✅ | 🔲 TODO | 🔲 TODO |
| Dark mode detection | ✅ | 🔲 TODO | 🔲 TODO |
| Notifications / alerts | ✅ | 🔲 TODO | 🔲 TODO |
| Config / log directories | ✅ | ✅ (XDG) | ✅ (AppData) |

🔲 = stub returns `CodeNotSupported`. Replace with real implementation.

---

## Error handling in stubs

Stubs that are not yet implemented return:
```go
derrors.New(derrors.CodeNotSupported, "X not yet implemented on <os>")
```

Callers that need to degrade gracefully should check:
```go
if derrors.IsNotSupported(err) {
    // skip or log, don't crash
}
```

Stubs that are intentional permanent no-ops (e.g., `IsSecureInputEnabled` on
Linux — secure input is a macOS concept) return `false` / no error with a
comment explaining why.

---

## Build system

```bash
just build         # current platform (CGO_ENABLED=1 on macOS, 0 elsewhere)
just release-ci    # all platforms (macOS arm64/amd64, Linux arm64/amd64, Windows arm64/amd64)
just lint          # includes depguard to catch architecture violations
just test          # unit tests
```

Cross-compilation from macOS:
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/neru
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/neru
```

---

## CLI layer cross-platform notes

### `neru services` — service management

`internal/cli/services.go` carries `//go:build darwin` because it uses `launchctl`
and macOS `.plist` files. On Linux/Windows, `services_other.go` provides a stub
that prints a helpful message pointing users to their native service manager.

**To add Linux service management:**
1. Create `internal/cli/services_linux.go` with `//go:build linux`.
2. Implement install/uninstall/start/stop using `systemctl` (systemd) or a
   cross-distro approach.
3. Register the subcommands in `init()` just like the darwin version does.

### `IsRunningFromAppBundle`

`internal/cli/root_darwin.go` detects `.app/Contents/MacOS` paths so the daemon
auto-starts when double-clicked in Finder. `root_other.go` always returns false.

### `cmd/neru/main_darwin.go` vs `main_other.go`

`main_darwin.go` calls `runtime.LockOSThread()` before anything else — required
for Cocoa's main-thread requirement. `main_other.go` omits this. Never add
`LockOSThread` to shared code.

---

## Configuration defaults

Platform-specific defaults live in `internal/config/`:

| File | Purpose |
|---|---|
| `config_darwin.go` | macOS bundle IDs for excluded apps, AX roles |
| `config_linux.go` | Linux app IDs (desktop IDs), AT-SPI roles |
| `config_windows.go` | Windows UIA control type roles |
| `config_defaults.go` | `commonDefaultConfig()` — shared baseline |
| `config_platform.go` | `DefaultConfig()` — calls common + platform |

To add defaults for a new platform, add `applyPlatformDefaults` logic to the
relevant `config_<os>.go` file.

---

## Application identifier terminology

The codebase uses "bundle ID" as a generic term for the platform application
identifier. The mapping per platform is:

| Platform | Term | Example |
|---|---|---|
| macOS | Bundle ID | `com.apple.Safari` |
| Linux | Desktop ID / executable | `firefox.desktop` or `firefox` |
| Windows | AppUserModelID / executable | `Microsoft.Edge` or `msedge.exe` |

The `FocusedAppBundleID` method in `ports.AccessibilityPort` returns whatever
the platform uses. The exclusion list in config (`general.excluded_apps`) should
use the same format for the target platform.
