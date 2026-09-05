# Linux Desktop Environments

Per-desktop-environment notes for Neru on Linux: measured protocol support,
desktop-specific setup, known issues, and how to diagnose them.

This document is the **empirical, per-desktop** layer. The mechanism behind
each capability, which protocol or API implements it and why, is in the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix). Host preparation
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

Routing lives in `system_wayland_input.go`. If the compositor advertises
`zwlr_virtual_pointer_v1` it uses the virtual pointer, otherwise libei. The two
paths never overlap. Code slots: `platform/linux/system_wayland_kde_*.go`,
`platform/kwin/`, `accessibility/atspi/kwin_origin.go`.

AT-SPI reports window-relative coordinates, so a KWin script pushes
focused-window geometry over D-Bus to translate them into global compositor
space. The same script answers `FocusedWindowBounds`, see
[window-origin offsets](CROSS_PLATFORM.md#accessibility-and-hints). A
`kwin --replace` or a Plasma crash takes the script with it, and Neru reinstalls
it when KWin comes back on the session bus.

### Protocol support (KWin 6.6.4, measured)

| Protocol                              | Purpose                   | KWin 6.6.4 |
| ------------------------------------- | ------------------------- | ---------- |
| `zwlr_layer_shell_v1`                 | Overlay surfaces          | yes (v5)   |
| `zxdg_output_manager_v1`              | Screen geometry           | yes (v3)   |
| `zwlr_foreign_toplevel_manager_v1`    | Focused-app app_id        | yes (v3)   |
| `zwlr_virtual_pointer_v1`             | Pointer move / click      | **no**     |
| `zwp_virtual_keyboard_manager_v1`     | Sticky-modifier injection | **no**     |
| `org_kde_kwin_fake_input`             | KWin-native emulation     | **no**     |
| `zwlr_screencopy_manager_v1`          | Screen capture            | **no**     |

Re-measure with the one-liner under
[Checking compositor protocols](#checking-compositor-protocols).

With no screencopy protocol, screen capture on KDE goes through
`org.freedesktop.portal.ScreenCast` with frames over PipeWire. That is a consent
gate rather than a protocol Neru can bind, see
[Screen-sharing consent](#screen-sharing-consent).

### Setup notes (beyond LINUX_SETUP.md)

1. **RemoteDesktop consent.** The first daemon start on a machine shows a
   "Remote Control" portal prompt. Approve it once. Neru asks the portal to
   persist the session and keeps the restore token in
   `$XDG_STATE_HOME/neru/remote-desktop.token` (`~/.local/state/neru/` by
   default), readable by you alone. Deleting that file, or revoking the
   permission in System Settings, brings the prompt back on the next start.
2. **Hotkeys.** Neru's own `[hotkeys]` config works on KDE Wayland when the
   daemon can read `/dev/input`, see
   [Global hotkeys on Wayland](#global-hotkeys-on-wayland). If you would rather
   not grant that access, bind the modes in **System Settings > Shortcuts >
   Custom Shortcuts** instead, using the absolute path so KWin resolves it
   reliably:

    | Action         | Command                                      |
    | -------------- | -------------------------------------------- |
    | Hints          | `/home/<you>/.local/bin/neru hints`          |
    | Grid           | `/home/<you>/.local/bin/neru grid`           |
    | Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
    | Scroll         | `/home/<you>/.local/bin/neru scroll`         |

3. **Portal services.** Input and screen capture both need
   `xdg-desktop-portal` and `xdg-desktop-portal-kde` running in the session.

### Screen-sharing consent

`hints.strategy = "vision"` or `"contour"` reads the screen through the
portal's ScreenCast session. This is a second grant, separate from "Remote
Control", because sharing your screen and driving your pointer are two
different permissions and KDE asks them separately.

The prompt appears the first time a capture-strategy hint activation needs a
frame, not at startup, and it is a source picker rather than a yes/no dialog.
Pick every screen you are willing to have read: a region on a screen you did
not share fails rather than coming back cropped. Neru asks for monitors only,
never windows, because only a monitor stream says where on screen its pixels
are. The pointer is left out of the frames.

Approve it once. The restore token is kept in
`$XDG_STATE_HOME/neru/screen-cast.token`, readable by you alone, so later
starts restore the grant with no picker. Deleting that file, or revoking the
permission in **System Settings > Apps & Window Management > Application
Permissions**, brings the picker back. A capture never raises the dialog by
itself: a hint refresh that finds no grant fails and says a grant is needed.

The session is established once and reused. The PipeWire connection under it
is opened per capture and closed with it, so KWin is not streaming your screen
between the frames Neru reads.

### Known issues

- **Modifier keys and `feed` need a keyboard device from the portal.** The
  RemoteDesktop grant defaults to pointer-only. Without a keyboard device,
  modified clicks degrade and `neru action feed` returns `CodeNotSupported`
  with a message naming the fix below.
- **Hints coverage** depends on each app exposing an AT-SPI tree. Grid and
  scroll work without AT-SPI.

### Troubleshooting

**"could not establish a libei input session via the RemoteDesktop portal"**

Approve the consent dialog before the connect times out. If denied, revoke and
re-grant in System Settings (Apps & Window Management / portal permissions),
and confirm the portal services are running.

A grant revoked while Neru still held its restore token needs no manual
cleanup: the stored token is dropped on the first refusal and the prompt is
shown once more on that same start. To start clean, delete
`~/.local/state/neru/remote-desktop.token`.

**"key feeding unavailable on KDE: the RemoteDesktop portal session did not grant a keyboard device"**

Start from a fresh portal grant:

- **Plasma 6.5+**: open **System Settings > Applications > Remote Desktop**,
  find any saved `neru` permission, and remove it.
- **Plasma 6.3+ (CLI)**: run

    ```sh
    flatpak permission-remove kde-authorized remote-desktop ""
    ```

    This clears KDE's portal permission store for host applications and works
    on any Plasma 6.3 or later whether or not you use flatpak packages.

Then restart the daemon, and when the **"Remote Control"** prompt appears from
`xdg-desktop-portal-kde`, **check "Enable keyboard"** before clicking Allow. The
startup log confirms with `Wayland input warm-up complete`, or warns when the
keyboard was not granted.

**Verifying injected input with the KWin Debug Console**

```bash
qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole
```

The Input Events tab logs every pointer motion, button, and key event with its
source device. Neru's injected events appear with an "Unknown" input device
(libei). Real hardware shows a physical device path.

---

## wlroots compositors

**Backend:** `wayland-wlroots`
**Status:** Supported: Sway, Hyprland, niri, River, Wayfire

This is the reference Wayland path: `zwlr_layer_shell_v1` overlays with an
empty `input_region` for click-through, `zwlr_virtual_pointer_v1` for pointer
input, `zwp_virtual_keyboard_v1` for sticky modifiers, `zwlr_screencopy_manager_v1`
for screen capture, and `/dev/uinput` for scrolling and the keyboard proxy. The
full per-capability breakdown is in the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix).

Two behaviors are specific enough to note here:

- **Cursor position.** Wayland hides the global pointer, so Neru keeps a
  client-side cache and establishes a known position at startup with a
  layer-shell surface plus a virtual-pointer wiggle. On Hyprland it reads
  `hyprctl` instead.
- **Per-compositor window origins.** niri, Sway, and Hyprland each expose
  focused-window geometry differently (`niri msg`, `swaymsg -t get_tree`,
  `hyprctl -j activewindow`). River and Wayfire expose none, so hints there stay
  window-relative. On niri, **tiled** windows, including a maximized column,
  expose no on-screen position
  ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are
  misaligned there. Details in
  [CROSS_PLATFORM.md](CROSS_PLATFORM.md#accessibility-and-hints).

Code slots: `platform/linux/system_wayland_wlroots_*.go` and the shared wlroots
C client.

### Testing tips

- **Multi-monitor cursor discovery.** Verify the initial cursor position on
  asymmetric layouts after daemon start.
- **Modified keys.** Exercise `Shift`, `Ctrl`, and symbols like `+` / `,` under
  rapid modifier taps.
- **Click-through.** In recursive grid, a synthetic click should reach the app
  beneath the overlay.
- **Scroll.** Scroll mode should feel smooth, with no compositor event-queue
  lag.

### Known issues

- **Device permissions.** Without read access to `/dev/input` there is no
  keyboard proxy: Neru falls back to overlay-focused keyboard capture, modified
  clicks may degrade, and `[hotkeys]` do not fire. Without write access to
  `/dev/uinput` the proxy reads passively and scrolling falls back to the
  virtual pointer, which Chromium and Electron apps on Hyprland ignore. Both
  are install-time steps in
  [LINUX_SETUP.md](./LINUX_SETUP.md#install-time-environment-adjustments).
- **Modified scroll on Hyprland** goes out on the uinput wheel with the
  modifier held on the virtual keyboard, in whole notches, because a
  virtual-pointer scroll under a virtual-keyboard modifier produces no event
  there ([#1474](https://github.com/y3owk1n/neru/pull/1474)).

---

## X11 sessions

**Backend:** `x11`
**Status:** Supported: XOrg, i3, GNOME on X11, and other X11 window managers

The simplest configuration: global hotkeys come from Neru's own config via
`XGrabKey`, input uses XTest, and no compositor keybinding setup is required.
Two limits are the display server's: modifier passthrough is not available
(`XGrabKeyboard` is all-or-nothing), and smooth scroll animates in whole
notches. Build dependencies and systemd deployment are in
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

**Use a GNOME X11 session instead.** Everything works there through the `x11`
backend.

Future work targets libei (the same family as KDE) plus a GNOME Shell extension.
See
[wayland_gnome/PLACEHOLDER.md](../internal/adapter/platform/linux/wayland_gnome/PLACEHOLDER.md).

---

## Global hotkeys on Wayland

Applies to both KDE and wlroots. X11 is unaffected, it uses `XGrabKey`.

No Wayland protocol lets an ordinary client register a global hotkey, so Neru
offers two paths and prefers the first:

1. **Neru's own `[hotkeys]` config**, through the evdev keyboard proxy. Neru
   holds the keyboards and re-emits every key through a uinput keyboard of its
   own, so a matched chord is consumed before the focused application sees it
   and a mode captures keys the instant it opens. Without a writable
   `/dev/uinput` the proxy reads passively instead: the chord still matches,
   but the application receives it too.
2. **Compositor keybindings.** Bind `neru hints`, `neru grid`, and friends in
   your compositor config or System Settings. Always available, needs no
   permissions, and the right choice if you would rather not grant `/dev/input`
   access.

Path 1 has two requirements:

- **Read access to `/dev/input`.** Add your user to the `input` group and
  re-login (see
  [LINUX_SETUP.md](./LINUX_SETUP.md#wayland-keyboard-capture-permissions)), or
  grant narrower access via udev/ACL.
- **A CGO build.** evdev support compiles out when `CGO_ENABLED=0`, leaving a
  stub that reports `CodeNotSupported`. Official Linux builds enable CGO.

On startup the daemon logs which path it took: `Wayland global hotkeys enabled
via evdev; config keybindings are active` on success, or a warning naming both
the `input` group and the compositor-binding fallback on failure. The hotkey
manager health-check re-initializes the listener if it dies or ends up with
zero devices.

Bindings keep working from inside a mode: the proxy hands every press to the
mode session while one is open, and the mode handler resolves the global table
itself. A chord bound in the *compositor* cannot fire while a mode is open,
because the compositor is not reading the keyboard then.

How the proxy shares one reader between hotkeys and the in-mode event tap:
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#keyboard-capture-and-hotkeys). Why it
holds the keyboards at all:
[ADR 0014](adr/0014-the-wayland-keyboard-is-a-proxy.md).

---

## Checking compositor protocols

Run inside the graphical session (`WAYLAND_DISPLAY` set):

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|zwlr_screencopy|fake_input|xdg_output'
```

Neru's wlroots input path needs **both** `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. If the pointer protocol is missing, the compositor
needs a desktop-specific input path. KDE uses libei; GNOME has none yet.

When evaluating a new desktop (COSMIC, for instance): if both protocols are
present the shared wlroots path applies as-is, and the work is adding the
compositor to backend detection plus a focused-window geometry source. The
daemon refuses to start on a compositor it does not recognize, as
`wayland-other`. Plan a mechanism-specific backend file rather than a whole
per-DE stack, see
[organize by mechanism, not by desktop](CROSS_PLATFORM.md#organize-by-mechanism-not-by-desktop).
