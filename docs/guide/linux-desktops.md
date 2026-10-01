# Linux desktops

What each Linux desktop needs on top of [Linux setup](linux.md), and the
issues specific to it. Find your desktop below.

## KDE Plasma (Wayland)

Supported on Plasma 6.

### Setup

1. Make sure `xdg-desktop-portal` and `xdg-desktop-portal-kde` run in your
   session.
2. Start Neru. A "Remote Control" prompt appears.
3. Tick **Enable keyboard**, then allow.
4. Bind your modes, see [Global hotkeys on Wayland](#global-hotkeys-on-wayland).

> [!WARNING]
> If you allow without ticking **Enable keyboard**, modified clicks degrade
> and `neru action feed` is refused. See
> [the fix](#key-feeding-unavailable-the-remotedesktop-portal-session-did-not-grant-a-keyboard-device).

Neru keeps the grant in `remote-desktop.token` under `$XDG_STATE_HOME/neru/`,
by default `~/.local/state/neru/`. Delete
that file, or revoke the permission in System Settings, to see the prompt
again.

### Screen-sharing consent

The `vision` and `contour` hint strategies read the screen. The first time
one needs a frame, KDE shows a source picker. Pick every screen Neru may read,
because hints on an unshared screen fail. Neru asks for monitors only, not
the pointer, and a capture never raises the picker by itself.

Neru keeps this grant in `screen-cast.token` in the same directory. Delete that
file, or revoke it in **System Settings > Apps & Window Management >
Application Permissions**, to see the picker again.

### "could not establish a libei input session via the RemoteDesktop portal"

Approve the consent prompt before it times out. If you denied it, revoke and
re-grant it in System Settings, and check the portal services run. After a
revoke, Neru drops its stale token and prompts again.

### "key feeding unavailable: the RemoteDesktop portal session did not grant a keyboard device"

The grant is missing the keyboard. Clear it, restart the daemon, and tick
**Enable keyboard** in the new prompt. To clear it:

- **Plasma 6.5 or later:** in **System Settings > Applications > Remote
  Desktop**, remove any saved `neru` permission.
- **Plasma 6.3 or later, from a terminal:**
  `flatpak permission-remove kde-authorized remote-desktop ""`. This clears
  KDE's portal permissions for every host app, flatpak or not.

## COSMIC (Wayland)

Supported on cosmic-comp. Pointer input needs `xdg-desktop-portal-cosmic` 1.7
or later.

### Setup

1. Check `xdg-desktop-portal-cosmic` is 1.7 or later. Fedora 44 ships 1.6,
   where the daemon starts but every pointer action fails and names the
   missing portal.
2. Start Neru and approve the prompts, as on
   [KDE](#kde-plasma-wayland), including
   [screen-sharing consent](#screen-sharing-consent).
3. Bind your modes, see [Global hotkeys on Wayland](#global-hotkeys-on-wayland).

## wlroots compositors

Supported on Sway, Hyprland, niri, River, Wayfire and labwc. Other wlroots
compositors, such as dwl, cage, SwayFX and scroll, work if they have the
protocols under [Checking compositor protocols](#checking-compositor-protocols).
Neru treats a compositor as wlroots when `XDG_CURRENT_DESKTOP` contains
`:wlroots` or is unset.

### Setup

Follow [Linux setup](linux.md#setup-steps), including the `input` group and
the `/dev/uinput` rule. What breaks without them is in
[Running without the permissions](linux.md#running-without-the-permissions).

### Hints in native apps are offset

Hints read window positions from the compositor.

- **Sway, Hyprland, niri:** the compositor reports positions, so hints line
  up.
- **niri tiled windows:** niri reports no position for them
  ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are
  misaligned. Floating and fullscreen windows are fine.
- **River, Wayfire, labwc:** the compositor reports no positions, so Neru
  places hints in native apps relative to the window, not the screen.

The same positions place `--capture-scope window`, `vision` and `contour` on
these compositors, and on River and Wayfire those report that they are not
supported. Grid modes with the default screen scope are not affected.

## X11 sessions

Supported on XOrg, i3, GNOME on X11 and other X11 window managers.
`[hotkeys]` work with no compositor binding or device permission. Its limits,
such as no modifier passthrough, are in the
[capability matrix](../reference/platform-support.md#capability-matrix).

## GNOME (Wayland)

Supported with Xwayland and the Neru GNOME Shell extension, on GNOME Shell 50.

### Setup

1. Make sure Xwayland is enabled. The daemon refuses to start without
   `DISPLAY`, because its overlay is an Xwayland window.
2. Follow [Linux setup](linux.md#setup-steps). On GNOME the `input` group is
   required, since keyboard capture has no fallback.
3. Start Neru. It installs and enables its extension, and warns once.
4. **Log out and back in**, so GNOME Shell loads the extension.
5. Approve the input and screen-sharing prompts, as on
   [KDE](#screen-sharing-consent). GNOME grants the keyboard with the pointer,
   so there is no box to tick.

Until you log back in, modes work, but per-app config does not apply, hints in
native apps are offset, and `neru doctor` reports `app_watcher` as a stub.

### The extension

The extension `neru@y3owk1n.github.io` tells Neru which window has focus, its
app id, title and position. It reads only the focused window, and neither logs
nor stores anything. Neru installs it in
`$XDG_DATA_HOME/gnome-shell/extensions/neru@y3owk1n.github.io/`. Neru rewrites
it only when a release changes it, and never re-enables it if you disable it.

### The overlay is offset on a scaled monitor

Mutter's experimental `xwayland-native-scaling` feature moves the overlay on
scaled monitors. Turn it off. The daemon warns at startup when it sees this.

### Budgie, Cinnamon and Pantheon

Budgie identifies as GNOME and takes this backend untested, and its shell
cannot load the extension. Cinnamon and Pantheon on Wayland are not
supported, and the daemon refuses to start.

## Global hotkeys on Wayland

Bind modes in one of two ways. X11 needs neither, since its `[hotkeys]` work
with no permissions.

| | Neru's `[hotkeys]` | Compositor keybindings |
| --- | --- | --- |
| Needs | The `input` group, see [Linux setup](linux.md#setup-steps) | Nothing |
| The focused app sees the chord | No, if `/dev/uinput` is writable | No |
| Works while a mode is open | Yes | No |

With `[hotkeys]`, the daemon logs
`Wayland global hotkeys enabled via evdev; config keybindings are active` at
startup, or a warning naming the `input` group.

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

On KDE Plasma, add them in **System Settings > Shortcuts > Custom Shortcuts**
with the binary's full path, such as `/home/<you>/.local/bin/neru hints`.

## Checking compositor protocols

Run this inside the graphical session:

```bash
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|zwlr_screencopy|fake_input|xdg_output'
```

A wlroots compositor needs `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. Neru refuses to start on a compositor it does not
recognize. To help add support for one, see the
[porting guide](../contributing/porting.md#organize-by-mechanism-not-by-desktop).
Which protocols each supported desktop has is in
[platform internals](../contributing/platform-internals.md#wayland-desktops).
