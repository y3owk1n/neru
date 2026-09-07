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
- [COSMIC (Wayland)](#cosmic-wayland)
- [wlroots compositors](#wlroots-compositors)
- [X11 sessions](#x11-sessions)
- [GNOME (Wayland)](#gnome-wayland)
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

## COSMIC (Wayland)

**Backend:** `wayland-cosmic`
**Status:** Supported (cosmic-comp; pointer input needs xdg-desktop-portal-cosmic 1.7 or later)

COSMIC sits between wlroots and KDE. cosmic-comp implements layer-shell and
the virtual keyboard, so overlays, screen geometry and sticky modifiers take
the shared wlroots client unchanged.

Windows are a different story. cosmic-comp has no wlr toplevel manager. It
names its windows through `ext_foreign_toplevel_list_v1`, and its own
`zcosmic_toplevel_info_v1` extends each of those handles with the activated
state and the window's geometry. The shared client tracks that pair in place
of the wlr manager, and one source then answers the focused app, the app
watcher, window-relative hint correction and `FocusedWindowBounds`. No
compositor CLI or scripting bridge is involved, which is more than niri or
Sway give us.

Input and capture go the KDE way. cosmic-comp implements neither
`zwlr_virtual_pointer_v1` nor a screencopy protocol, so pointer input goes
through libei via `org.freedesktop.portal.RemoteDesktop` and screen capture
through `org.freedesktop.portal.ScreenCast`, with the same one-time consent
prompts described under [Screen-sharing consent](#screen-sharing-consent).
Neither is a COSMIC branch in the code. `system_wayland_input.go` asks the
compositor for the virtual pointer and falls back to libei when it is absent.

The catch is the portal version. `xdg-desktop-portal-cosmic` only gained
RemoteDesktop in its 1.7 release, and Fedora 44 ships 1.6. On the older
portal the daemon starts, the overlay draws and keys are captured, but every
pointer action fails and names the missing portal. Keyboard capture and
global hotkeys are the evdev proxy, as on every Wayland session.

### Protocol support (cosmic-comp 1.6.0, measured)

| Protocol                              | Purpose                   | cosmic-comp |
| ------------------------------------- | ------------------------- | ----------- |
| `zwlr_layer_shell_v1`                 | Overlay surfaces          | yes (v5)    |
| `zxdg_output_manager_v1`              | Screen geometry           | yes (v3)    |
| `zwp_virtual_keyboard_manager_v1`     | Sticky-modifier injection | yes         |
| `ext_foreign_toplevel_list_v1`        | Focused-app app_id, title | yes (v1)    |
| `zcosmic_toplevel_info_v1`            | Activated state, geometry | yes (v3)    |
| `zwlr_virtual_pointer_v1`             | Pointer move / click      | **no**      |
| `zwlr_foreign_toplevel_manager_v1`    | (wlr toplevel manager)    | **no**      |
| `zwlr_screencopy_manager_v1`          | Screen capture            | **no**      |

Geometry arrives in each output's own coordinates. Neru adds the output's
global origin back, so a window on a second monitor lands in the same global
top-left space every other backend uses. cosmic-comp replays existing windows
when the daemon connects. Later changes reach it on the compositor's next
refresh.

### Known issues

- Fedora's COSMIC spin ships `xdg-desktop-portal-cosmic` 1.6, which has no
  RemoteDesktop portal. Upgrade it to 1.7 or later for pointer input.

---

## wlroots compositors

**Backend:** `wayland-wlroots`
**Status:** Supported: Sway, Hyprland, niri, River, Wayfire, labwc

Detection is by name for those six. Two more routes land here. A compositor
that tags `XDG_CURRENT_DESKTOP` with `:wlroots`, the convention
xdg-desktop-portal-wlr keys on and labwc's default, and a compositor that
leaves the variable unset. That covers dwl, cage, SwayFX, scroll and most
small wlroots compositors. From there the only question is whether the
compositor advertises the protocols below, and
[Checking compositor protocols](#checking-compositor-protocols) answers it in
one line.

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
  `hyprctl -j activewindow`). River, Wayfire and labwc expose none, so hints
  there stay window-relative. On niri, **tiled** windows, including a maximized column,
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

## GNOME (Wayland)

**Backend:** `wayland-gnome`
**Status:** Supported with Xwayland and the Neru GNOME Shell extension
(GNOME Shell 50 / Mutter, measured)

Mutter implements none of the wlr family beyond `zxdg_output_manager_v1`, so
GNOME borrows more mechanisms from other desktops than any of them, and
carries one of its own:

- **Screens** come off xdg-output through the shared wlroots client, as on
  every other Wayland desktop.
- **Pointer input** goes through libei via `org.freedesktop.portal.RemoteDesktop`,
  the KDE and COSMIC path, with the same one-time "Remote Control" consent and
  stored restore token described under
  [KDE setup notes](#setup-notes-beyond-linux_setupmd). GNOME's portal grants
  the keyboard device together with the pointer, so modified clicks and
  `neru action feed` work from the first grant.
- **Screen capture** is the portal's ScreenCast stream, with the
  [screen-sharing consent](#screen-sharing-consent) picker on the first
  capture-strategy hint refresh.
- **Keyboard capture and global hotkeys** are the evdev proxy, as on every
  Wayland session, and there is no compositor fallback. Without `/dev/input`
  access, bind the modes in **Settings > Keyboard > Custom Shortcuts** and
  expect no keyboard capture inside a mode.
- **The overlay is drawn on Xwayland.** Mutter has no layer shell. A
  fullscreen `xdg_toplevel` was the obvious substitute and I measured it. It
  is the wrong shape for an overlay, because GNOME Shell animates its map and
  unmap and gives it keyboard focus. Mutter stacks an override-redirect X
  window with an empty input shape above every toplevel, never animates or
  focuses it, and passes clicks through it, so the X11 overlay serves GNOME
  unchanged. The daemon refuses to start on a GNOME session with no
  `DISPLAY`, naming Xwayland.
- **Cursor position** uses the same trick. Neru maps a transparent
  override-redirect window that accepts input, the compositor sends the
  pointer's entry with global coordinates, and Neru destroys the window
  again, all inside a frame. Mutter keeps the X root in the logical layout,
  so those coordinates are the ones every other subsystem uses.
- **The focused window comes from a GNOME Shell extension.** Mutter tells a
  client neither which window is focused nor where it is, and the shell's own
  `org.gnome.Shell.Introspect` answers only the portals. Neru ships a small
  extension, `neru@y3owk1n.github.io`, that owns `org.neru.Shell` on the
  session bus and reports the focused window's app id (Mutter's `WM_CLASS`,
  which is the Wayland `app_id` for native clients), title and frame, on
  request and on every change. It is the same fact three readers share, as
  the KWin script is on KDE: per-app config and the app watcher key on the
  app id, hints in native Wayland apps are offset by the frame's origin, and
  `FocusedWindowBounds` reports the frame. Code:
  `internal/adapter/platform/gnomeshell`, `atspi/gnome_origin.go`.

### The extension

The daemon installs the extension itself. On a GNOME session where
`org.neru.Shell` is not on the bus, it writes the two files into
`$XDG_DATA_HOME/gnome-shell/extensions/neru@y3owk1n.github.io/`
(`~/.local/share` by default), adds the UUID to the shell's
`enabled-extensions` setting, and warns once with what it did. GNOME Shell
loads a new extension only at login, so **log out and back in** after the
first daemon start on a machine. Until then every mode works, but per-app
config does not apply and hints in native Wayland apps stay window-relative;
`neru doctor` reports `app_watcher` as a stub with the same instruction. The
files are rewritten only when a new Neru ships a changed extension, and a
reinstall never re-enables one you disabled.

The extension does nothing but answer that one question. It reads
`global.display.focus_window`, watches it for moves, resizes and title
changes, and emits `FocusedWindowChanged`; it neither logs nor stores
anything.

### Protocol support (Mutter 50.4, measured)

| Protocol                              | Purpose                   | Mutter 50.4 |
| ------------------------------------- | ------------------------- | ----------- |
| `zxdg_output_manager_v1`              | Screen geometry           | yes (v3)    |
| `zwlr_layer_shell_v1`                 | Overlay surfaces          | **no**      |
| `zwlr_virtual_pointer_v1`             | Pointer move / click      | **no**      |
| `zwp_virtual_keyboard_manager_v1`     | Sticky-modifier injection | **no**      |
| `zwlr_foreign_toplevel_manager_v1`    | Focused-app app_id        | **no**      |
| `ext_foreign_toplevel_list_v1`        | Focused-app app_id        | **no**      |
| `zwlr_screencopy_manager_v1`          | Screen capture            | **no**      |

### Known issues

- **A fresh install needs one re-login** before the extension serves, see
  above. `neru doctor` says so until it does.
- **HiDPI needs Mutter's default Xwayland scaling.** The overlay draws in the
  X root's coordinate space, which Mutter keeps equal to the logical layout
  by default, and Mutter then upscales the overlay with every other X client
  on a scaled monitor. With the `xwayland-native-scaling` experimental feature
  on, the root is in physical pixels and the overlay lands at the wrong place
  on scaled monitors. The daemon warns at startup when Xft.dpi says so.
- **Budgie** identifies itself as GNOME and takes this backend untested; its
  shell is not GNOME Shell, so the extension has nowhere to load. Cinnamon
  (Muffin) and Pantheon (Gala) are Mutter-based too but resolve to
  `wayland-other` and are refused. Someone has to run the protocol check
  below on each before it can be admitted.

---

## Global hotkeys on Wayland

Applies to every Wayland desktop. X11 is unaffected, it uses `XGrabKey`.

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
needs a desktop-specific input path. KDE, COSMIC and GNOME use libei. If the
layer shell is missing too, the overlay needs one as well, and GNOME draws it
on Xwayland.

When evaluating a new desktop: if both protocols are present the shared
wlroots path applies as-is, and the work is adding the compositor to backend
detection plus a focused-window geometry source. A compositor with layer-shell
but no virtual pointer can still take the libei path if its portal implements
RemoteDesktop, as KDE and COSMIC do. The
daemon refuses to start on a compositor it does not recognize, as
`wayland-other`. Plan a mechanism-specific backend file rather than a whole
per-DE stack, see
[organize by mechanism, not by desktop](CROSS_PLATFORM.md#organize-by-mechanism-not-by-desktop).
