# Linux Desktop Environments

Per-desktop notes for Neru on Linux: what each desktop needs from you, the
consent prompts it shows, the protocols it was measured to offer, and the
issues specific to it.

This is the **per-desktop** layer. Which protocol or API implements each
capability, and why, is in the
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

KWin implements layer-shell but not `zwlr_virtual_pointer_v1`, so overlays and
screen geometry take the shared Wayland client while pointer and keyboard
injection go through **libei** via `org.freedesktop.portal.RemoteDesktop`.
Screen capture goes through `org.freedesktop.portal.ScreenCast`. A KWin script
reports focused-window geometry over D-Bus so AT-SPI's window-relative hint
coordinates can be placed on screen; Neru reinstalls it when KWin restarts.

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

### Setup notes (beyond LINUX_SETUP.md)

1. **RemoteDesktop consent.** The first daemon start shows a "Remote Control"
   portal prompt. **Check "Enable keyboard"** before allowing, or modified
   clicks degrade and `neru action feed` is refused. Neru persists the grant
   with a restore token in `$XDG_STATE_HOME/neru/remote-desktop.token`
   (`~/.local/state/neru/` by default). Deleting that file, or revoking the
   permission in System Settings, brings the prompt back on the next start.
2. **Hotkeys.** Neru's own `[hotkeys]` work when the daemon can read
   `/dev/input`, see [Global hotkeys on Wayland](#global-hotkeys-on-wayland).
   Otherwise bind the modes in **System Settings > Shortcuts > Custom
   Shortcuts** with the absolute path, for example
   `/home/<you>/.local/bin/neru hints`.
3. **Portal services.** Input and screen capture both need
   `xdg-desktop-portal` and `xdg-desktop-portal-kde` running in the session.

### Screen-sharing consent

`hints.strategy = "vision"` or `"contour"` reads the screen through the
portal's ScreenCast session. This is a second grant, separate from "Remote
Control", and it appears the first time a capture-strategy hint activation
needs a frame, as a source picker. Pick every screen you are willing to have
read: a region on a screen you did not share fails rather than coming back
cropped. Neru asks for monitors only, with the pointer left out.

The restore token is kept in `$XDG_STATE_HOME/neru/screen-cast.token`, so later
starts restore the grant with no picker. Deleting that file, or revoking the
permission in **System Settings > Apps & Window Management > Application
Permissions**, brings the picker back. A capture never raises the dialog by
itself, and the PipeWire connection is opened per capture and closed with it,
so KWin is not streaming your screen between frames.

### Known issues

- **Hints coverage** depends on each app exposing an AT-SPI tree. Grid and
  scroll work without AT-SPI.

### Troubleshooting

**"could not establish a libei input session via the RemoteDesktop portal"**

Approve the consent dialog before the connect times out. If denied, revoke and
re-grant in System Settings, and confirm the portal services are running. A
grant revoked while Neru still held its token needs no cleanup: the token is
dropped on the first refusal and the prompt shown again on that start.

**"key feeding unavailable: the RemoteDesktop portal session did not grant a keyboard device"**

The grant was made without the keyboard. Clear it and start over:

- **Plasma 6.5+**: **System Settings > Applications > Remote Desktop**, remove
  any saved `neru` permission.
- **Plasma 6.3+ (CLI)**: `flatpak permission-remove kde-authorized remote-desktop ""`
  clears KDE's portal permission store for host applications, flatpak or not.

Then restart the daemon and **check "Enable keyboard"** in the prompt.

**Verifying injected input** with the KWin Debug Console
(`qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole`): the Input Events tab
shows Neru's events with an "Unknown" device (libei), real hardware with a
device path.

---

## COSMIC (Wayland)

**Backend:** `wayland-cosmic`
**Status:** Supported (cosmic-comp; pointer input needs xdg-desktop-portal-cosmic 1.7 or later)

COSMIC sits between wlroots and KDE. cosmic-comp implements layer-shell and the
virtual keyboard, so overlays, screen geometry and sticky modifiers take the
shared client unchanged. It has no wlr toplevel manager; its
`ext_foreign_toplevel_list_v1` plus `zcosmic_toplevel_info_v1` answer the
focused app, the app watcher, hint offsets and `FocusedWindowBounds` from one
source, with no compositor CLI involved. Pointer input and screen capture go
the KDE way through the two portals, with the same one-time consent prompts
described under [Screen-sharing consent](#screen-sharing-consent).

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

### Known issues

- **Portal version.** `xdg-desktop-portal-cosmic` gained RemoteDesktop in 1.7,
  and Fedora 44 ships 1.6. On the older portal the daemon starts, the overlay
  draws and keys are captured, but every pointer action fails and names the
  missing portal. Upgrade to 1.7 or later.

---

## wlroots compositors

**Backend:** `wayland-wlroots`
**Status:** Supported: Sway, Hyprland, niri, River, Wayfire, labwc

Detection is by name for those six, plus any compositor that tags
`XDG_CURRENT_DESKTOP` with `:wlroots` or leaves it unset, which covers dwl,
cage, SwayFX, scroll and most small wlroots compositors. From there the only
question is whether the compositor advertises the protocols
[Checking compositor protocols](#checking-compositor-protocols) lists.

This is the reference Wayland path: layer-shell overlays, `zwlr_virtual_pointer_v1`
input, `zwp_virtual_keyboard_v1` for sticky modifiers, `zwlr_screencopy_manager_v1`
for capture, and `/dev/uinput` for scrolling and the keyboard proxy.

Two behaviors are specific enough to note here:

- **Cursor position.** Wayland hides the global pointer, so Neru keeps a
  client-side cache and establishes a known position at startup with a
  layer-shell surface plus a virtual-pointer wiggle. On Hyprland it reads
  `hyprctl` instead.
- **Window origins.** niri, Sway and Hyprland expose focused-window geometry
  through their CLIs; River, Wayfire and labwc expose none, so hints there stay
  window-relative. On niri, **tiled** windows expose no on-screen position
  ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are
  misaligned there. Details in
  [CROSS_PLATFORM.md](CROSS_PLATFORM.md#accessibility-and-hints).

### Known issues

- **Device permissions.** Without read access to `/dev/input` there is no
  keyboard proxy: Neru falls back to overlay-focused keyboard capture, modified
  clicks may degrade, and `[hotkeys]` do not fire. Without write access to
  `/dev/uinput` the proxy reads passively and scrolling falls back to the
  virtual pointer, which Chromium and Electron apps on Hyprland ignore. Both
  are install-time steps in
  [LINUX_SETUP.md](./LINUX_SETUP.md#install-time-environment-adjustments).
- **Modified scroll on Hyprland** goes out on the uinput wheel in whole
  notches, because a virtual-pointer scroll under a virtual-keyboard modifier
  produces no event there ([#1474](https://github.com/y3owk1n/neru/pull/1474)).

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
GNOME borrows: pointer input and screen capture take the portal path with the
same "Remote Control" and [screen-sharing](#screen-sharing-consent) consents
as KDE (GNOME's portal grants the keyboard with the pointer, so `feed` works
from the first grant); keyboard capture and global hotkeys are the evdev proxy
with no compositor fallback; and **the overlay is drawn on Xwayland**, because
Mutter stacks an override-redirect X window above every toplevel without
animating or focusing it, where a fullscreen `xdg_toplevel` gets both. The
daemon refuses to start on a GNOME session with no `DISPLAY`, naming Xwayland.

**The focused window comes from a GNOME Shell extension.** Mutter tells a
client neither which window is focused nor where it is, so Neru ships
`neru@y3owk1n.github.io`, which owns `org.neru.Shell` on the session bus and
reports the focused window's app id, title and frame on request and on every
change. Per-app config, hints in native Wayland apps and `FocusedWindowBounds`
all read it. It reads `global.display.focus_window` and nothing else, and
neither logs nor stores anything.

### The extension

The daemon installs it on first start: it writes the two files into
`$XDG_DATA_HOME/gnome-shell/extensions/neru@y3owk1n.github.io/`, adds the UUID
to `enabled-extensions`, and warns once with what it did. GNOME Shell loads a
new extension only at login, so **log out and back in** after the first daemon
start. Until then every mode works, but per-app config does not apply and
hints in native Wayland apps stay window-relative; `neru doctor` reports
`app_watcher` as a stub with the same instruction. The files are rewritten
only when a new Neru ships a changed extension, and a reinstall never
re-enables one you disabled.

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

- **A fresh install needs one re-login** before the extension serves.
  `neru doctor` says so until it does.
- **HiDPI needs Mutter's default Xwayland scaling.** With the
  `xwayland-native-scaling` experimental feature on, the X root is in physical
  pixels and the overlay lands at the wrong place on scaled monitors. The
  daemon warns at startup when `Xft.dpi` says so.
- **Budgie** identifies itself as GNOME and takes this backend untested; its
  shell is not GNOME Shell, so the extension has nowhere to load. Cinnamon and
  Pantheon are Mutter-based too but resolve to `wayland-other` and are refused
  until someone runs the protocol check below on them.

---

## Global hotkeys on Wayland

Applies to every Wayland desktop. X11 is unaffected, it uses `XGrabKey`.

No Wayland protocol lets an ordinary client register a global hotkey, so Neru
offers two paths and prefers the first:

1. **Neru's own `[hotkeys]` config**, through the evdev keyboard proxy. Neru
   holds the keyboards and re-emits every key through a uinput keyboard of its
   own, so a matched chord is consumed before the focused application sees it.
   Without a writable `/dev/uinput` the proxy reads passively: the chord still
   matches, but the application receives it too.
2. **Compositor keybindings.** Bind `neru hints`, `neru grid`, and friends in
   your compositor config or System Settings. Always available, needs no
   permissions, and the right choice if you would rather not grant `/dev/input`
   access.

Path 1 needs **read access to `/dev/input`** (the `input` group, then re-login,
see [LINUX_SETUP.md](./LINUX_SETUP.md#wayland-keyboard-capture-permissions))
and **a CGO build**, which the official Linux builds are. On startup the daemon
logs `Wayland global hotkeys enabled via evdev; config keybindings are active`
on success, or a warning naming both the `input` group and the compositor
fallback.

Bindings keep working from inside a mode, because the proxy hands every press
to the mode session and the mode handler resolves the global table itself. A
chord bound in the *compositor* cannot fire while a mode is open, since the
compositor is not reading the keyboard then. Why the proxy holds the keyboards
at all: [ADR 0014](adr/0014-the-wayland-keyboard-is-a-proxy.md); how it shares
one reader with the in-mode tap:
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#keyboard-capture-and-hotkeys).

---

## Checking compositor protocols

Run inside the graphical session (`WAYLAND_DISPLAY` set):

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|zwlr_screencopy|fake_input|xdg_output'
```

Neru's wlroots path needs **both** `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. A compositor with layer-shell but no virtual pointer
can still take the libei path if its portal implements RemoteDesktop, as KDE
and COSMIC do; one with neither needs its overlay drawn on Xwayland, as GNOME
does. The daemon refuses to start on a compositor it does not recognize, as
`wayland-other`. Adding one is a backend-detection entry, a focused-window
geometry source, and at most a mechanism-specific file, never a per-DE stack:
[organize by mechanism, not by desktop](CROSS_PLATFORM.md#organize-by-mechanism-not-by-desktop).
