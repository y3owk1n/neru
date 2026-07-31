# Linux Desktop Environments

Per-desktop-environment (DE) implementation notes for Neru on Linux: how each
compositor is wired, important design decisions, and known issues.

**Related:** [Linux setup](./LINUX_SETUP.md) ·
[Cross-Platform Guide](./CROSS_PLATFORM.md) · [Troubleshooting](./TROUBLESHOOTING.md)

---

## Table of Contents

- [KDE Plasma (Wayland)](#kde-plasma-wayland)
- [wlroots compositors](#wlroots-compositors)
- [X11 sessions](#x11-sessions)
- [GNOME (not supported)](#gnome-not-supported)
- [Global hotkeys on Wayland](#global-hotkeys-on-wayland)
- [Checking compositor protocols](#checking-compositor-protocols)

---

## KDE Plasma (Wayland)

**Backend:** `wayland-kde`
**Status:** Supported (Plasma 6 / KWin on Wayland)

### Architecture decisions

KWin does **not** implement `zwlr_virtual_pointer_v1` (confirmed on KWin 6.6.4 via
`wayland-info`). Neru therefore splits responsibilities:

| Concern                         | Mechanism                                                               | Why                                                                                |
| ------------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Overlay + screen geometry       | Shared wlroots client (`zwlr_layer_shell_v1`, `zxdg_output_manager_v1`) | KWin exposes these; same path as Sway/Hyprland                                     |
| Pointer / click / scroll        | `libei` via `org.freedesktop.portal.RemoteDesktop`                      | Only input path KWin exposes for third-party automation                            |
| Key feeding (`action feed`)     | `libei` via `org.freedesktop.portal.RemoteDesktop`                     | Keyboard device must be granted in portal; defaults to pointer-only               |
| Hints (AT-SPI)                  | AT-SPI D-Bus + KWin geometry bridge                                     | AT-SPI coords are window-relative; bridge maps to global compositor space          |
| Global hotkeys                  | Passive `evdev` read of `/dev/input/event*`, else compositor keybindings | Wayland has no global-hotkey protocol; Neru reads keyboards directly when permitted |
| Systray                         | D-Bus StatusNotifierItem + `com.canonical.dbusmenu`                     | Matches KDE/GNOME tray hosts; no GTK dependency                                    |

Routing lives in `system_linux_wayland_input.go`: if the compositor advertises
`zwlr_virtual_pointer_v1`, use the wlroots virtual pointer; otherwise use libei.
The two paths never overlap.

Code slots: `system_linux_wayland_kde_*.go` (libei), shared wlroots client for
overlay, `accessibility/kwin_geometry_linux.go`, `accessibility/atspi_linux.go`.

### Protocol support (KWin 6.6.4, measured)

| Protocol                              | Purpose                   | KWin 6.6.4 |
| ------------------------------------- | ------------------------- | ---------- |
| `zwlr_layer_shell_v1`                 | Overlay surfaces          | yes (v5)   |
| `zxdg_output_manager_v1`              | Screen geometry           | yes (v3)   |
| `zwlr_foreign_toplevel_manager_v1`    | Focused-app app_id        | yes (v3)   |
| `zwlr_virtual_pointer_v1`             | Pointer move / click      | **no**     |
| `zwp_virtual_keyboard_manager_v1`     | Sticky-modifier injection | **no**     |
| `org_kde_kwin_fake_input`             | KWin-native emulation     | **no**     |

See [Checking compositor protocols](#checking-compositor-protocols) for the
`wayland-info` one-liner.

### Setup notes (beyond LINUX_SETUP.md)

1. **RemoteDesktop consent** — First input in a session shows a "Remote Control"
   portal prompt. Approve once per daemon lifetime. The prompt **reappears on
   every fresh daemon start** (reboot, logout, relaunch): `liboeffis` does not
   expose restore-token / `persist_mode`, so KDE cannot persist the grant across
   launches.
2. **Hotkeys** — Neru's own `[hotkeys]` config works on KDE Wayland when the
   daemon can read `/dev/input` (see [Global hotkeys on Wayland](#global-hotkeys-on-wayland)).
   If you would rather not grant that access, bind the modes in **System
   Settings → Shortcuts → Custom Shortcuts** instead. Use the absolute path to
   the binary so KWin resolves it reliably:

    | Action         | Command                                      |
    | -------------- | -------------------------------------------- |
    | Hints          | `/home/<you>/.local/bin/neru hints`          |
    | Grid           | `/home/<you>/.local/bin/neru grid`           |
    | Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
    | Scroll         | `/home/<you>/.local/bin/neru scroll`         |

3. **Portal services** — Input needs `xdg-desktop-portal` and
   `xdg-desktop-portal-kde` running in the session.

### Known issues

- **Consent re-prompt every daemon launch** — See above. Planned follow-up:
  drive `org.freedesktop.portal.RemoteDesktop` directly with a stored
  `restore_token` + `persist_mode` instead of relying on `liboeffis` alone.
- **Modifier keys need a keyboard device from the portal** — If the grant
  includes only a pointer device, modified clicks degrade.
- **Key feeding needs a keyboard device from the portal** — `action feed`
  requires keyboard capability from the RemoteDesktop portal. The portal defaults
  to pointer-only; if keyboard is not granted, `neru action feed` returns
  `CodeNotSupported` with a clear message.
- **Monitor hotplug tracked live** — Plugging or unplugging a monitor is
  detected (via `wl_output` add/remove) and the overlay follows without a
  relaunch. A resolution or scale change to an *existing* monitor is picked up
  on the next mode activation.
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

**"key feeding unavailable on KDE: the RemoteDesktop portal session did not grant a keyboard device"**

The RemoteDesktop portal defaults to pointer-only capability. To enable key
feeding on KDE, start with a fresh portal grant:

**Option A — Plasma 6.5+:** Open **System Settings → Applications → Remote
Desktop**, find any saved `neru` permission, and remove it.

**Option B — Plasma 6.3+ (CLI):** Run:
```sh
flatpak permission-remove kde-authorized remote-desktop ""
```

This clears KDE's portal permission store for host applications. The command
works on any Plasma ≥ 6.3 regardless of whether `flatpak` packages are used.

After clearing the saved permission:

1. Restart the `neru` daemon.
2. When the **"Remote Control" consent prompt** appears by
   `xdg-desktop-portal-kde`, **check the "Enable keyboard" box** before
   clicking "Allow".

The daemon's startup log confirms success with `"Wayland input warm-up
complete"` (keyboard available) or a warning when keyboard was not granted.

**Verifying injected input with the KWin Debug Console**

KWin ships a built-in input inspector, useful for confirming that Neru's
libei events actually reach the compositor:

```bash
qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole
```

The Input Events tab logs every pointer motion, button, and key event with
its source device. Neru's injected events appear with an "Unknown" input
device (libei), while real hardware shows the physical device path.

---

## wlroots compositors

**Backend:** `wayland-wlroots`
**Status:** Supported — Sway, Hyprland, niri, River

### Architecture decisions

| Concern                       | Mechanism                                                                                                         |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Overlay                       | `zwlr_layer_shell_v1` with empty `input_region` for click-through                                                 |
| Pointer / click / scroll      | `zwlr_virtual_pointer_v1`                                                                                         |
| Sticky modifiers              | `zwp_virtual_keyboard_v1` when available                                                                          |
| Key feeding (`action feed`)   | `zwp_virtual_keyboard_v1` — same virtual keyboard path as sticky modifiers                                        |
| Keyboard capture during modes | `evdev` on `/dev/input/event*` (requires `input` group)                                                           |
| Global hotkeys                | Passive `evdev` read of Neru's own `[hotkeys]`; compositor config (`bind` / `bindsym` / `spawn-sh`) as fallback   |
| Cursor position               | Client-side cache (Wayland hides global pointer); "agitation" via layer-shell + virtual pointer wiggle at startup |
| Focused app                   | `zwlr_foreign_toplevel_manager_v1` app_id (no PID; `FocusedApplicationPID` best-effort matches app_id against `/proc`) |

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
- **Monitor hotplug tracked live** — Adding or removing a monitor is detected
  and the overlay follows; a relaunch is only needed for a resolution/scale
  change to an existing monitor on Wayland.

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

The daemon does not start in a GNOME Wayland session: `platform.NewSystemPort`
returns `CodeNotSupported` during the first initialization phase rather than
running in a degraded state. Use a **GNOME X11 session** instead — everything
works there through the `x11` backend.

---

## Global hotkeys on Wayland

Applies to both KDE and wlroots; X11 is unaffected (it uses `XGrabKey`).

No Wayland protocol lets an ordinary client register a global hotkey. Neru
therefore has two paths, and prefers the first:

1. **Neru's own `[hotkeys]` config**, via a **passive** `evdev` listener that
   reads `/dev/input/event*`. It never grabs devices or injects anything, so the
   focused application still receives every key. While a mode is active the
   in-mode event tap grabs the same devices, so the listener goes quiet until
   the mode exits.
2. **Compositor keybindings** — bind `neru hints`, `neru grid`, etc. in your
   compositor config or System Settings. Always available, no permissions
   needed, and the right choice if you prefer not to grant `/dev/input` access.

Path 1 requires two things:

- **Read access to `/dev/input`** — add your user to the `input` group and
  re-login (see [LINUX_SETUP.md](./LINUX_SETUP.md)), or grant narrower access
  via udev/ACL.
- **A CGO build** — evdev support is compiled out when `CGO_ENABLED=0`, leaving
  a no-op stub. Official Linux builds enable CGO.

On startup the daemon logs which path it took: `"Wayland global hotkeys enabled
via evdev; config keybindings are active"` on success, or a warning naming both
the `input` group and the compositor-binding fallback on failure. The hotkey
manager health-check re-initializes the listener if it dies or ends up with zero
devices.

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
