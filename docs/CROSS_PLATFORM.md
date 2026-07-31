# Cross-Platform Contributor Guide

This guide is the practical entry point for contributors working on Linux,
Windows, or platform-neutral infrastructure in Neru.

It explains:

- where platform code lives
- how to choose the right file before writing code
- how Linux backend splits work
- when to use CGO
- how to add a new platform capability safely
- what tests and docs to update when you are done
- **what actually works where** — see [Feature Parity Reference](#feature-parity-reference)

For the higher-level design, see [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Table of Contents

- [Goals](#goals)
- [First Stops](#first-stops)
- [First 15 Minutes](#first-15-minutes)
- [File Layout Rules](#file-layout-rules)
- [Build And Test Commands](#build-and-test-commands)
- [Where To Implement What](#where-to-implement-what)
- [Linux Backend Model](#linux-backend-model)
- [Windows Model](#windows-model)
- [CGO Guidance](#cgo-guidance)
- [Hotkeys And Modifiers](#hotkeys-and-modifiers)
- [Adding A New Capability](#adding-a-new-capability)
- [Error Handling Rules](#error-handling-rules)
- [Capability Reporting](#capability-reporting)
- [Testing Checklist](#testing-checklist)
- [Documentation Checklist](#documentation-checklist)
- [Suggested First Contributions](#suggested-first-contributions)
- [What "Done" Looks Like](#what-done-looks-like)
- [Feature Parity Reference](#feature-parity-reference)
    - [Port Capability Matrix](#port-capability-matrix)
    - [Overlay Rendering](#overlay-rendering)
    - [Accessibility Systems](#accessibility-systems)
    - [Input Handling](#input-handling)
    - [Mode Feature Coverage](#mode-feature-coverage)
    - [Linux Backend Breakdown](#linux-backend-breakdown)
    - [Windows Feature Detail](#windows-feature-detail)
    - [Platform-Specific Features](#platform-specific-features)

---

## Goals

Neru aims to make cross-platform work predictable and low-friction.

The guiding principles are:

- shared business logic stays in pure Go
- platform-specific code is easy to locate
- Linux backend differences are explicit
- contributors implement in existing slots instead of inventing new file layout
- unsupported features fail clearly with `CodeNotSupported`

---

## First Stops

Before changing code, read these files first:

- [internal/core/infra/platform/profile.go](../internal/core/infra/platform/profile.go)
- [internal/core/ports/system.go](../internal/core/ports/system.go)
- [internal/core/ports/capabilities.go](../internal/core/ports/capabilities.go)
- [internal/core/ports/capability_presets.go](../internal/core/ports/capability_presets.go)
- [internal/core/ports/font.go](../internal/core/ports/font.go) — FontResolver port (fontconfig on Linux, NSFont on macOS)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [CONVENTIONS.md](./go/CONVENTIONS.md)

If you are contributing Linux support, also inspect the reserved backend files in
the package you plan to touch, such as:

- `*_linux_common.go`
- `*_linux_x11.go`
- `*_linux_wayland.go`

---

## First 15 Minutes

If you are new to the codebase, this is the recommended startup path.

### Any platform

1. Read [profile.go](../internal/core/infra/platform/profile.go)
2. Read the relevant port in `internal/core/ports/`
3. Find the implementation slot you expect to touch
4. Run:

```bash
just build
just test-foundation
```

### Linux contributors

1. Identify whether your work belongs in `common`, `x11`, or `wayland`
2. Open the target file in that slot before writing code
3. Build a Linux foundations binary from any host if needed:

```bash
just build-linux
```

4. If you are on Linux, also run:

```bash
just test
```

### Windows contributors

1. Start in the `*_windows.go` slot for the package you are changing
2. Build a Windows foundations binary from any host if needed:

```bash
just build-windows
```

3. If you are on Windows, run:

```bash
just test
```

### macOS contributors

If your work touches the real native product path, run:

```bash
just build-darwin
just test
```

---

## File Layout Rules

Platform-specific files should make the intended implementation slot obvious.

Use these suffixes:

- `*_darwin.go`: macOS implementation
- `*_windows.go`: Windows implementation
- `*_linux_common.go`: shared Linux wrapper or current fallback behavior
- `*_linux_x11.go`: Linux X11 implementation slot
- `*_linux_wayland.go`: Linux Wayland implementation slot
- `*_linux_wayland_<compositor>.go`: Wayland compositor sub-slot when one
  compositor family needs a distinct implementation (e.g.
  `*_linux_wayland_wlroots.go`, `*_linux_wayland_kde.go`). Still the same
  package + build tags; selection is at runtime (see Linux Backend Model).
- `*_other.go`: non-target fallback for dispatch-style packages

App-level platform dispatch also follows this pattern. For example,
per-hotkey CGEventTaps are re-registered on layout change (via
`NeruSetKeymapLayoutChangeCallback2`) because `NeruKeyNameToCode` maps
key names to layout-aware keycodes.

Do not create new ad hoc platform filenames if an existing slot already exists.

Do not create fake empty `darwin` / `linux` / `windows` files just for symmetry.
Only add a new file when there is a real new implementation slot.

---

## Build And Test Commands

These are the main contributor commands:

| Goal                                            | Command                |
| ----------------------------------------------- | ---------------------- |
| build for current host                          | `just build`           |
| build macOS binary on macOS                     | `just build-darwin`    |
| build Linux foundations binary                  | `just build-linux`     |
| build Windows foundations binary                | `just build-windows`   |
| run focused cross-platform-safe tests           | `just test-foundation` |
| run full unit + integration suite on current OS | `just test`            |
| run lint checks                                 | `just lint`            |

Notes:

- `just build-linux` produces a Beta backend with overlay rendering, hints,
  grid, scroll, input injection, multi-monitor (per-monitor rendering, HiDPI /
  fractional scaling, live hotplug), and `monitor_select` on X11 and
  wlroots/KWin Wayland; GNOME/Mutter Wayland stays unsupported.
- `just build-windows` produces a binary with grid, recursive grid,
  scroll, global hotkeys, mouse injection, IPC, and initial UIA accessibility.
- `just release-ci-linux <version>` and `just release-ci-windows <version>` to release a version tagged binary in ci.
- macOS remains the only fully native product path today.

---

## Where To Implement What

Use this table as the default routing guide.

| Capability                                                   | Primary location                                             |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| screen bounds, cursor, dark mode, notifications, permissions | `internal/core/infra/platform/<os>/`                         |
| global hotkeys                                               | `internal/core/infra/hotkeys/`                               |
| keyboard event capture                                       | `internal/core/infra/eventtap/`                              |
| accessibility integration                                    | `internal/core/infra/accessibility/`                         |
| overlay window orchestration                                 | `internal/ui/overlay/`                                       |
| overlay rendering by mode                                    | `internal/app/components/*/overlay_*.go`                     |
| app watcher / isolated platform hooks                        | dispatch-style `platform_*.go` files in the relevant package |

Examples:

- X11 hotkeys belong in [manager_linux_x11.go](../internal/core/infra/hotkeys/manager_linux_x11.go)
- Wayland keyboard capture belongs in [eventtap_linux_wayland.go](../internal/core/infra/eventtap/eventtap_linux_wayland.go)
- shared Linux system fallbacks belong in [system_linux_common.go](../internal/core/infra/platform/linux/system_linux_common.go)

---

## Linux Backend Model

Linux is treated as a backend family, not a single target.

The expected split is:

- `common`: Linux-shared wrapper behavior, current fallback behavior, or backend selection
- `x11`: X11-specific implementation
- `wayland`: Wayland or compositor-specific implementation

This does not mean every Linux package must fully implement both backends
immediately. It means contributors should write code in the right slot from the
start.

Use `common` for:

- shared Linux types
- shared fallback behavior
- backend detection or routing
- helpers reused by both X11 and Wayland

Use `x11` for:

- X11 display enumeration
- X11 event capture
- X11 overlays
- X11 cursor movement or pointer queries

Use `wayland` for:

- compositor-specific capture or overlay behavior
- layer-shell integrations
- Wayland-specific output enumeration or pointer behavior

Accessibility is the main exception: much of Linux accessibility can stay
shared around AT-SPI, even when other capabilities split by backend.

### Wayland compositor backends (two axes)

Wayland is not one target. KDE/KWin, the wlroots family (Sway, Hyprland, niri,
COSMIC), and GNOME/Mutter differ in input, overlay, and hotkey mechanisms. Keep
two axes separate:

- **Compile-time axis** — OS + CGO. Expressed by build tags and the file
  suffixes above. Build tags cannot distinguish compositors (KDE and GNOME are
  both `linux` + Wayland at compile time), so the suffix never encodes a single
  DE on its own.
- **Runtime axis** — which compositor is live. Expressed by the `LinuxBackend`
  family in [linux_backend.go](../internal/core/infra/platform/linux_backend.go)
  (`BackendWaylandWlroots`, `BackendWaylandKDE`, `BackendWaylandGNOME`,
  `BackendWaylandOther`), detected from `XDG_CURRENT_DESKTOP` and routed by the
  factory + dispatch seams (e.g. `system_linux_wayland_input.go`).

Per-DE behavior, protocol measurements, and known issues live in
[LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md). Host setup (deps, build, deploy) lives
in [LINUX_SETUP.md](./LINUX_SETUP.md).

Organize implementation by the axis that actually varies, which is usually the
**mechanism**, not the DE, because DEs share mechanisms:

- Input: KDE uses libei (RemoteDesktop portal); GNOME also uses libei; wlroots
  and COSMIC use `zwlr_virtual_pointer`. So a libei backend can serve multiple
  DEs — do not duplicate it per DE.
- Overlay: layer-shell works on KDE, wlroots, and COSMIC; only GNOME/Mutter
  lacks it.
- Genuinely DE-specific: active-window geometry (KWin D-Bus vs Mutter D-Bus)
  and hotkey registration. Put these in DE-named files (e.g.
  `kwin_geometry_linux.go`).

Use a `*_linux_wayland_<compositor>.go` sub-slot when a compositor family needs
a distinct path that another family does not share. Current slots:
`system_linux_wayland_wlroots_*.go` (virtual-pointer input) and
`system_linux_wayland_kde_*.go` (libei input), with
`system_linux_wayland_input.go` as the shared routing seam.

To add a compositor (e.g. COSMIC): add a `LinuxBackend` value + detection in
`linux_backend.go`, route it in the factory and the relevant dispatch seams,
and only add a new `*_linux_wayland_<compositor>.go` slot if it cannot reuse an
existing mechanism file.

---

## Windows Model

Windows is currently treated as one backend family with basic support.

For now, prefer:

- `*_windows.go` for the implementation slot
- pure Go Win32 / COM bindings where practical

Supported capabilities:

- **grid, recursive grid, scroll** modes — layered GDI overlay, keyboard navigation
- **direct mouse injection** — via `SendInput` / `SetCursorPos`
- **global hotkeys** — via `RegisterHotKey`
- **keyboard event capture** — via `WH_KEYBOARD_LL` hook
- **accessibility** — UI Automation (UIA) COM-based integration (initial coverage)
- **named-pipe IPC** — daemon CLI commands over `\\.\pipe\neru`

Stubbed (not yet implemented):

- notifications (Windows toast notifications)
- app watcher (Win32 foreground-window notifications)
- dark mode detection (personalization APIs)

Do not introduce additional Windows backend naming until there is a real reason.

---

## CGO Guidance

Do not decide CGO usage by OS alone.

Use [profile.go](../internal/core/infra/platform/profile.go)
as the source of truth for the current backend plan.

Current intent:

- macOS: CGO required
- Linux: backend-dependent
- Windows: prefer pure Go first

Good default instincts:

- AT-SPI and freedesktop notifications should prefer pure Go / D-Bus paths
- X11 may be possible in pure Go depending on library choice
- some Wayland/compositor integrations may require CGO or native helpers
- Win32 hotkeys, hooks, monitor APIs, and UIA should prefer pure Go bindings first

If you introduce a backend that changes the build story, update:

- [profile.go](../internal/core/infra/platform/profile.go)
- [justfile](../justfile)
- this document

When in doubt, make the build assumption explicit in your PR description and in
the relevant backend comments.

---

## Hotkeys And Modifiers

Shared code should avoid hard-coding macOS conventions.

Use these rules:

- Use `Primary` when you mean "main accelerator modifier"
- `Primary` maps to `Cmd` on macOS and `Ctrl` on Linux/Windows
- keep backend-specific key translation inside infra/platform code
- do not spread X11, Wayland, Carbon, or Win32 naming into shared app logic

Relevant files:

- [internal/config/config.go](../internal/config/config.go)
- [internal/core/domain/action/modifiers.go](../internal/core/domain/action/modifiers.go)
- [internal/app/hotkeys.go](../internal/app/hotkeys.go)

---

## Adding A New Capability

Use this flow.

### Option A: Broad system capability

If multiple services or app layers will need the capability:

1. Add it to [internal/core/ports/system.go](../internal/core/ports/system.go)
2. Implement it in the darwin adapter
3. Add Linux common fallback behavior in `system_linux_common.go`
4. Add Windows fallback behavior in `system.go` under the Windows platform package
5. If Linux needs backend-specific behavior, push that implementation into `system_linux_x11.go` or `system_linux_wayland.go`
6. Update capability reporting if the support surface changed

### Option B: Isolated package-only platform behavior

If only one infra package needs the capability:

1. Keep the shared package code platform-agnostic
2. Use `platform_darwin.go` and `platform_other.go` dispatch files when that pattern fits
3. If Linux needs backend-specific behavior, add Linux backend files in that package instead of pushing the logic up into shared app code

---

## Error Handling Rules

For unimplemented platform behavior, return `CodeNotSupported`.

Example:

```go
return derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
```

Use clear messages that name the missing operation and platform.

Do not silently no-op unless the behavior is explicitly documented as best-effort.

When a feature becomes real:

- replace the `CodeNotSupported` return
- update capability details
- remove stale TODO wording if it no longer applies

---

## Capability Reporting

Capability reporting is part of the contributor contract, not just a user nicety.

When you implement or partially implement a feature, review:

- [internal/core/ports/capabilities.go](../internal/core/ports/capabilities.go)
- [internal/core/ports/capability_presets.go](../internal/core/ports/capability_presets.go)
- [internal/app/ipc_info.go](../internal/app/ipc_info.go)

`neru doctor` should help contributors and users understand what is actually
available on the current platform.

If a feature remains incomplete, keep the capability honest.

---

## Testing Checklist

Every platform contribution should update tests at the right level.

Use this checklist:

- unit tests for shared parsing, normalization, routing, or config logic
- contract tests for unsupported behavior and capability semantics
- integration tests for real platform behavior on the target OS

Typical test placement:

- `*_test.go`: shared or mocked logic
- `*_integration_linux_test.go`: real Linux behavior
- `*_integration_darwin_test.go`: real macOS behavior
- `*_integration_windows_test.go`: real Windows behavior when added

Good questions to answer in tests:

- does the adapter return the right error when unsupported?
- does the capability matrix reflect the new state?
- does backend selection route to the right Linux slot?
- does shared logic stay platform-neutral?

---

## Documentation Checklist

When you land platform work, update docs in the same PR.

Usually that means checking these files:

- [README.md](../README.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [DEVELOPMENT.md](./DEVELOPMENT.md)
- [LINUX_SETUP.md](./LINUX_SETUP.md) — build, deps, deploy (keep DE-agnostic)
- [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md) — per-DE decisions and known issues
- [CONVENTIONS.md](./go/CONVENTIONS.md)

Update them when:

- the capability matrix changed
- the backend plan changed
- the build or CGO story changed
- a contributor-facing file naming pattern changed

---

## Suggested First Contributions

Good cross-platform starter tasks:

- improve capability detail text for an existing platform slice
- replace a Linux `CodeNotSupported` return with real X11 or AT-SPI behavior
- add a contract test for a currently stubbed feature
- add Linux or Windows integration test scaffolding
- document missing backend assumptions in the package you are touching

Higher-risk tasks:

- changing shared input semantics
- introducing CGO to a backend that was previously pure Go
- moving shared logic into platform packages
- mixing backend detection into app/service code

If your change falls into the higher-risk group, open or link an issue first.

---

## What "Done" Looks Like

A good platform PR usually leaves the repo better in five ways:

- the implementation lives in the intended file slot
- unsupported paths remain explicit and honest
- capability reporting is updated
- tests cover the new behavior or contract
- docs tell the next contributor what changed

That is the bar to aim for, even for small slices.

---

## Feature Parity Reference

This section provides a **ground-truth comparison** of what Neru does on
macOS, Linux, and Windows.

Every claim below is derived from code in `internal/core/infra/platform/`,
`internal/core/infra/accessibility/`, `internal/core/infra/eventtap/`,
`internal/core/infra/hotkeys/`, `internal/ui/overlay/`,
`internal/app/components/`, and the capability presets at
`internal/core/ports/capability_presets.go`. If a doc claim contradicts code,
trust the code.

### OS Support Overview

| Aspect                  | macOS (Darwin)                       | Linux                                             | Windows                                  |
| ----------------------- | ------------------------------------ | ------------------------------------------------- | ---------------------------------------- |
| **Status**              | Production-ready                     | Beta (X11 & Wayland wlroots/KDE)                  | Alpha (initial coverage)                 |
| **CGO Required**        | Yes (Objective-C bridge)             | Backend-dependent                                 | No (pure Go Win32/COM)                   |
| **Build Tag**           | `darwin`                             | `linux`                                           | `windows`                                |
| **Primary Modifier**    | `Cmd`                                | `Ctrl`                                            | `Ctrl`                                   |
| **Display Server**      | Cocoa/Quartz                         | X11, Wayland (wlroots/KDE/GNOME)                  | Win32 Desktop Window Manager             |
| **Accessibility API**   | AXUIElement (Cocoa)                  | AT-SPI (D-Bus)                                    | UI Automation (COM)                      |
| **Overlay Window Type** | NSPanel (borderless, non-activating) | X11 override-redirect / wlr-layer-shell           | Layered HWND (WS_POPUP \| WS_EX_LAYERED) |
| **Overlay Rendering**   | CoreAnimation (CALayer, GPU)         | Cairo (X11: Xlib surface; Wayland: SHM buffers)   | GDI + software SDF (BGRA bitmaps)        |
| **Keyboard Capture**    | CGEventTap (Quartz event taps)       | XGrabKeyboard / evdev grab / layer-shell keyboard | WH_KEYBOARD_LL (SetWindowsHookEx)        |
| **Global Hotkeys**      | Per-key CGEventTap                   | XGrabKey / evdev passive grab                     | RegisterHotKey                           |

### Port Capability Matrix

From `internal/core/ports/capability_presets.go` and actual adapter code.

#### Legend

- ✅ **Supported** — implemented and expected to work in production
- 🟡 **Stub** — code path exists but returns `CodeNotSupported` or no-ops
- ⚠️ **Partial** — works for some backends or has known gaps
- ❌ **Not present** — no code path at all

| Capability                          | macOS                       | Linux (X11)        | Linux (wlroots)           | Linux (KDE)              | Linux (GNOME)     | Windows                  |
| ----------------------------------- | --------------------------- | ------------------ | ------------------------- | ------------------------ | ----------------- | ------------------------ |
| **Focused App PID**                 | ✅                          | ✅                 | ⚠️ (app_id; PID best-effort) | ⚠️ (app_id; PID best-effort) | 🟡                | ✅                       |
| **Screen Bounds**                   | ✅                          | ✅                 | ✅                        | ✅                       | ✅                | ✅                       |
| **Screen Enumeration**              | ✅                          | ✅ (XRandR)        | ✅ (xdg-output)           | ✅ (xdg-output)          | ✅ (xdg-output)   | ✅ (EnumDisplayMonitors) |
| **Cursor Position**                 | ✅                          | ✅                 | ✅ (sync surface)         | ✅ (sync surface)        | ❌                | ✅                       |
| **Cursor Move**                     | ✅                          | ✅ (XTest)         | ✅ (zwlr_virtual_pointer) | ✅ (libei)               | ❌                | ✅ (SetCursorPos)        |
| **Cursor Smooth Animation**         | ✅                          | ✅                 | ✅                        | ✅                       | ❌                | ❌                       |
| **Scroll Smooth Animation**         | ✅                          | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |
| **Accessibility Element Discovery** | ✅ (AXUIElement)            | ⚠️ (AT-SPI walk)   | ⚠️ (AT-SPI walk)          | ⚠️ (AT-SPI walk)         | 🟡 (no injection) | ✅ (UIA initial)         |
| **Accessibility Click/Scroll**      | ✅                          | ✅ (XTest)         | ✅ (virtual-pointer)      | ✅ (libei)               | 🟡                | ✅ (UIA initial)         |
| **Overlay**                         | ✅ (Cocoa NSPanel)          | ✅ (X11 Cairo)     | ✅ (wl-layer Cairo)       | ✅ (wl-layer Cairo)      | ❌                | ✅ (Layered HWND)        |
| **Global Hotkeys**                  | ✅ (CGEventTap)             | ✅ (XGrabKey)      | ✅ (evdev grab)           | ✅ (evdev grab)          | ❌                | ✅ (RegisterHotKey)      |
| **Keyboard Event Capture**          | ✅ (CGEventTap)             | ✅ (XGrabKeyboard) | ✅ (evdev + wl-keyboard)  | ✅ (evdev + wl-keyboard) | ❌                | ✅ (WH_KEYBOARD_LL)      |
| **App Watcher**                     | ✅ (NSWorkspace)            | ✅ (WM_CLASS poll) | ✅ (toplevel app_id poll)  | ✅ (toplevel app_id poll) | 🟡 (no source)    | 🟡                       |
| **Dark Mode Detection**             | ✅ (Cocoa)                  | ✅ (xdg-portal)    | ✅ (xdg-portal)           | ✅ (kdeglobals)          | ✅ (xdg-portal)   | ✅ (registry)            |
| **Secure Input Detection**          | ✅                          | 🟡 (always false)  | 🟡 (always false)         | 🟡 (always false)        | 🟡 (always false) | 🟡 (always false)        |
| **Native Notifications**            | ✅ (NSAlert/UNNotification) | 🟡                 | 🟡                        | 🟡                       | 🟡                | 🟡                       |
| **Native Alerts**                   | ✅ (NSAlert)                | 🟡                 | 🟡                        | 🟡                       | 🟡                | ✅ (Win32 MessageBox)    |
| **Font Resolution**                 | ✅ (NSFont)                 | ✅ (fontconfig)    | ✅ (fontconfig)           | ✅ (fontconfig)          | ✅ (fontconfig)   | ✅ (DirectWrite)         |
| **System Cursor Hide**              | ✅ (CGDisplayHideCursor)    | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |
| **Monitor Select Panels**           | ✅ (native Cocoa panels)    | ✅ (Cairo)         | ✅ (Cairo)                | ✅ (Cairo)               | ❌                | ❌                       |

**Smooth cursor animation on Linux:** off by default and opt-in via
`smooth_cursor.move_mouse_enabled` (the same cross-platform
`SmoothCursorConfig` macOS uses). When enabled, `SystemAdapter.MoveCursorToPoint`
routes through `smoothCursorAnimator` (`mouse_animator_linux.go`) — a single
worker goroutine that samples the current position once, then steps the direct
per-backend warp (XTest on X11, `zwlr_virtual_pointer` on wlroots, libei on KDE)
toward the target with linear interpolation, and `WaitForCursorIdle` now blocks
until it settles instead of returning immediately. This mirrors the darwin
animator (coalescing, latest-target-wins), but drives discrete warps rather than
a Quartz event stream, so there is no drag-event distinction. It applies to the
same flows macOS animates — grid/recursive-grid cursor-follow, `move_mouse`, and
selection moves; clicks stay instant. On Wayland the interpolation start point
comes from the client-side cursor cache; a stale read only skews the glide path,
never the final landing point (the last step lands exactly on the target).
GNOME/Mutter has no injection path, so it stays unsupported there.

**Focused App PID on Wayland:** wlroots and KDE sessions resolve the focused
window through the `wlr-foreign-toplevel-management` protocol, which exposes the
window's **app_id** (used as the bundle identifier for per-app config) but not
its PID — Wayland clients cannot read another client's process credentials.
`SystemPort.FocusedApplicationPID` therefore best-effort matches the app_id
against `/proc`; when no process matches it returns `CodeNotSupported` with the
app_id rather than a fabricated number. GNOME/Mutter does not implement the
protocol, so it stays a stub there (use an X11 session under GNOME).

**App Watcher on Linux:** macOS receives focus changes from an NSWorkspace
observer; Linux has no equivalent push API, so the watcher
(`internal/core/infra/appwatcher/platform_linux.go`) **polls** the focused
application's identity every 400ms and dispatches activate/deactivate events
when it changes. The identity comes from
`platform/linux.FocusedAppID(backend)`: the active window's **WM_CLASS** on X11
(`neru_x11_get_window_class`) and the focused toplevel's **app_id** on Wayland
wlroots/KDE (`wlr-foreign-toplevel-management`, reusing the same source as
Focused App PID). This drives per-app global-hotkey switching
(`App.handleAppActivation`), which previously never fired on Linux. GNOME/Mutter
exposes no focused-app source, so the poll yields nothing and no events fire
there — per-app hotkey switching needs an X11 session under GNOME. Only
activate/deactivate are emitted; launch, terminate, screen-parameter, and
Mission Control events remain macOS-only.

**Accessibility on Linux (⚠️, not a stub):** hints-mode element discovery is
implemented via AT-SPI over D-Bus. `ATSPIClient` (`atspi_linux.go`) enables
assistive-tech mode, finds the active frame, and walks its tree
(`ClickableNodes`) for clickable elements, emitting native AT-SPI role names.
Configured roles are resolved into that vocabulary at config load, so both
sides of the filter speak AT-SPI. This is the client the Linux adapter actually uses
(`platform_client_linux.go` → `Adapter.ClickableElements` → `client.ClickableNodes`);
the `TreeNode`/`BuildTree` stub in `tree_linux.go` is the macOS-style tree API
and is **not** on the Linux hints path. Click/scroll then inject at the hint
point through the embedded `InfraAXClient` — XTest on X11, `zwlr_virtual_pointer`
on wlroots, libei on KDE, plus evdev/uinput for Wayland scroll. It is marked ⚠️
rather than ✅ because coverage depends on each app exposing AT-SPI (Qt/GTK do
with accessibility enabled; some toolkits expose little), and there is no
Vision/OCR fallback like macOS. AT-SPI discovery itself is display-server
independent, but GNOME/Mutter has no usable injection path, so hints are not
usable there (use an X11 session under GNOME).

**Chromium/Electron apps on Linux:** Chromium-based apps (Chrome, Electron,
and forks such as Helium) do **not** expose their web-content accessibility
tree over AT-SPI by default — they gate it behind their own runtime detection
and, unlike macOS, there is no per-app attribute Neru can toggle to force it on
(the macOS `AXManualAccessibility` nudge in `electron.EnsureAccessibility` is a
no-op on Linux). The result is an AT-SPI frame with a single empty child, so
hints find nothing inside such windows. The reliable workaround is to launch the
app with `--force-renderer-accessibility` (e.g. `chromium --force-renderer-accessibility`),
which forces the full web tree. Native GTK/Qt apps and Firefox expose their tree
without this flag. This is a Chromium behavior, not a Neru limitation.

**Active-window selection on Wayland:** the AT-SPI `ACTIVE` state is unreliable
on wlroots compositors (niri, Sway, Hyprland) — the focused window can report
`ACTIVE=false` while background frames report `ACTIVE=true`. Neru therefore
selects the AT-SPI frame by matching the compositor's focused **app_id** (from
`wlr-foreign-toplevel-management`, the same source as Focused App PID / the app
watcher), falling back to the `ACTIVE`/`SHOWING` heuristic only on X11, GNOME,
or when no app_id is available. See `findActiveFrame` in `atspi_linux.go`.

**Window-origin offset on Wayland:** a Wayland client cannot know its own
on-screen position, so AT-SPI reports element coordinates relative to the
window. To place the hint overlay correctly, neru offsets those by the focused
window's screen origin, supplied by a compositor-specific `windowOriginSource`
(`window_origin_linux.go`), selected by environment:

- **KDE / KWin** — a small KWin script pushes the focused window's geometry over
  D-Bus (`kwin_geometry_linux.go`).
- **niri** (`NIRI_SOCKET`) — `niri msg -j focused-window`/`focused-output`.
  Works for **floating** windows (niri populates `tile_pos_in_workspace_view`)
  and **fullscreen** windows (whose frame sits at the output origin with no
  offset needed). For **tiled** windows — including a maximized column
  (`maximize-column`, the default `Mod+F`) — niri does not expose the on-screen
  position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so neru
  falls back to unoffset coordinates and hints are misaligned there. Use a
  floating window, or true fullscreen (`fullscreen-window`), for aligned hints.
- **Sway** (`SWAYSOCK`) — `swaymsg -t get_tree`, focused node `rect + window_rect`.
- **Hyprland** (`HYPRLAND_INSTANCE_SIGNATURE`) — `hyprctl -j activewindow` `at`/`size`.

Each source verifies the reported window size matches the AT-SPI frame (a focus
change can race the query) and is best-effort — an unavailable origin degrades
to unoffset (window-relative) coordinates rather than misplacing hints.

### Overlay Rendering

#### Visual Features

| Feature               | macOS                                            | Linux X11                                 | Linux Wayland                                | Windows                                                   |
| --------------------- | ------------------------------------------------ | ----------------------------------------- | -------------------------------------------- | --------------------------------------------------------- |
| **Window type**       | NSPanel (borderless, non-activating)             | X11 override-redirect Window              | wlr_layer_shell_v1 overlay surface           | Win32 layered HWND                                        |
| **Rendering API**     | CoreAnimation (CALayer, GPU)                     | Cairo (CPU, Xlib)                         | Cairo (CPU, SHM buffers)                     | GDI + software SDF (CPU, BGRA)                            |
| **Per-pixel alpha**   | `[NSColor clearColor]` + layer opaque=NO         | `CAIRO_OPERATOR_CLEAR`                    | `CAIRO_OPERATOR_CLEAR`                       | `AC_SRC_ALPHA` via `UpdateLayeredWindow`                  |
| **Click-through**     | `setIgnoresMouseEvents:YES`                      | XFixes empty input region                 | `wl_surface_set_input_region` (empty)        | `WS_EX_TRANSPARENT`                                       |
| **Always on top**     | `NSScreenSaverWindowLevel`                       | `_NET_WM_STATE_ABOVE` + `MapRaised`       | `ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY`          | `HWND_TOPMOST`                                            |
| **Focus prevention**  | Non-activating panel (returns NO)                | override_redirect=YES, no WM decorations  | Keyboard interactivity: controlled           | `WS_EX_NOACTIVATE`                                        |
| **High-DPI / Retina** | Dynamic `contentsScale`, backing change callback | `Xft.dpi` UI scaling (single global factor) | `wl_output` scale + `wp_fractional_scale_v1`/`wp_viewporter` (crisp fractional) | Not explicit                                              |
| **Multi-monitor**     | Clamps per display, screen change tracking       | Enumerates all monitors, per-monitor render, live RandR hotplug | Per-output `wl_surface` array (max 16), live `wl_output` hotplug | Cursor-screen tracking, separate indicator/sticky windows |
| **Font rendering**    | NSFontManager (postscript/family names)          | Cairo `select_font_face` + `show_text`    | Cairo `select_font_face` + `show_text`       | GDI `CreateFontW` + `DrawTextW` + alpha composite         |
| **Rounded rects**     | NSBezierPath (`bezierPathWithRoundedRect`)       | Cairo arc-based `rounded_path`            | Cairo arc-based `rounded_path`               | SDF `alphaFillRoundedRect` (software)                     |
| **Borders**           | NSBezierPath stroke                              | Cairo stroke (`fill_preserve` + `stroke`) | Cairo stroke                                 | SDF multi-pass alpha `strokeRoundedRect`                  |
| **Coordinate origin** | Bottom-left (Quartz → Y-flip)                    | Top-left                                  | Top-left                                     | Top-left (negative DIB height)                            |
| **Buffer model**      | Layer-backed, OS-managed                         | Single cairo surface                      | Triple-buffered SHM pool (3 buffers)         | Single pixel buffer                                       |

#### Animation & Interaction

| Feature                       | macOS                                              | Linux X11                              | Linux Wayland                               | Windows                                   |
| ----------------------------- | -------------------------------------------------- | -------------------------------------- | ------------------------------------------- | ----------------------------------------- |
| **Grid transition animation** | CoreAnimation 120Hz, ease-in-out + linear fallback | Goroutine 120fps, easeInOut smoothstep | Goroutine 120fps, easeInOut smoothstep      | ❌                                        |
| **Mouse action indicator**    | CoreAnimation CABasicAnimation (scale+opacity)     | Goroutine 120fps, scale+opacity fade   | Goroutine 120fps, scale+opacity fade        | Goroutine 60fps, custom cubic easing, SDF |
| **Smooth cursor animation**   | ✅ (goroutine, configurable easing, tween)         | ✅ (goroutine, stepped warp)           | ✅ (goroutine, stepped warp)                | ❌                                        |
| **Smooth scroll animation**   | ✅ (goroutine, ease-out cubic)                     | ❌                                     | ❌                                          | ❌                                        |
| **Animation frame rate**      | 120fps (NSTimer display link)                      | 120fps (ticker)                        | ~120fps                                     | 60fps (16ms ticker)                       |
| **Thread model**              | Main thread dispatch (dispatch_async/sync)         | `renderMu sync.Mutex`                  | `displayMu sync.Mutex` (shared w/ renderMu) | Dedicated UI thread (`LockOSThread`)      |

#### Visual Elements Per Mode

| Element                        | macOS                                        | Linux X11                        | Linux Wayland                    | Windows                             |
| ------------------------------ | -------------------------------------------- | -------------------------------- | -------------------------------- | ----------------------------------- |
| **Hint badges — rectangular**  | ✅ (with rounded rect + text)                | ✅ (Cairo rounded rect)          | ✅ (Cairo rounded rect)          | ✅ (SDF rounded rect)               |
| **Hint arrows (top/bottom)**   | ✅ (1pt arrow height, NSBezierPath)          | ✅ (Cairo triangle tail)         | ✅ (Cairo triangle tail)         | ❌                                  |
| **Hint boundary highlight**    | ✅ (rounded rect behind element border)      | ✅ (Cairo)                       | ✅ (Cairo)                       | ✅ (SDF, capped 4px radius)         |
| **Hint search input overlay**  | ✅ (`/ query count/` rounded badge)          | ❌                               | ❌                               | ✅ (`/ query count/` rounded badge) |
| **Grid cell labels**           | ✅                                           | ✅                               | ✅                               | ✅                                  |
| **Grid sub-key preview**       | ✅ (centered mini-grid, auto-hide threshold) | ✅ (centered mini-grid)          | ✅ (centered mini-grid)          | ✅ (single-line bottom text)        |
| **Grid auto-hide labels**      | ✅ (fontSize × multiplier threshold)         | ✅                               | ✅                               | ✅                                  |
| **Recursive grid badge**       | ✅                                           | ✅ (Cairo)                       | ✅ (Cairo)                       | ✅ (SDF)                            |
| **Mode indicator**             | ✅ (own NSPanel, positioned on cursor)       | ✅ (DrawBadge on shared overlay) | ✅ (DrawBadge on shared overlay) | ✅ (dedicated indicatorWin window)  |
| **Sticky modifiers indicator** | ✅ (own NSPanel, sized to fit)               | ✅ (DrawBadge on shared overlay) | ✅ (DrawBadge on shared overlay) | ✅ (dedicated stickyWin window)     |
| **Virtual pointer**            | ✅ (own NSPanel, animated w/ grid)           | ✅ (drawn in overlay)            | ✅ (drawn in overlay)            | 🟡                                  |
| **Screen sharing hide**        | ✅ (NSWindowSharingNone/ReadOnly per window) | ❌                               | ❌                               | ❌                                  |

#### Rendering Architecture

The architecture differs fundamentally:

**macOS (Darwin):** Each component owns its own NSPanel. The component
overlay\_\*.go files (e.g., `hints/overlay_darwin.go`) directly call into
Objective-C bridge functions. Full GPU-backed rendering via CoreAnimation.

**Linux:** Component files are stubs (Style + BuildStyle only). Actual
rendering happens in the manager (`manager_linux_x11.go`,
`manager_linux_wayland_cgo.go`) which directly calls C Cairo rendering
functions. All elements draw into a shared Cairo surface.

**Windows:** Same pattern as Linux — component stubs, actual rendering in
manager (`manager_windows.go`, `manager_windows_overlay.go`,
`manager_windows_features.go`) via `winplatform.OverlayWindow` (GDI + SDF).

Key files:

- macOS component overlays: `internal/app/components/{hints,grid,modeindicator,virtualpointer}/overlay_darwin.go`
- Linux component overlays: `internal/app/components/{hints,grid,recursivegrid,modeindicator,stickyindicator}/overlay_linux_*.go` (stubs; actual rendering in manager)
- Windows component overlays: `internal/app/components/{hints,grid,recursivegrid,modeindicator,stickyindicator}/overlay_windows.go` (stubs; actual rendering in manager)

### Accessibility Systems

#### Element Discovery

| Aspect                     | macOS                                                                                                                                                                                    | Linux                                                                   | Windows                                                                              |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| **Backend**                | AXUIElement (via CGO ObjC bridge)                                                                                                                                                        | AT-SPI via D-Bus (pure Go)                                              | UI Automation via COM (pure Go)                                                      |
| **Adapter location**       | `internal/core/infra/accessibility/element_darwin.go`                                                                                                                                    | `internal/core/infra/accessibility/element_linux.go` + `atspi_linux.go` | `internal/core/infra/accessibility/element_windows.go` + `uia_windows.go`            |
| **Client**                 | `InfraAXClient` wraps Element/TreeNode → ObjC bridge                                                                                                                                     | `ATSPIClient` → D-Bus `org.a11y.atspi`                                  | `UIAClient` → raw COM vtable calls                                                   |
| **Tree building**          | Full recursive tree walk of AXUIElement hierarchy (`tree.go`). Collects from: frontmost window, popover windows, menubar, dock, notification center, Stage Manager, PIP, screen capture. | Recursive AT-SPI D-Bus walk of the active frame (`ATSPIClient.ClickableNodes`, `atspi_linux.go`), depth/node capped. The `tree_linux.go` `BuildTree` stub is the macOS-style API and is not on this path. | Shallow UIA tree walk returning root-level clickable nodes (`tree_windows.go`).      |
| **Clickable filtering**    | Extensive: role matching, size/position heuristics, excluded apps list. Multiple strategy backends (AX + Vision).                                                                        | Native AT-SPI role matching, SHOWING-state + on-screen extents check; coverage depends on the app's AT-SPI support. | Basic: collects `IUIAutomationElement` with `IsControlElement` + `IsContentElement`. |
| **Strategy support**       | `config.StrategyAX` (default), `config.StrategyVision` (macOS Vision Framework).                                                                                                         | AX only (no Vision/OCR fallback).                                       | AX only.                                                                             |
| **Popover / menu support** | ✅ (AXOrientation-based popover detection, menubar walking).                                                                                                                             | ⚠️ (popovers/menus in the active frame's AT-SPI subtree are walked; no separate menubar/dock collection). | 🟡                                                                                   |
| **Focused application**    | ✅ (NSWorkspace + AXUIElement).                                                                                                                                                          | ✅ (X11: `_NET_ACTIVE_WINDOW`; Wayland: `wlr-foreign-toplevel` app_id on wlroots/KDE, XWayland fallback). | ✅ (Win32 `GetForegroundWindow`).                                                    |

#### Action Execution

Every action type in `internal/core/domain/action/action.go` is dispatched via `InfraAXClient.PerformAction`:

| Action                  | macOS                                                | Linux                                                                | Windows                                                            |
| ----------------------- | ---------------------------------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------ |
| **Click at point**      | ✅ (`CGEventPost` via ObjC bridge)                   | ✅ (XTest btn 1 / zwlr_virtual_pointer)                              | ✅ (UIA pattern + `SendInput`)                                     |
| **Right click**         | ✅ (`CGEventPost` right mouse down/up)               | ✅ (XTest btn 3 / zwlr_virtual_pointer)                              | ✅ (`SendInput` right button)                                      |
| **Middle click**        | ✅ (`CGEventPost` middle mouse down/up)              | ✅ (XTest btn 2 / zwlr_virtual_pointer)                              | ✅ (`SendInput` middle button)                                     |
| **Mouse down (any button)** | ✅ (`CGEventPost` left/right/other mouse down)   | ✅ (XTest btn 1/2/3 press / zwlr_virtual_pointer press)              | ✅ (`SendInput` left/right/middle down)                            |
| **Mouse up (any button)**   | ✅ (`CGEventPost` left/right/other mouse up)     | ✅ (XTest btn 1/2/3 release / zwlr_virtual_pointer release)          | ✅ (`SendInput` left/right/middle up)                              |
| **Drag while held**     | ✅ (move posts the matching `*MouseDragged` type)    | ✅ (pointer warp with the button held)                               | ✅ (`SetCursorPos` with the button held)                           |
| **Move mouse to point** | ✅ (`CGEventPost` mouse move)                        | ✅ (XTest `XWarpPointer` / zwlr_virtual_pointer)                     | ✅ (`SetCursorPos`)                                                |
| **Move mouse relative** | ✅ (same as absolute, delta applied)                 | ✅ (same as absolute, delta applied)                                 | ✅ (same as absolute, delta applied)                               |
| **Scroll at cursor**    | ✅ (`CGEventCreateScrollWheelEvent` + `CGEventPost`) | ✅ (X11: XTest btn 4/5; Wayland: evdev uinput + wlr virtual-pointer) | ✅ vertical only (`SendInput` `MOUSEEVENTF_WHEEL`; deltaX ignored) |

**Held mouse buttons:** press and release are separate actions, so every backend
has to remember what it pressed. That bookkeeping is shared rather than
per-platform: each adapter keeps a
[`mousestate.Tracker`](../internal/core/infra/platform/mousestate/tracker.go)
recording which buttons are down, where they were pressed, and with which
modifiers. It drives three behaviors identically everywhere — toggle actions
resolve against it (held → release, free → press), `EnsureMouseUp` releases
every held button when Neru returns to idle, and on macOS it selects the drag
event type for cursor moves. macOS is the only backend where a move needs that
distinction: Quartz requires `kCGEventLeftMouseDragged` /
`kCGEventRightMouseDragged` / `kCGEventOtherMouseDragged` (with a matching
button number) instead of `kCGEventMouseMoved`, while X11, Wayland, and Windows
warp the pointer and the compositor or OS infers the drag from the held button.
When several buttons are held at once, a macOS move is attributed to the
left-most held button — a single event cannot describe more than one.

#### Tree Building Comparison

**macOS** (`tree.go`) builds the most comprehensive accessibility tree:

1. Walks the full AXUIElement hierarchy from the frontmost application root
2. Collects from: frontmost window, all windows, popovers, menu bar, dock, notification center
3. Applies per-app strategy overrides and role-based filtering
4. Supports multiple concurrent strategies (AX + Vision)
5. Deduplicates overlapping elements
6. Filters by size, position, visibility, and excluded apps

**Linux** (`atspi_linux.go`) walks the AT-SPI tree over D-Bus:

- `ATSPIClient.FrontmostWindow` finds the active frame (ACTIVE + SHOWING state)
  across registered applications, skipping virtual keyboards and system surfaces
- `ClickableNodes` recursively walks that frame's subtree (depth/node capped),
  keeping SHOWING elements whose AT-SPI role is in the requested set
- Configured roles are resolved from the semantic vocabulary into native
  AT-SPI names before they reach the client
  (`element.ResolveRoles`), so the shared filter pipeline matches them
- On Wayland it offsets element extents by the focused window's screen origin
  (via the KWin geometry bridge) since AT-SPI reports window-relative coords
- Element structs in `element_linux.go` implement cursor/scroll/click injection
- The `tree_linux.go` `BuildTree`/`TreeNode` API is a stub, but it is the
  macOS-style tree path and is **not** used by the Linux adapter

**Windows** (`tree_windows.go`):

- Builds a shallow tree from the root `IUIAutomation` element
- Uses `TreeWalker.GetFirstChildElement` / `GetNextSiblingElement` for traversal
- Filters by bounding rectangle (skip zero-size elements)
- Returns `IUAElement` wrappers with basic metadata
- The `uia_windows.go` provides the raw COM client

### Input Handling

#### Keyboard Event Capture

| Aspect                   | macOS                                            | Linux X11               | Linux Wayland                      | Windows                      |
| ------------------------ | ------------------------------------------------ | ----------------------- | ---------------------------------- | ---------------------------- |
| **Mechanism**            | `CGEventTapCreate` (Quartz)                      | `XGrabKeyboard`         | evdev grab + wl-keyboard           | `WH_KEYBOARD_LL` hook        |
| **CGO required**         | Yes                                              | Yes                     | Yes (cgo path)                     | No                           |
| **Modifier passthrough** | ✅ (CGEventTap callback can pass events through) | ❌ (grab is all-or-nothing) | ✅ (evdev only; re-inject via virtual kbd) | ❌ (no-op)                   |
| **PostModifierEvent**    | ✅                                               | ✅                      | ✅                                 | ❌ (no-op)                   |
| **Sticky modifiers**     | ✅                                               | ✅                      | ✅                                 | ✅                           |
| **File**                 | `eventtap_darwin.go`                             | `eventtap_linux_x11.go` | `eventtap_linux_wayland.go` / `..._evdev_cgo.go` | `eventtap_windows.go`        |
| **Event source**         | System-wide CGEvent stream                       | X11 grabbed keyboard    | `/dev/uinput` + wl_keyboard events | System-wide `WH_KEYBOARD_LL` |

**Modifier passthrough on Linux (Wayland evdev only):** while a mode is active
Neru captures the keyboard exclusively, so shortcuts it doesn't bind (e.g.
`Ctrl+C`, `Ctrl+Tab`) are normally swallowed. With
`general.passthrough_unbounded_keys`, unbound Ctrl/Alt/Cmd chords are instead
re-injected to the focused app. This works on the **Wayland evdev** backend
because Neru holds an `EVIOCGRAB` on the physical device but injects through a
*separate* `zwp_virtual_keyboard_v1`, which bypasses that grab and reaches the
app with no feedback loop (see `handleWaylandEvdevEvent` →
`passthroughEvdevChord`). It is **not** available on **X11** — an `XGrabKeyboard`
routes Neru's own synthetic XTest events back to itself, and `XSendEvent` is
ignored by most apps — nor on the rare **wl-keyboard fallback**, which has no
injection path. Classification (blacklist, mode-intercepted keys, the mode's own
hotkeys) and the post-passthrough hint refresh are shared cross-platform in
`internal/app/modes/passthrough.go`; only the final re-injection is
backend-specific. The blacklist keeps chosen chords consumed, and
`general.should_exit_after_passthrough` exits the mode after a passthrough.

#### Global Hotkeys

| Aspect                               | macOS                              | Linux X11                           | Linux Wayland                          | Windows                   |
| ------------------------------------ | ---------------------------------- | ----------------------------------- | -------------------------------------- | ------------------------- |
| **Mechanism**                        | Per-key CGEventTap                 | `XGrabKey`                          | evdev passive key grab (GrabKey ioctl) | `RegisterHotKey`          |
| **CGO required**                     | Yes                                | Yes                                 | No (pure Go via evdev)                 | No                        |
| **Press/release detection**          | ✅ (separate CGEventTap callbacks) | ✅ (X11 KeyPress/KeyRelease events) | ⚠️ (press only in some configs)        | ✅ (WM_HOTKEY with flags) |
| **Individual hotkey enable/disable** | ✅                                 | ✅                                  | ✅                                     | ✅                        |
| **File**                             | `manager_darwin.go`                | `manager_linux_x11.go`              | `manager_linux_wayland.go`             | `manager_windows.go`      |

#### Cursor & Mouse

| Aspect                  | macOS                                | Linux X11                 | Linux Wayland                                                                    | Windows                    |
| ----------------------- | ------------------------------------ | ------------------------- | -------------------------------------------------------------------------------- | -------------------------- |
| **Get position**        | ✅ (`CGEventGetLocation`)            | ✅ (XQueryPointer)        | ✅ (sync surface trick)                                                          | ✅ (GetCursorPos)          |
| **Move to point**       | ✅ (`CGWarpMouseCursorPosition`)     | ✅ (XTest `XWarpPointer`) | ✅ (zwlr_virtual_pointer)                                                        | ✅ (SetCursorPos)          |
| **Left click**          | ✅ (CGEventPost)                     | ✅ (XTest button 1)       | ✅ (zwlr_virtual_pointer button)                                                 | ✅ (SendInput)             |
| **Right click**         | ✅                                   | ✅ (XTest button 3)       | ✅                                                                               | ✅                         |
| **Middle click**        | ✅                                   | ✅ (XTest button 2)       | ✅                                                                               | ✅                         |
| **Hold / release button** | ✅ (left, right, middle)           | ✅ (XTest button 1/2/3)   | ✅ (zwlr_virtual_pointer BTN_LEFT/RIGHT/MIDDLE)                                  | ✅ (SendInput down/up)     |
| **Drag while held**     | ✅ (`*MouseDragged` event type)      | ✅ (warp with button held) | ✅ (warp with button held)                                                      | ✅ (warp with button held) |
| **Scroll**              | ✅ (CGScrollWheelEvent)              | ✅ (XTest button 4/5)     | ✅ (zwlr_virtual_pointer axis)                                                   | ✅ (SendInput mouse wheel) |
| **Smooth animation**    | ✅ (mouse_animator.go, configurable) | ✅ (mouse_animator_linux.go) | ✅ (mouse_animator_linux.go)                                                  | ❌                         |
| **Wayland cursor sync** | N/A                                  | N/A                       | ✅ (brief map of transparent layer surface to capture `wl_pointer.enter` coords) | N/A                        |

### Mode Feature Coverage

#### Hints Mode

| Feature                        | macOS                                                             | Linux                                       | Windows                         |
| ------------------------------ | ----------------------------------------------------------------- | ------------------------------------------- | ------------------------------- |
| **Element discovery**          | ✅ Full AXUIElement tree walk + Vision framework                  | ⚠️ (AT-SPI walk; toolkit-dependent)         | ⚠️ (UIA initial — shallow tree) |
| **Multi-letter labels**        | ✅                                                                | ✅ (shared alphabet logic)                  | ✅                              |
| **Label filtering / search**   | ✅ (interactive search with `/` prefix, role filter, text filter) | ✅ (logic shared; no search input badge)    | ✅                              |
| **Split word**                 | ✅                                                                | ✅                                          | ✅                              |
| **Label direction**            | ✅ (configurable: horizontal, vertical, row-major, col-major)     | ✅                                          | ✅                              |
| **Hide unmatched**             | ✅                                                                | ✅                                          | ✅                              |
| **Strategy selection**         | `ax` (default), `vision` (macOS Vision Framework)                 | `ax` only                                   | `ax` only                       |
| **Vision fallback**            | ✅ (AX → Vision → combined)                                       | N/A (Vision is macOS-only)                  | N/A                             |
| **Per-app strategy overrides** | ✅                                                                | N/A                                         | N/A                             |
| **Click at point**             | ✅                                                                | ✅ (system mouse click via XTest/SendInput) | ✅ (SendInput)                  |
| **Hover**                      | ✅                                                                | 🟡                                          | 🟡                              |
| **Right click**                | ✅                                                                | ✅ (XTest button 3)                         | ✅ (SendInput right)            |
| **Middle click**               | ✅                                                                | ✅ (XTest button 2)                         | ✅ (SendInput middle)           |
| **Hold button at element**     | ✅ (`--action left_mouse_down` etc.)                              | ✅ (XTest button press)                     | ✅ (SendInput down)             |
| **Scroll at element**          | ✅ (CGScrollWheelEvent)                                           | 🟡                                          | 🟡                              |
| **Menubar elements**           | ✅                                                                | 🟡                                          | 🟡                              |
| **Dock elements**              | ✅                                                                | N/A                                         | N/A                             |
| **Popup/popover elements**     | ✅ (AXOrientation-based)                                          | ⚠️ (walked when in active frame's subtree)  | 🟡                              |
| **Hint overlay arrows**        | ✅ (top/bottom arrow indicators on labels)                        | ✅ (Cairo triangle tail)                    | ❌                              |
| **Boundary highlight**         | ✅                                                                | ✅ (Cairo)                                  | ✅ (SDF)                        |
| **Search input badge**         | ✅                                                                | ❌                                          | ✅                              |

**Implementation note:** On macOS, hints work fully because AXUIElement tree
walking gives a complete picture of clickable elements on screen. On Windows,
the UIA integration provides initial element coverage but the tree is shallow.
On Linux, hints work via an AT-SPI (D-Bus) walk of the active frame, so coverage
depends on the app exposing AT-SPI (Qt/GTK apps with accessibility enabled do;
some toolkits expose little) and there is no Vision/OCR fallback. Chromium/Electron
apps expose nothing unless launched with `--force-renderer-accessibility`, and
GNOME/Mutter has no usable injection path, so hints are not usable there. See the
Linux accessibility notes under [Port Capability Matrix](#port-capability-matrix)
for details.

#### Grid Mode

| Feature                           | macOS                                  | Linux                   | Windows                 |
| --------------------------------- | -------------------------------------- | ----------------------- | ----------------------- |
| **Full-screen grid**              | ✅                                     | ✅                      | ✅                      |
| **Character-labeled cells**       | ✅                                     | ✅                      | ✅                      |
| **Cell navigation (type coords)** | ✅                                     | ✅                      | ✅                      |
| **Subgrid (3×3 zoom)**            | ✅                                     | ✅                      | ✅                      |
| **Hide unmatched cells**          | ✅                                     | ✅                      | ✅                      |
| **Grid transition animation**     | ✅ (CoreAnimation 120Hz)               | ✅ (goroutine 120fps)   | ❌                      |
| **Sub-key preview**               | ✅ (centered mini-grid)                | ✅ (centered mini-grid) | ✅ (single-line text)   |
| **Auto-hide labels**              | ✅                                     | ✅                      | ✅                      |
| **Cursor-follow on activation**   | ✅                                     | ✅                      | ✅                      |
| **Pending action on cell**        | ✅ (click, right-click, hover, scroll) | ✅ (click, right-click) | ✅ (click, right-click) |

#### Recursive Grid Mode

| Feature                     | macOS              | Linux          | Windows |
| --------------------------- | ------------------ | -------------- | ------- |
| **Multi-depth zoom**        | ✅                 | ✅             | ✅      |
| **Per-depth layout config** | ✅                 | ✅             | ✅      |
| **Center preview**          | ✅                 | ✅             | ✅      |
| **Backtrack navigation**    | ✅                 | ✅             | ✅      |
| **Grid transition**         | ✅ (CoreAnimation) | ✅ (goroutine) | ❌      |

#### Scroll Mode

| Feature                     | macOS                                   | Linux | Windows |
| --------------------------- | --------------------------------------- | ----- | ------- |
| **Vim-style scrolling**     | ✅                                      | ✅    | ✅      |
| **Scroll overlay**          | ✅                                      | ✅    | ✅      |
| **Smooth scroll animation** | ✅ (scroll_animator.go, ease-out cubic) | ❌    | ❌      |
| **Line-by-line**            | ✅                                      | ✅    | ✅      |
| **Page-by-page**            | ✅                                      | ✅    | ✅      |
| **Half-page**               | ✅                                      | ✅    | ✅      |
| **Top/bottom**              | ✅                                      | ✅    | ✅      |
| **Custom scroll amounts**   | ✅ (configurable)                       | ✅    | ✅      |

#### Monitor Select Mode

| Feature               | macOS                    | Linux                   | Windows                 |
| --------------------- | ------------------------ | ----------------------- | ----------------------- |
| **Panel per display** | ✅ (native Cocoa panels) | ✅ (Cairo; X11 + wlroots/KDE, not GNOME) | ❌ (`CodeNotSupported`) |
| **Label navigation**  | ✅                       | ✅                      | ❌                      |
| **Cursor jump**       | ✅                       | ✅                      | ❌                      |

### Linux Backend Breakdown

Linux runtime backend detection is via `XDG_CURRENT_DESKTOP`, `WAYLAND_DISPLAY`, and
`DISPLAY` (see `internal/core/infra/platform/linux_backend.go`). See the Port
Capability Matrix above for per-backend capability status and mechanisms.

**GNOME Wayland:** Not supported. GNOME/Mutter lacks `wlr-layer-shell` and has
no usable input injection path. Users should use the X11 session with GNOME.

### Windows Feature Detail

Windows has pure Go implementations for all subsystems (see the Port Capability Matrix above for mechanisms).

**Known gaps on Windows:**

1. **App watcher** — no foreground-window change watching
2. **Notifications** — no Windows toast notifications
3. **Tree walking depth** — UIA tree is shallow; needs deeper traversal for complex apps
4. **Grid transition animation** — not implemented (macOS and Linux have it)
5. **Smooth cursor/scroll animation** — not implemented (smooth cursor exists on macOS + Linux; smooth scroll is macOS-only)
6. **Modifier passthrough, PostModifierEvent** — no-ops
7. **Scroll horizontal** — `ScrollAtCursor` ignores `deltaX` (vertical only)
8. **Virtual pointer overlay** — stub only
9. **Monitor select** — not implemented

### Platform-Specific Features

The following features exist on exactly one platform:

#### macOS-Only

| Feature                   | Location                                                                                | Reason Not Cross-Platform                                                                        |
| ------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| System cursor hide/show   | `internal/app/modes/cursor_darwin.go`                                                   | Uses `CGDisplayHideCursor` / `CGDisplayShowCursor` (Quartz). Other platforms have no equivalent. |
| Smooth scroll animation   | `internal/core/infra/platform/darwin/scroll_animator.go`                                | Platform scroll event stream. (Smooth **cursor** animation is now cross-platform on Linux too — see `mouse_animator_linux.go`.) |
| Vision framework strategy | `internal/core/ports/vision.go` + `internal/core/infra/platform/darwin/vision_darwin.m` | macOS-only `VNRequest` / `VGImageRequestHandler` APIs                                            |
| Screen sharing hide       | `internal/core/infra/platform/darwin/overlay_darwin.m`                                  | NSWindow sharing level property (Quartz only)                                                    |
| Secure input detection    | `internal/core/infra/platform/darwin/secureinput.go`                                    | Uses `CGSessionCopyCurrentDictionary` — private API                                              |

#### Linux-Only

| Feature                         | Location                                                                                       | Notes                                                                        |
| ------------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Wayland sync cursor surface     | `internal/core/infra/platform/linux/system_linux_wayland.go` `SyncCursorPosition()`            | Maps a transparent layer-shell surface to get `wl_pointer.enter` coordinates |
| evdev scroll direct injection   | `internal/core/infra/eventtap/eventtap_linux_wayland_evdev_cgo.go` `IsUinputScrollAvailable()` | Direct `/dev/uinput` scroll; not available on macOS/Windows                  |
| Linux backend detection         | `internal/core/infra/platform/linux_backend.go`                                                | `XDG_CURRENT_DESKTOP` / `WAYLAND_DISPLAY` / `DISPLAY` based routing          |
| X11 event tap via XGrabKeyboard | `internal/core/infra/eventtap/eventtap_linux_x11.go`                                           | X11-specific mechanism                                                       |
| Wayland focused-app app_id      | `internal/core/infra/platform/linux/wlroots_client.c` `neru_wlr_focused_app_id()`              | `wlr-foreign-toplevel-management` tracks the activated toplevel's app_id (wlroots/KDE) |

#### Windows-Only

| Feature                  | Location                                                | Notes                                                                    |
| ------------------------ | ------------------------------------------------------- | ------------------------------------------------------------------------ |
| `WH_KEYBOARD_LL` hook    | `internal/core/infra/eventtap/eventtap_windows.go`      | Low-level keyboard hook (different from macOS CGEventTap or Linux evdev) |
| `RegisterHotKey` hotkeys | `internal/core/infra/hotkeys/manager_windows.go`        | Win32 `RegisterHotKey` API                                               |
| SDF rendering            | `internal/core/infra/platform/windows/overlay_color.go` | Signed-distance-field rounded rectangle rendering (software)             |
| GDI font compositing     | `internal/core/infra/platform/windows/overlay_ui.go`    | GDI `CreateFontW` + `DrawTextW` + alpha composite                        |
