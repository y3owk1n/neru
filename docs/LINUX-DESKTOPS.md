# Linux Desktop Environments

Per-desktop-environment (DE) implementation notes for Neru on Linux: how each
compositor is wired, important design decisions, and known issues. For building,
installing dependencies, and preparing a host to run Neru, see
[LINUX_SETUP.md](./LINUX_SETUP.md).

---

## Table of Contents

- [KDE Plasma (Wayland)](#kde-plasma-wayland)
- [wlroots compositors](#wlroots-compositors)
- [X11 sessions](#x11-sessions)
- [GNOME (not supported)](#gnome-not-supported)
- [Checking compositor protocols](#checking-compositor-protocols)

---

## KDE Plasma (Wayland)

**Backend:** `wayland-kde`  
**Status:** Supported (Plasma 6 / KWin on Wayland)

### Architecture decisions

KWin does **not** implement `zwlr_virtual_pointer_v1` (confirmed on KWin 6.6.4 via
`wayland-info`). Neru therefore splits responsibilities:

| Concern | Mechanism | Why |
| ------- | --------- | --- |
| Overlay + screen geometry | Shared wlroots client (`zwlr_layer_shell_v1`, `zxdg_output_manager_v1`) | KWin exposes these; same path as Sway/Hyprland |
| Pointer / click / scroll / keys | `libei` via `org.freedesktop.portal.RemoteDesktop` | Only input path KWin exposes for third-party automation |
| Hints (AT-SPI) | AT-SPI D-Bus + KWin geometry bridge | AT-SPI coords are window-relative; bridge maps to global compositor space |
| Global hotkeys | Compositor keybindings only | Wayland has no global-hotkey protocol; user binds `neru <mode>` in System Settings |
| Systray | D-Bus StatusNotifierItem + `com.canonical.dbusmenu` | Matches KDE/GNOME tray hosts; no GTK dependency |

Routing lives in `system_linux_wayland_input.go`: if the compositor advertises
`zwlr_virtual_pointer_v1`, use the wlroots virtual pointer; otherwise use libei.
The two paths never overlap.

Code slots: `system_linux_wayland_kde_*.go` (libei), shared wlroots client for
overlay, `accessibility/kwin_geometry_linux.go`, `accessibility/atspi_linux.go`.

### Protocol support (KWin 6.6.4, measured)

| Protocol | Purpose | KWin 6.6.4 |
| -------- | ------- | ---------- |
| `zwlr_layer_shell_v1` | Overlay surfaces | yes (v5) |
| `zxdg_output_manager_v1` | Screen geometry | yes (v3) |
| `zwlr_virtual_pointer_v1` | Pointer move / click | **no** |
| `zwp_virtual_keyboard_manager_v1` | Sticky-modifier injection | **no** |
| `org_kde_kwin_fake_input` | KWin-native emulation | **no** |

See [Checking compositor protocols](#checking-compositor-protocols) for the
`wayland-info` one-liner.

### Setup notes (beyond LINUX_SETUP.md)

1. **RemoteDesktop consent** — First input in a session shows a "Remote Control"
   portal prompt. Approve once per daemon lifetime. The prompt **reappears on
   every fresh daemon start** (reboot, logout, relaunch): `liboeffis` does not
   expose restore-token / `persist_mode`, so KDE cannot persist the grant across
   launches.
2. **Hotkeys** — Bind in **System Settings -> Shortcuts -> Custom Shortcuts**.
   Use the absolute path to the binary so KWin resolves it reliably:

   | Action | Command |
   | ------ | ------- |
   | Hints | `/home/<you>/.local/bin/neru hints` |
   | Grid | `/home/<you>/.local/bin/neru grid` |
   | Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
   | Scroll | `/home/<you>/.local/bin/neru scroll` |

3. **Portal services** — Input needs `xdg-desktop-portal` and
   `xdg-desktop-portal-kde` running in the session.

### Known issues

- **Consent re-prompt every daemon launch** — See above. Planned follow-up:
  drive `org.freedesktop.portal.RemoteDesktop` directly with a stored
  `restore_token` + `persist_mode` instead of relying on `liboeffis` alone.
- **Modifier keys need a keyboard device from the portal** — If the grant
  includes only a pointer device, modified clicks degrade.
- **Screen geometry cached at startup** — Resolution changes after launch
  (monitor hotplug, VM resize) require a daemon relaunch. Same limitation as
  other Wayland backends; tracked for live refresh.
- **Hints coverage** — Depends on each app exposing an AT-SPI tree. Grid and
  scroll work without AT-SPI.

### Troubleshooting

**"could not establish a libei input session via the RemoteDesktop portal"**

Approve the consent dialog before the connect times out. If denied, revoke and
re-grant in System Settings (Apps & Window Management / portal permissions).
Confirm portal services are running.

**"compositor does not support zwlr_virtual_pointer_v1" on KDE**

Expected. Neru routes input through libei on KDE; this message applies to
compositors that lack both virtual pointer and a libei path.

---

## wlroots compositors

**Backend:** `wayland-wlroots`  
**Status:** Supported — Sway, Hyprland, niri, River

### Architecture decisions

| Concern | Mechanism |
| ------- | --------- |
| Overlay | `zwlr_layer_shell_v1` with empty `input_region` for click-through |
| Pointer / click / scroll | `zwlr_virtual_pointer_v1` |
| Sticky modifiers | `zwp_virtual_keyboard_v1` when available |
| Keyboard capture during modes | `evdev` on `/dev/input/event*` (requires `input` group) |
| Global hotkeys | Compositor config (`bind` / `bindsym` / `spawn-sh`) |
| Cursor position | Client-side cache (Wayland hides global pointer); "agitation" via layer-shell + virtual pointer wiggle at startup |

Code slots: `system_linux_wayland_wlroots_*.go`, shared wlroots C client.

### Testing tips

- **Multi-monitor cursor discovery** — Verify initial cursor position on asymmetric
  layouts after daemon start.
- **Modified keys** — Exercise `Shift`, `Ctrl`, and symbols like `+` / `,` under
  rapid modifier taps.
- **Click-through** — In recursive grid, synthetic click should reach the app
  under the overlay.
- **Scroll** — Scroll mode should feel smooth; no compositor event-queue lag.

### Known issues

- **`evdev` access** — Without membership in the `input` group (or tighter
  udev/ACL), Neru falls back to overlay-focused keyboard capture; modified
  clicks may degrade.
- **Screen geometry cached at startup** — Relaunch after resolution or monitor
  changes.

---

## X11 sessions

**Backend:** `x11`  
**Status:** Supported — XOrg, i3, etc.

Global hotkeys use `XGrabKey` from Neru's config. Input uses XTest. No compositor
keybinding setup required. See [LINUX_SETUP.md](./LINUX_SETUP.md) for build deps
and systemd deployment.

---

## GNOME (not supported)

**Backend:** `wayland-gnome`  
**Status:** Not supported

GNOME Shell uses private protocols instead of wlr layer-shell / virtual pointer.
Future work targets libei (same family as KDE) plus a GNOME Shell extension.
See `internal/core/infra/platform/linux/wayland_gnome/PLACEHOLDER.md`.

---

## Checking compositor protocols

Run inside the graphical session (`WAYLAND_DISPLAY` set):

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|fake_input|xdg_output'
```

Neru's wlroots input path needs **both** `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. If the pointer protocol is missing, the compositor
needs a desktop-specific input path (KDE uses libei; GNOME is not yet supported).

When evaluating a new desktop (e.g. COSMIC): if both protocols are present, the
shared wlroots path applies; otherwise plan a mechanism-specific backend file
rather than duplicating per DE.
