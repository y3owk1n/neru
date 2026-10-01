# Linux desktop environments

Host setup for every desktop, such as libraries, the `input` group, the
`/dev/uinput` rule and the systemd service, is in [Linux setup](./linux.md).
Which protocol implements each capability is in the
[capability matrix](../reference/platform-support.md#capability-matrix).

## KDE Plasma (Wayland)

**Backend:** `wayland-kde`. **Status:** Supported (Plasma 6 / KWin on Wayland).

Pointer and keyboard input go through libei and the RemoteDesktop portal, and
screen capture through the ScreenCast portal. Neru installs a KWin script that
reports focused-window geometry, and reinstalls it when KWin restarts.

### Protocol support (KWin 6.6.4, measured)

| Protocol                           | Purpose                   | KWin 6.6.4 |
| ---------------------------------- | ------------------------- | ---------- |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | yes (v5)   |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)   |
| `zwlr_foreign_toplevel_manager_v1` | Focused-app app_id        | yes (v3)   |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**     |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | **no**     |
| `org_kde_kwin_fake_input`          | KWin-native emulation     | **no**     |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**     |

### Setup notes

1. Run `xdg-desktop-portal` and `xdg-desktop-portal-kde` in the session.
2. On the first daemon start, a "Remote Control" prompt appears. **Check
   "Enable keyboard"** before allowing, or modified clicks degrade and
   `neru action feed` is refused. Neru keeps the grant in
   `$XDG_STATE_HOME/neru/remote-desktop.token` (default
   `~/.local/state/neru/`). Delete that file or revoke the permission in System
   Settings to see the prompt again.
3. Bind modes as in [Global hotkeys on Wayland](#global-hotkeys-on-wayland).

### Screen-sharing consent

`hints.strategy = "vision"` or `"contour"` reads the screen through the
ScreenCast portal. This second grant appears as a source picker the first time
a hint activation needs a frame. Pick every screen Neru may read, because a
region on an unshared screen fails. Neru asks for monitors only, without the
pointer.

Neru keeps the grant in `$XDG_STATE_HOME/neru/screen-cast.token`. Delete that
file or revoke it in **System Settings > Apps & Window Management > Application
Permissions** to see the picker again. A capture never raises the dialog by
itself, and Neru opens the PipeWire stream only for the length of a capture.

### Known issues

- **Hints coverage** depends on each app exposing an AT-SPI tree. Grid and
  scroll work without AT-SPI.

### Troubleshooting

**"could not establish a libei input session via the RemoteDesktop portal"**

Approve the consent dialog before the connect times out. If you denied it,
revoke and re-grant it in System Settings, and confirm the portal services run.
After a revoke, Neru drops its stale token and prompts again on that start.

**"key feeding unavailable: the RemoteDesktop portal session did not grant a keyboard device"**

The grant lacks the keyboard. Clear it, restart the daemon, and **check
"Enable keyboard"** in the prompt:

- **Plasma 6.5+**: **System Settings > Applications > Remote Desktop**, remove
  any saved `neru` permission.
- **Plasma 6.3+ (CLI)**: `flatpak permission-remove kde-authorized remote-desktop ""`
  clears KDE's portal permissions for host applications, flatpak or not.

**Verifying injected input.** Run
`qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole`. Its Input Events tab
shows Neru's events with an "Unknown" device, which is libei.

## COSMIC (Wayland)

**Backend:** `wayland-cosmic`. **Status:** Supported (cosmic-comp). Pointer
input needs xdg-desktop-portal-cosmic 1.7 or later.

Overlays, screen geometry and sticky modifiers use the shared Wayland client.
The focused app and window geometry come from `ext_foreign_toplevel_list_v1`
and `zcosmic_toplevel_info_v1`. Pointer input and screen capture use the
portals, with the same consent prompts as
[KDE](#screen-sharing-consent).

### Protocol support (cosmic-comp 1.6.0, measured)

| Protocol                           | Purpose                   | cosmic-comp |
| ---------------------------------- | ------------------------- | ----------- |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | yes (v5)    |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)    |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | yes         |
| `ext_foreign_toplevel_list_v1`     | Focused-app app_id, title | yes (v1)    |
| `zcosmic_toplevel_info_v1`         | Activated state, geometry | yes (v3)    |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**      |
| `zwlr_foreign_toplevel_manager_v1` | (wlr toplevel manager)    | **no**      |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**      |

### Known issues

- **Portal version.** Fedora 44 ships `xdg-desktop-portal-cosmic` 1.6, which
  lacks RemoteDesktop. The daemon starts and captures keys, but every pointer
  action fails and names the missing portal. Upgrade to 1.7 or later.

## wlroots compositors

**Backend:** `wayland-wlroots`. **Status:** Supported: Sway, Hyprland, niri,
River, Wayfire, labwc.

Neru also takes this backend when `XDG_CURRENT_DESKTOP` contains `:wlroots` or
is unset, which covers dwl, cage, SwayFX and scroll. Such a compositor works if
it has the protocols under
[Checking compositor protocols](#checking-compositor-protocols).

- **Cursor position.** Neru caches the pointer position and sets a known one at
  startup with a brief pointer wiggle. On Hyprland it reads `hyprctl` instead.
- **Window origins.** niri, Sway and Hyprland report focused-window geometry.
  On River, Wayfire and labwc hints stay window-relative. On niri, tiled
  windows report no position
  ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are
  misaligned. See
  [Platform support](../reference/platform-support.md#accessibility-and-hints).

### Known issues

- **Device permissions.** Without `/dev/input` and `/dev/uinput` access,
  `[hotkeys]` do not fire and scrolling uses the virtual pointer, which
  Chromium and Electron apps on Hyprland ignore. See
  [Install-time environment adjustments](./linux.md#install-time-environment-adjustments).
- **Modified scroll on Hyprland** moves in whole notches, because Neru sends it
  on the uinput wheel.

## X11 sessions

**Backend:** `x11`. **Status:** Supported: XOrg, i3, GNOME on X11, and other
X11 window managers.

`[hotkeys]` work with no compositor binding or device permission. Modifier
passthrough is not available, and smooth scroll moves in whole notches.

## GNOME (Wayland)

**Backend:** `wayland-gnome`. **Status:** Supported with Xwayland and the Neru
GNOME Shell extension (GNOME Shell 50 / Mutter, measured).

Pointer input and screen capture use the portals, with the same consents as
[KDE](#screen-sharing-consent). GNOME grants the keyboard with the pointer, so
`feed` works from the first grant. Keyboard capture and hotkeys need the evdev
proxy, with no compositor fallback. The overlay draws on Xwayland, so the
daemon refuses to start without `DISPLAY`.

### The extension

The Neru extension `neru@y3owk1n.github.io` reports the focused window's app
id, title and frame. Per-app config, hints in native Wayland apps and
`FocusedWindowBounds` depend on it. It reads only
`global.display.focus_window`, and neither logs nor stores anything.

The daemon installs it on first start into
`$XDG_DATA_HOME/gnome-shell/extensions/neru@y3owk1n.github.io/`, enables it,
and warns once. **Log out and back in** afterwards. Until then modes work, but
per-app config does not apply, native-app hints stay window-relative, and
`neru doctor` reports `app_watcher` as a stub. Neru rewrites the files only when
a release changes the extension, and never re-enables one you disabled.

### Protocol support (Mutter 50.4, measured)

| Protocol                           | Purpose                   | Mutter 50.4 |
| ---------------------------------- | ------------------------- | ----------- |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)    |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | **no**      |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**      |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | **no**      |
| `zwlr_foreign_toplevel_manager_v1` | Focused-app app_id        | **no**      |
| `ext_foreign_toplevel_list_v1`     | Focused-app app_id        | **no**      |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**      |

### Known issues

- **HiDPI needs Mutter's default Xwayland scaling.** With the
  `xwayland-native-scaling` experimental feature on, the overlay lands in the
  wrong place on scaled monitors. The daemon warns at startup when `Xft.dpi`
  shows this.
- **Budgie** identifies as GNOME and takes this backend untested. Its shell
  cannot load the extension. Cinnamon and Pantheon resolve to `wayland-other`
  and are refused.

## Global hotkeys on Wayland

Neru ships no default global hotkeys on Linux, see
[Binding your first hotkeys](./getting-started.md#binding-your-first-hotkeys).
X11 is unaffected. On Wayland, bind modes one of two ways:

1. **Neru's `[hotkeys]` config** through the evdev keyboard proxy. This needs
   the `input` group, see
   [Wayland keyboard capture permissions](./linux.md#wayland-keyboard-capture-permissions).
   Neru consumes a matched chord before the focused app sees it. Without a
   writable `/dev/uinput`, the app receives it too.
2. **Compositor keybindings** that run `neru hints`, `neru grid` and so on.
   This needs no permissions, but a compositor binding cannot fire while a mode
   is open.

With path 1, bindings keep working inside a mode, and a `Super` chord does not
also fire the desktop's Super-alone shortcut. On startup the daemon logs
`Wayland global hotkeys enabled via evdev; config keybindings are active`, or a
warning naming the `input` group and the compositor fallback.
[ADR 0014](../adr/0014-the-wayland-keyboard-is-a-proxy.md) records why the
proxy holds the keyboards.

### Compositor bindings

```sway
# Sway: ~/.config/sway/config
bindsym $mod+Shift+h exec neru hints
bindsym $mod+Shift+g exec neru grid
bindsym $mod+Shift+s exec neru scroll
```

```hyprlang
# Hyprland: ~/.config/hypr/hyprland.conf
bind = $mod SHIFT, H, exec, neru hints
bind = $mod SHIFT, G, exec, neru grid
bind = $mod SHIFT, S, exec, neru scroll
```

```kdl
// niri: ~/.config/niri/config.kdl
binds {
    Mod+Shift+H { spawn-sh "neru hints"; }
    Mod+Shift+G { spawn-sh "neru grid"; }
    Mod+Shift+S { spawn-sh "neru scroll"; }
    Mod+Shift+R { spawn-sh "neru recursive_grid"; }
}
```

KDE Plasma: **System Settings > Shortcuts > Custom Shortcuts**, with the
absolute binary path, such as `/home/<you>/.local/bin/neru hints`.

## Checking compositor protocols

Run inside the graphical session:

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|zwlr_screencopy|fake_input|xdg_output'
```

The wlroots backend needs `zwlr_layer_shell_v1` and `zwlr_virtual_pointer_v1`.
A compositor Neru does not recognize resolves to `wayland-other` and is
refused. To add one, see
[organize by mechanism, not by desktop](../contributing/porting.md#organize-by-mechanism-not-by-desktop).
