# Cross-Platform Contribution Guide

Practical guidance for contributors working on Linux, Windows, or platform-neutral infrastructure in Neru.

For the high-level design, see [Architecture](ARCHITECTURE.md).

---

## First Steps

Before changing code, read these files:

1. `internal/core/infra/platform/profile.go` — Build and backend plan
2. `internal/core/ports/capabilities.go` — Capability definitions
3. `internal/core/ports/system.go` — System port interface

```bash
# Baseline
just build
just test-foundation

# Linux contributors
just build-linux      # Linux foundations binary

# Windows contributors
just build-windows    # Windows foundations binary
```

---

## File Layout

| Suffix                            | Purpose                       |
| :-------------------------------- | :---------------------------- |
| `*_darwin.go`                     | macOS implementation          |
| `*_windows.go`                    | Windows implementation        |
| `*_linux_common.go`               | Shared Linux wrapper/fallback |
| `*_linux_x11.go`                  | Linux X11 implementation      |
| `*_linux_wayland.go`              | Linux Wayland implementation  |
| `*_linux_wayland_<compositor>.go` | Per-compositor sub-slot       |
| `*_other.go`                      | Non-target fallback           |

Don't create empty files for symmetry — only add a file for a real implementation.

---

## Where to Implement What

| Capability                                                   | Location                                 |
| :----------------------------------------------------------- | :--------------------------------------- |
| Screen bounds, cursor, dark mode, notifications, permissions | `internal/core/infra/platform/<os>/`     |
| Global hotkeys                                               | `internal/core/infra/hotkeys/`           |
| Keyboard event capture                                       | `internal/core/infra/eventtap/`          |
| Accessibility integration                                    | `internal/core/infra/accessibility/`     |
| Overlay window orchestration                                 | `internal/ui/overlay/`                   |
| Overlay rendering by mode                                    | `internal/app/components/*/overlay_*.go` |

---

## Linux Backend Model

Linux is a **backend family**, not a single target. Runtime compositor is detected from `XDG_CURRENT_DESKTOP`.

| Compositor           | Backend         | Input Mechanism           | File Slot                    |
| :------------------- | :-------------- | :------------------------ | :--------------------------- |
| Sway, Hyprland, niri | wayland-wlroots | `zwlr_virtual_pointer_v1` | `*_linux_wayland_wlroots.go` |
| KDE Plasma           | wayland-kde     | libei (RemoteDesktop)     | `*_linux_wayland_kde.go`     |
| X11/XOrg, i3         | x11             | XTest                     | `*_linux_x11.go`             |
| GNOME                | wayland-gnome   | Not supported             | —                            |

### Wayland Compositor Guidance

Organize implementation by **mechanism**, not by desktop environment:

- **Shared mechanisms** — Overlay via `zwlr_layer_shell_v1` works on KDE, wlroots, and COSMIC. Put in shared wayland files.
- **DE-specific** — Active-window geometry (KWin D-Bus vs Mutter D-Bus) and hotkey registration go in DE-named files.

To add a new compositor: add a `LinuxBackend` value + detection in `linux_backend.go`, route it in the factory and dispatch seams, and only create a new file slot if it can't reuse an existing mechanism file.

---

## Per-Desktop Environment Details

### KDE Plasma (Wayland)

**Status:** Supported (Plasma 6 / KWin on Wayland)

KWin does not implement `zwlr_virtual_pointer_v1`. Neru uses **libei** (via `org.freedesktop.portal.RemoteDesktop`) for input injection.

| Concern                   | Mechanism                                                               |
| :------------------------ | :---------------------------------------------------------------------- |
| Overlay + screen geometry | Shared wlroots client (`zwlr_layer_shell_v1`, `zxdg_output_manager_v1`) |
| Pointer/click/scroll/keys | libei via RemoteDesktop portal                                          |
| Hints (AT-SPI)            | AT-SPI D-Bus + KWin geometry bridge                                     |
| Global hotkeys            | System Settings → Shortcuts → Custom Shortcuts                          |

**Setup notes:**

1. Portal services `xdg-desktop-portal` and `xdg-desktop-portal-kde` must be running
2. RemoteDesktop consent prompt appears on every daemon restart (not persisted)
3. Use absolute binary paths in KDE custom shortcuts

**Known issues:**

- Consent re-prompt on every daemon launch
- Modifier keys need a keyboard device from the portal
- Screen geometry cached at startup — relaunch after resolution changes
- Hints coverage depends on AT-SPI exposure; grid/scroll work without it

**Troubleshooting:**

```
qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole   # KWin input inspector
```

### wlroots Compositors (Sway, Hyprland, niri, River)

**Status:** Supported

| Concern              | Mechanism                                               |
| :------------------- | :------------------------------------------------------ |
| Overlay              | `zwlr_layer_shell_v1` with empty `input_region`         |
| Pointer/click/scroll | `zwlr_virtual_pointer_v1`                               |
| Sticky modifiers     | `zwp_virtual_keyboard_v1`                               |
| Keyboard capture     | `evdev` on `/dev/input/event*` (requires `input` group) |
| Global hotkeys       | Compositor config (`bindsym`/`bind`)                    |

**Known issues:** Without `input` group membership, falls back to overlay-focused keyboard capture; modified clicks may degrade. Screen geometry cached at startup.

### X11 Sessions

**Status:** Supported

Global hotkeys use `XGrabKey` from config. Input uses XTest. No compositor keybinding setup needed.

### GNOME (Not Supported)

GNOME Shell lacks `zwlr_layer_shell_v1` and `zwlr_virtual_pointer_v1`. Future work would target libei plus a GNOME Shell extension. The platform factory detects GNOME via `XDG_CURRENT_DESKTOP` and returns `CodeNotSupported`.

### Checking Compositor Protocols

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|xdg_output'
```

Neru's wlroots input path needs **both** `zwlr_layer_shell_v1` and `zwlr_virtual_pointer_v1`.

---

## Windows Model

Single backend family. Prefer pure Go Win32/COM bindings.

**Supported:** grid/recursive-grid/scroll, mouse injection, global hotkeys, keyboard hooks, UIA accessibility (initial), named-pipe IPC, GDI overlay.

**Stubbed:** notifications, app watcher, dark mode detection.

---

## CGO Guidance

| Platform |   CGO Required?   | Notes                         |
| :------- | :---------------: | :---------------------------- |
| macOS    |        ✅         | Native bridge via Objective-C |
| Linux    | Backend-dependent | Check `profile.go`            |
| Windows  |        ❌         | Prefer pure Go first          |

If you introduce a new CGO dependency, update `profile.go`, `justfile`, and this document.

---

## Hotkeys & Modifiers

- **`Primary`** — main accelerator modifier (`Cmd` on macOS, `Ctrl` on Linux/Windows)
- Keep backend-specific key translation inside infra/platform code
- Relevant files: `internal/config/config.go`, `internal/core/domain/action/modifiers.go`, `internal/app/hotkeys.go`

---

## Adding a New Capability

### Broad System Capability

1. Add to `internal/core/ports/system.go`
2. Implement in darwin adapter
3. Add Linux fallback in `system_linux_common.go`
4. Add Windows fallback
5. Add Linux backend-specific in `system_linux_x11.go` or `system_linux_wayland.go`
6. Update capability reporting

### Isolated Package-Only Platform Behavior

1. Keep shared code platform-agnostic
2. Use `platform_darwin.go` + `platform_other.go` dispatch files
3. Add Linux backend files in that package if needed

---

## Error Handling

For unimplemented platform behavior, return `CodeNotSupported`:

```go
return derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
```

---

## Capability Reporting

When you implement a feature, update `internal/core/ports/capabilities.go`, `internal/core/ports/capability_presets.go`, and `internal/app/ipc_info.go`. `neru doctor` should accurately reflect platform state.

---

## Testing Checklist

| Type        | File                         | Purpose                                    |
| :---------- | :--------------------------- | :----------------------------------------- |
| Unit        | `*_test.go`                  | Shared parsing, routing, config            |
| Contract    | `*_test.go`                  | Unsupported behavior, capability semantics |
| Integration | `*_integration_<os>_test.go` | Real platform behavior                     |

---

## Documentation Checklist

Update when platform work lands: `README.md` (capability matrix), `ARCHITECTURE.md` (platform status), `INSTALLATION.md` (build deps), this guide.

---

## What "Done" Looks Like

1. ✅ Implementation lives in the intended file slot
2. ✅ Unsupported paths remain explicit (`CodeNotSupported`)
3. ✅ Capability reporting is updated
4. ✅ Tests cover new behavior or contract
5. ✅ Docs tell the next contributor what changed
