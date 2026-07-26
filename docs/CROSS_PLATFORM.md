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

- `just build-linux` is currently best viewed as a
  foundations smoke test while that platform is still mostly scaffolding.
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
| **Focused App PID**                 | ✅                          | ✅                 | 🟡                        | 🟡                       | 🟡                | ✅                       |
| **Screen Bounds**                   | ✅                          | ✅                 | ✅                        | ✅                       | ✅                | ✅                       |
| **Screen Enumeration**              | ✅                          | ✅ (XRandR)        | ✅ (xdg-output)           | ✅ (xdg-output)          | ✅ (xdg-output)   | ✅ (EnumDisplayMonitors) |
| **Cursor Position**                 | ✅                          | ✅                 | ✅ (sync surface)         | ✅ (sync surface)        | ❌                | ✅                       |
| **Cursor Move**                     | ✅                          | ✅ (XTest)         | ✅ (zwlr_virtual_pointer) | ✅ (libei)               | ❌                | ✅ (SetCursorPos)        |
| **Cursor Smooth Animation**         | ✅                          | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |
| **Scroll Smooth Animation**         | ✅                          | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |
| **Accessibility Element Discovery** | ✅ (AXUIElement)            | 🟡 (AT-SPI stub)   | 🟡 (AT-SPI stub)          | 🟡 (AT-SPI stub)         | 🟡 (AT-SPI stub)  | ✅ (UIA initial)         |
| **Accessibility Click/Scroll**      | ✅                          | 🟡                 | 🟡                        | 🟡                       | 🟡                | ✅ (UIA initial)         |
| **Overlay**                         | ✅ (Cocoa NSPanel)          | ✅ (X11 Cairo)     | ✅ (wl-layer Cairo)       | ✅ (wl-layer Cairo)      | ❌                | ✅ (Layered HWND)        |
| **Global Hotkeys**                  | ✅ (CGEventTap)             | ✅ (XGrabKey)      | ✅ (evdev grab)           | ✅ (evdev grab)          | ❌                | ✅ (RegisterHotKey)      |
| **Keyboard Event Capture**          | ✅ (CGEventTap)             | ✅ (XGrabKeyboard) | ✅ (evdev + wl-keyboard)  | ✅ (evdev + wl-keyboard) | ❌                | ✅ (WH_KEYBOARD_LL)      |
| **App Watcher**                     | ✅ (NSWorkspace)            | 🟡                 | 🟡                        | 🟡                       | 🟡                | 🟡                       |
| **Dark Mode Detection**             | ✅ (Cocoa)                  | ✅ (xdg-portal)    | ✅ (xdg-portal)           | ✅ (kdeglobals)          | ✅ (xdg-portal)   | ✅ (registry)            |
| **Secure Input Detection**          | ✅                          | 🟡 (always false)  | 🟡 (always false)         | 🟡 (always false)        | 🟡 (always false) | 🟡 (always false)        |
| **Native Notifications**            | ✅ (NSAlert/UNNotification) | 🟡                 | 🟡                        | 🟡                       | 🟡                | 🟡                       |
| **Native Alerts**                   | ✅ (NSAlert)                | 🟡                 | 🟡                        | 🟡                       | 🟡                | ✅ (Win32 MessageBox)    |
| **Font Resolution**                 | ✅ (NSFont)                 | ✅ (fontconfig)    | ✅ (fontconfig)           | ✅ (fontconfig)          | ✅ (fontconfig)   | ✅ (DirectWrite)         |
| **System Cursor Hide**              | ✅ (CGDisplayHideCursor)    | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |
| **Monitor Select Panels**           | ✅ (native Cocoa panels)    | ❌                 | ❌                        | ❌                       | ❌                | ❌                       |

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
| **High-DPI / Retina** | Dynamic `contentsScale`, backing change callback | Not explicit                              | `wl_output` scale listener + `cairo_scale()` | Not explicit                                              |
| **Multi-monitor**     | Clamps per display, screen change tracking       | Resize to active screen cursor            | Per-output `wl_surface` array (max 16)       | Cursor-screen tracking, separate indicator/sticky windows |
| **Font rendering**    | NSFontManager (postscript/family names)          | Cairo `select_font_face` + `show_text`    | Cairo `select_font_face` + `show_text`       | GDI `CreateFontW` + `DrawTextW` + alpha composite         |
| **Rounded rects**     | NSBezierPath (`bezierPathWithRoundedRect`)       | Cairo arc-based `rounded_path`            | Cairo arc-based `rounded_path`               | SDF `alphaFillRoundedRect` (software)                     |
| **Borders**           | NSBezierPath stroke                              | Cairo stroke (`fill_preserve` + `stroke`) | Cairo stroke                                 | SDF multi-pass alpha `strokeRoundedRect`                  |
| **Coordinate origin** | Bottom-left (Quartz → Y-flip)                    | Top-left                                  | Top-left                                     | Top-left (negative DIB height)                            |
| **Buffer model**      | Layer-backed, OS-managed                         | Single cairo surface                      | Triple-buffered SHM pool (3 buffers)         | Single pixel buffer                                       |

#### Animation & Interaction

| Feature                       | macOS                                              | Linux X11                              | Linux Wayland                               | Windows                                   |
| ----------------------------- | -------------------------------------------------- | -------------------------------------- | ------------------------------------------- | ----------------------------------------- |
| **Grid transition animation** | CoreAnimation 120Hz, ease-in-out + linear fallback | Goroutine 120fps, easeInOut smoothstep | Goroutine 120fps, easeInOut smoothstep      | ❌                                        |
| **Mouse action indicator**    | CoreAnimation CABasicAnimation (scale+opacity)     | ❌                                     | ❌                                          | Goroutine 60fps, custom cubic easing, SDF |
| **Smooth cursor animation**   | ✅ (goroutine, configurable easing, tween)         | ❌                                     | ❌                                          | ❌                                        |
| **Smooth scroll animation**   | ✅ (goroutine, ease-out cubic)                     | ❌                                     | ❌                                          | ❌                                        |
| **Animation frame rate**      | 120fps (NSTimer display link)                      | 120fps (ticker)                        | ~120fps                                     | 60fps (16ms ticker)                       |
| **Thread model**              | Main thread dispatch (dispatch_async/sync)         | `renderMu sync.Mutex`                  | `displayMu sync.Mutex` (shared w/ renderMu) | Dedicated UI thread (`LockOSThread`)      |

#### Visual Elements Per Mode

| Element                        | macOS                                        | Linux X11                        | Linux Wayland                    | Windows                             |
| ------------------------------ | -------------------------------------------- | -------------------------------- | -------------------------------- | ----------------------------------- |
| **Hint badges — rectangular**  | ✅ (with rounded rect + text)                | ✅ (Cairo rounded rect)          | ✅ (Cairo rounded rect)          | ✅ (SDF rounded rect)               |
| **Hint arrows (top/bottom)**   | ✅ (1pt arrow height, NSBezierPath)          | ❌                               | ❌                               | ❌                                  |
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
| **Tree building**          | Full recursive tree walk of AXUIElement hierarchy (`tree.go`). Collects from: frontmost window, popover windows, menubar, dock, notification center, Stage Manager, PIP, screen capture. | Stub — `ClickableNodes` returns `(nil, nil)` (`tree_linux.go`).         | Shallow UIA tree walk returning root-level clickable nodes (`tree_windows.go`).      |
| **Clickable filtering**    | Extensive: role matching, size/position heuristics, excluded apps list. Multiple strategy backends (AX + Vision).                                                                        | Stub.                                                                   | Basic: collects `IUIAutomationElement` with `IsControlElement` + `IsContentElement`. |
| **Strategy support**       | `config.StrategyAX` (default), `config.StrategyVision` (macOS Vision Framework).                                                                                                         | AX only.                                                                | AX only.                                                                             |
| **Popover / menu support** | ✅ (AXOrientation-based popover detection, menubar walking).                                                                                                                             | 🟡                                                                      | 🟡                                                                                   |
| **Focused application**    | ✅ (NSWorkspace + AXUIElement).                                                                                                                                                          | ⚠️ (X11: `_NET_ACTIVE_WINDOW`; Wayland: XWayland).                      | ✅ (Win32 `GetForegroundWindow`).                                                    |

#### Action Execution

All 8 action types from `internal/core/domain/action/action.go` are dispatched via `InfraAXClient.PerformAction`:

| Action                  | macOS                                                | Linux                                                                | Windows                                                            |
| ----------------------- | ---------------------------------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------ |
| **Click at point**      | ✅ (`CGEventPost` via ObjC bridge)                   | ✅ (XTest btn 1 / zwlr_virtual_pointer)                              | ✅ (UIA pattern + `SendInput`)                                     |
| **Right click**         | ✅ (`CGEventPost` right mouse down/up)               | ✅ (XTest btn 3 / zwlr_virtual_pointer)                              | ✅ (`SendInput` right button)                                      |
| **Middle click**        | ✅ (`CGEventPost` middle mouse down/up)              | ✅ (XTest btn 2 / zwlr_virtual_pointer)                              | ✅ (`SendInput` middle button)                                     |
| **Mouse down**          | ✅ (`CGEventPost` left mouse down)                   | ✅ (XTest btn 1 press / zwlr_virtual_pointer press)                  | ✅ (`SendInput` left down)                                         |
| **Mouse up**            | ✅ (`CGEventPost` left mouse up)                     | ✅ (XTest btn 1 release / zwlr_virtual_pointer release)              | ✅ (`SendInput` left up)                                           |
| **Move mouse to point** | ✅ (`CGEventPost` mouse move)                        | ✅ (XTest `XWarpPointer` / zwlr_virtual_pointer)                     | ✅ (`SetCursorPos`)                                                |
| **Move mouse relative** | ✅ (same as absolute, delta applied)                 | ✅ (same as absolute, delta applied)                                 | ✅ (same as absolute, delta applied)                               |
| **Scroll at cursor**    | ✅ (`CGEventCreateScrollWheelEvent` + `CGEventPost`) | ✅ (X11: XTest btn 4/5; Wayland: evdev uinput + wlr virtual-pointer) | ✅ vertical only (`SendInput` `MOUSEEVENTF_WHEEL`; deltaX ignored) |

#### Tree Building Comparison

**macOS** (`tree.go`) builds the most comprehensive accessibility tree:

1. Walks the full AXUIElement hierarchy from the frontmost application root
2. Collects from: frontmost window, all windows, popovers, menu bar, dock, notification center
3. Applies per-app strategy overrides and role-based filtering
4. Supports multiple concurrent strategies (AX + Vision)
5. Deduplicates overlapping elements
6. Filters by size, position, visibility, and excluded apps

**Linux** (`tree_linux.go`) is a stub:

- Returns immediately with no elements
- AT-SPI client (`atspi_linux.go`) connects via D-Bus but tree queries are stubbed
- The `collectClickableNodes` function returns nil
- Element structs in `element_linux.go` implement cursor/scroll/click but tree queries are stubs

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
| **Modifier passthrough** | ✅ (CGEventTap callback can pass events through) | ❌ (no-op)              | ❌ (no-op)                         | ❌ (no-op)                   |
| **PostModifierEvent**    | ✅                                               | ✅                      | ✅                                 | ❌ (no-op)                   |
| **Sticky modifiers**     | ✅                                               | ✅                      | ✅                                 | ✅                           |
| **File**                 | `eventtap_darwin.go`                             | `eventtap_linux_x11.go` | `eventtap_linux_wayland.go`        | `eventtap_windows.go`        |
| **Event source**         | System-wide CGEvent stream                       | X11 grabbed keyboard    | `/dev/uinput` + wl_keyboard events | System-wide `WH_KEYBOARD_LL` |

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
| **Scroll**              | ✅ (CGScrollWheelEvent)              | ✅ (XTest button 4/5)     | ✅ (zwlr_virtual_pointer axis)                                                   | ✅ (SendInput mouse wheel) |
| **Smooth animation**    | ✅ (mouse_animator.go, configurable) | ❌                        | ❌                                                                               | ❌                         |
| **Wayland cursor sync** | N/A                                  | N/A                       | ✅ (brief map of transparent layer surface to capture `wl_pointer.enter` coords) | N/A                        |

### Mode Feature Coverage

#### Hints Mode

| Feature                        | macOS                                                             | Linux                                       | Windows                         |
| ------------------------------ | ----------------------------------------------------------------- | ------------------------------------------- | ------------------------------- |
| **Element discovery**          | ✅ Full AXUIElement tree walk + Vision framework                  | 🟡 (stub — returns no elements)             | ⚠️ (UIA initial — shallow tree) |
| **Multi-letter labels**        | ✅                                                                | N/A (no elements)                           | ✅                              |
| **Label filtering / search**   | ✅ (interactive search with `/` prefix, role filter, text filter) | N/A                                         | ✅                              |
| **Split word**                 | ✅                                                                | N/A                                         | ✅                              |
| **Label direction**            | ✅ (configurable: horizontal, vertical, row-major, col-major)     | N/A                                         | ✅                              |
| **Hide unmatched**             | ✅                                                                | N/A                                         | ✅                              |
| **Strategy selection**         | `ax` (default), `vision` (macOS Vision Framework)                 | `ax` only                                   | `ax` only                       |
| **Vision fallback**            | ✅ (AX → Vision → combined)                                       | N/A (Vision is macOS-only)                  | N/A                             |
| **Per-app strategy overrides** | ✅                                                                | N/A                                         | N/A                             |
| **Click at point**             | ✅                                                                | ✅ (system mouse click via XTest/SendInput) | ✅ (SendInput)                  |
| **Hover**                      | ✅                                                                | 🟡                                          | 🟡                              |
| **Right click**                | ✅                                                                | ✅ (XTest button 3)                         | ✅ (SendInput right)            |
| **Middle click**               | ✅                                                                | ✅ (XTest button 2)                         | ✅ (SendInput middle)           |
| **Scroll at element**          | ✅ (CGScrollWheelEvent)                                           | 🟡                                          | 🟡                              |
| **Menubar elements**           | ✅                                                                | 🟡                                          | 🟡                              |
| **Dock elements**              | ✅                                                                | N/A                                         | N/A                             |
| **Popup/popover elements**     | ✅ (AXOrientation-based)                                          | 🟡                                          | 🟡                              |
| **Hint overlay arrows**        | ✅ (top/bottom arrow indicators on labels)                        | ❌                                          | ❌                              |
| **Boundary highlight**         | ✅                                                                | ✅ (Cairo)                                  | ✅ (SDF)                        |
| **Search input badge**         | ✅                                                                | ❌                                          | ✅                              |

**Implementation note:** On macOS, hints work fully because AXUIElement tree
walking gives a complete picture of clickable elements on screen. On Windows,
the UIA integration provides initial element coverage but the tree is shallow.
On Linux, AT-SPI integration is stubbed — no clickable elements can be
discovered.

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
| **Panel per display** | ✅ (native Cocoa panels) | ❌ (`CodeNotSupported`) | ❌ (`CodeNotSupported`) |
| **Label navigation**  | ✅                       | ❌                      | ❌                      |
| **Cursor jump**       | ✅                       | ❌                      | ❌                      |

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
5. **Smooth cursor/scroll animation** — not implemented (macOS only)
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
| Smooth cursor animation   | `internal/core/infra/platform/darwin/mouse_animator.go`                                 | Quartz event-level animation tracking. Requires platform-specific cursor event stream.           |
| Smooth scroll animation   | `internal/core/infra/platform/darwin/scroll_animator.go`                                | Same — platform scroll event stream                                                              |
| Vision framework strategy | `internal/core/ports/vision.go` + `internal/core/infra/platform/darwin/vision_darwin.m` | macOS-only `VNRequest` / `VGImageRequestHandler` APIs                                            |
| Monitor select panels     | `internal/app/modes/monitor_select_overlay_darwin.go`                                   | Uses Cocoa NSPanel per display. Not implemented on other platforms.                              |
| Screen sharing hide       | `internal/core/infra/platform/darwin/overlay_darwin.m`                                  | NSWindow sharing level property (Quartz only)                                                    |
| Per-hint arrow indicators | `internal/app/components/hints/overlay_darwin.go`                                       | NSBezierPath arrow drawing — not in Cairo/SDF renderers                                          |
| Secure input detection    | `internal/core/infra/platform/darwin/secureinput.go`                                    | Uses `CGSessionCopyCurrentDictionary` — private API                                              |

#### Linux-Only

| Feature                         | Location                                                                                       | Notes                                                                        |
| ------------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Wayland sync cursor surface     | `internal/core/infra/platform/linux/system_linux_wayland.go` `SyncCursorPosition()`            | Maps a transparent layer-shell surface to get `wl_pointer.enter` coordinates |
| evdev scroll direct injection   | `internal/core/infra/eventtap/eventtap_linux_wayland_evdev_cgo.go` `IsUinputScrollAvailable()` | Direct `/dev/uinput` scroll; not available on macOS/Windows                  |
| Linux backend detection         | `internal/core/infra/platform/linux_backend.go`                                                | `XDG_CURRENT_DESKTOP` / `WAYLAND_DISPLAY` / `DISPLAY` based routing          |
| X11 event tap via XGrabKeyboard | `internal/core/infra/eventtap/eventtap_linux_x11.go`                                           | X11-specific mechanism                                                       |

#### Windows-Only

| Feature                  | Location                                                | Notes                                                                    |
| ------------------------ | ------------------------------------------------------- | ------------------------------------------------------------------------ |
| `WH_KEYBOARD_LL` hook    | `internal/core/infra/eventtap/eventtap_windows.go`      | Low-level keyboard hook (different from macOS CGEventTap or Linux evdev) |
| `RegisterHotKey` hotkeys | `internal/core/infra/hotkeys/manager_windows.go`        | Win32 `RegisterHotKey` API                                               |
| SDF rendering            | `internal/core/infra/platform/windows/overlay_color.go` | Signed-distance-field rounded rectangle rendering (software)             |
| GDI font compositing     | `internal/core/infra/platform/windows/overlay_ui.go`    | GDI `CreateFontW` + `DrawTextW` + alpha composite                        |
