# Linux Desktop Environments

Per-desktop-environment notes for Neru on Linux: measured protocol support,
desktop-specific setup, known issues, and how to diagnose them.

This document is the **empirical, per-desktop** layer. The mechanism behind each
capability — which protocol or API implements it, and why — is in the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix); host preparation
(dependencies, permissions, build, deploy) is in
[LINUX_SETUP.md](./LINUX_SETUP.md).

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

### Why KDE differs from wlroots

KWin does **not** implement `zwlr_virtual_pointer_v1`, so Neru cannot use the
wlroots input path here. It splits the difference: overlays and screen geometry
go through the shared wlroots client (KWin does implement layer-shell), while
all pointer and keyboard injection goes through **libei** via
`org.freedesktop.portal.RemoteDesktop`.

Routing lives in `system_wayland_input.go` — if the compositor advertises
`zwlr_virtual_pointer_v1` it uses the virtual pointer, otherwise libei. The two
paths never overlap. Code slots: `platform/linux/system_wayland_kde_*.go`,
`platform/kwin/`, `accessibility/atspi/kwin_origin.go`,
`accessibility/atspi/client.go`.

AT-SPI reports window-relative coordinates, so a KWin script pushes
focused-window geometry over D-Bus to translate them into global compositor
space — the same script every other focused-window answer here reads (see
[window-origin offsets](CROSS_PLATFORM.md#accessibility-and-hints)).

### Protocol support (KWin 6.6.4, measured)

| Protocol                              | Purpose                   | KWin 6.6.4 |
| ------------------------------------- | ------------------------- | ---------- |
| `zwlr_layer_shell_v1`                 | Overlay surfaces          | yes (v5)   |
| `zxdg_output_manager_v1`              | Screen geometry           | yes (v3)   |
| `zwlr_foreign_toplevel_manager_v1`    | Focused-app app_id        | yes (v3)   |
| `zwlr_virtual_pointer_v1`             | Pointer move / click      | **no**     |
| `zwp_virtual_keyboard_manager_v1`     | Sticky-modifier injection | **no**     |
| `org_kde_kwin_fake_input`             | KWin-native emulation     | **no**     |

Re-measure with the one-liner under
[Checking compositor protocols](#checking-compositor-protocols).

`zwlr_screencopy_manager_v1` is absent here too — KWin's own capture path is
`zkde_screencast_unstable_v1` and the portal's ScreenCast session, both of which
deliver frames over PipeWire. Neru therefore captures the screen here through
`org.freedesktop.portal.ScreenCast`, which is a consent gate rather than a
protocol Neru can simply bind: see
[Screen-sharing consent](#screen-sharing-consent) below. That claim about the
protocol set is read from KWin rather than taken from the measured session
above; the one-liner below now greps for it, so you can confirm it on your own
KWin.

### Setup notes (beyond LINUX_SETUP.md)

1. **RemoteDesktop consent** — the first daemon start on a machine shows a
   "Remote Control" portal prompt. Approve it once and later starts reuse the
   same grant with no prompt: Neru asks the portal to persist the session and
   keeps the restore token it hands back in
   `$XDG_STATE_HOME/neru/remote-desktop.token` (`~/.local/state/neru/…` by
   default), readable by you alone. Deleting that file, or revoking the
   permission in System Settings, brings the prompt back on the next start.
2. **Hotkeys** — Neru's own `[hotkeys]` config works on KDE Wayland when the
   daemon can read `/dev/input` (see
   [Global hotkeys on Wayland](#global-hotkeys-on-wayland)). If you would rather
   not grant that access, bind the modes in **System Settings → Shortcuts →
   Custom Shortcuts** instead, using the absolute path so KWin resolves it
   reliably:

    | Action         | Command                                      |
    | -------------- | -------------------------------------------- |
    | Hints          | `/home/<you>/.local/bin/neru hints`          |
    | Grid           | `/home/<you>/.local/bin/neru grid`           |
    | Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
    | Scroll         | `/home/<you>/.local/bin/neru scroll`         |

3. **Portal services** — input needs `xdg-desktop-portal` and
   `xdg-desktop-portal-kde` running in the session. So does screen capture.

### Screen-sharing consent

KWin has no screencopy protocol, so `hints.strategy = vision` reads the screen
through the portal's ScreenCast session — a second, separate grant from the
"Remote Control" one above, because sharing your screen and driving your
pointer are two different permissions and KDE asks them separately.

The prompt appears the first time a vision-strategy hint activation needs a
frame, not at startup, and it is a source picker rather than a yes/no dialog.
Pick every screen you are willing to have read: a region on a screen you did
not share fails rather than coming back cropped, and Neru asks for monitors
only — never windows — because only a monitor stream says where on screen its
pixels are. The pointer is left out of the frames.

Approve it once. Neru asks the portal to persist the session and keeps the
restore token in `$XDG_STATE_HOME/neru/screen-cast.token`
(`~/.local/state/neru/…` by default), readable by you alone, so later starts
restore the same grant with no picker. Deleting that file, or revoking the
permission in **System Settings → Apps & Window Management → Application
Permissions**, brings the picker back. No capture ever raises the dialog by
itself — a hint refresh that finds no grant fails and says a grant is needed.

The session is established once and reused; the PipeWire connection under it is
opened per capture and closed with it, so KWin is not streaming your screen
between the frames Neru actually reads.

### Known issues

- **Modifier keys need a keyboard device from the portal** — if the grant
  includes only a pointer device, modified clicks degrade.
- **Key feeding needs a keyboard device from the portal** — `action feed`
  requires keyboard capability from the RemoteDesktop portal, which defaults to
  pointer-only. Without it, `neru action feed` returns `CodeNotSupported` with a
  clear message.
- **Hints coverage** — depends on each app exposing an AT-SPI tree. Grid and
  scroll work without AT-SPI.

### Troubleshooting

**"could not establish a libei input session via the RemoteDesktop portal"**

Approve the consent dialog before the connect times out. If denied, revoke and
re-grant in System Settings (Apps & Window Management / portal permissions).
Confirm the portal services are running.

A grant that was revoked while Neru still held its restore token needs no
manual cleanup: the stored token is dropped on the first refusal and the prompt
is shown once more, on that same start. If you would rather start clean, delete
`~/.local/state/neru/remote-desktop.token`.

**"compositor does not support zwlr_virtual_pointer_v1" on KDE**

Expected — Neru routes input through libei on KDE. The message only indicates a
real problem on compositors that have neither a virtual pointer nor a libei
path.

**"key feeding unavailable on KDE: the RemoteDesktop portal session did not grant a keyboard device"**

The portal defaults to pointer-only capability. To enable key feeding, start
from a fresh portal grant:

- **Plasma 6.5+** — open **System Settings → Applications → Remote Desktop**,
  find any saved `neru` permission, and remove it.
- **Plasma 6.3+ (CLI)** — run:

    ```sh
    flatpak permission-remove kde-authorized remote-desktop ""
    ```

    This clears KDE's portal permission store for host applications, and works
    on any Plasma ≥ 6.3 whether or not you use flatpak packages.

Then restart the daemon, and when the **"Remote Control"** prompt appears from
`xdg-desktop-portal-kde`, **check "Enable keyboard"** before clicking Allow. The
startup log confirms with `"Wayland input warm-up complete"`, or warns when the
keyboard was not granted.

**Verifying injected input with the KWin Debug Console**

KWin ships an input inspector, useful for confirming Neru's libei events
actually reach the compositor:

```bash
qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole
```

The Input Events tab logs every pointer motion, button, and key event with its
source device. Neru's injected events appear with an "Unknown" input device
(libei); real hardware shows a physical device path.

---

## wlroots compositors

**Backend:** `wayland-wlroots`
**Status:** Supported — Sway, Hyprland, niri, River

This is the reference Wayland path: `zwlr_layer_shell_v1` overlays with an empty
`input_region` for click-through, `zwlr_virtual_pointer_v1` for pointer input,
`zwp_virtual_keyboard_v1` for sticky modifiers and `action feed`, and
`zwlr_screencopy_manager_v1` for screen capture. The full per-capability
breakdown is in the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix).

Two behaviors are specific enough to note here:

- **Cursor position** — Wayland hides the global pointer, so Neru keeps a
  client-side cache and "agitates" it at startup with a layer-shell surface plus
  a virtual-pointer wiggle to establish a known position.
- **Per-compositor window origins** — niri, Sway, and Hyprland each expose
  focused-window geometry differently (`niri msg`, `swaymsg -t get_tree`,
  `hyprctl -j activewindow`). On niri, **tiled** windows — including a maximized
  column — expose no on-screen position
  ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are
  misaligned there. Details in
  [CROSS_PLATFORM.md](CROSS_PLATFORM.md#accessibility-and-hints).

Code slots: `platform/linux/system_wayland_wlroots_*.go` and the shared wlroots
C client.

### Testing tips

- **Multi-monitor cursor discovery** — verify the initial cursor position on
  asymmetric layouts after daemon start.
- **Modified keys** — exercise `Shift`, `Ctrl`, and symbols like `+` / `,` under
  rapid modifier taps.
- **Click-through** — in recursive grid, a synthetic click should reach the app
  beneath the overlay.
- **Scroll** — scroll mode should feel smooth, with no compositor event-queue
  lag.

### Known issues

- **`evdev` access** — without membership in the `input` group (or a tighter
  udev/ACL setup), Neru falls back to overlay-focused keyboard capture and
  modified clicks may degrade.

---

## X11 sessions

**Backend:** `x11`
**Status:** Supported — XOrg, i3, and other X11 window managers

The simplest configuration: global hotkeys come from Neru's own config via
`XGrabKey`, input uses XTest, and no compositor keybinding setup is required.
Build dependencies and systemd deployment are in
[LINUX_SETUP.md](./LINUX_SETUP.md).

---

## GNOME (not supported)

**Backend:** `wayland-gnome`
**Status:** Not supported

**The daemon does not start in a GNOME Wayland session.**
`platform.NewSystemPort` returns `CodeNotSupported` during the first
initialization phase rather than running in a degraded state.

GNOME Shell uses private protocols instead of the wlr family: Mutter implements
neither `wlr-layer-shell` (overlays) nor `wlr-foreign-toplevel-management`
(focused app), and exposes no input-injection path Neru can use.

**Use a GNOME X11 session instead** — everything works there through the `x11`
backend.

Future work targets libei (the same family as KDE) plus a GNOME Shell extension.
See
[wayland_gnome/PLACEHOLDER.md](../internal/adapter/platform/linux/wayland_gnome/PLACEHOLDER.md).

---

## Global hotkeys on Wayland

Applies to both KDE and wlroots. X11 is unaffected — it uses `XGrabKey`.

No Wayland protocol lets an ordinary client register a global hotkey, so Neru
offers two paths and prefers the first:

1. **Neru's own `[hotkeys]` config**, through a passive `evdev` listener. It
   never grabs devices or injects anything, so the focused application still
   receives every key.
2. **Compositor keybindings** — bind `neru hints`, `neru grid`, and friends in
   your compositor config or System Settings. Always available, needs no
   permissions, and the right choice if you would rather not grant `/dev/input`
   access.

Path 1 has two requirements:

- **Read access to `/dev/input`** — add your user to the `input` group and
  re-login (see
  [LINUX_SETUP.md](./LINUX_SETUP.md#wayland-keyboard-capture-permissions)), or
  grant narrower access via udev/ACL.
- **A CGO build** — evdev support compiles out when `CGO_ENABLED=0`, leaving a
  no-op stub. Official Linux builds enable CGO.

On startup the daemon logs which path it took: `"Wayland global hotkeys enabled
via evdev; config keybindings are active"` on success, or a warning naming both
the `input` group and the compositor-binding fallback on failure. The hotkey
manager health-check re-initializes the listener if it dies or ends up with zero
devices.

Why a passive listener is the only option, and how it interacts with the in-mode
event tap, is explained in
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#keyboard-capture-and-hotkeys).

---

## Checking compositor protocols

Run inside the graphical session (`WAYLAND_DISPLAY` set):

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|zwlr_screencopy|fake_input|xdg_output'
```

Neru's wlroots input path needs **both** `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. If the pointer protocol is missing, the compositor
needs a desktop-specific input path — KDE uses libei; GNOME has none yet.

When evaluating a new desktop (COSMIC, for instance): if both protocols are
present the shared wlroots path applies as-is. Otherwise plan a
mechanism-specific backend file rather than duplicating a whole per-DE stack —
see [organize by mechanism, not by desktop](CROSS_PLATFORM.md#organize-by-mechanism-not-by-desktop).
