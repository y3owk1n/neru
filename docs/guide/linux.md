# Linux setup

What a Linux host needs before Neru runs. Per-desktop notes are in
[Linux desktops](./linux-desktops.md), and installing Neru, including Nix, is in
[Installation](./installation.md).

## Supported backends

Neru picks a backend once at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`.

| Compositor / session                                                                                               | Backend           | Status                                                                                                                 |
| ------------------------------------------------------------------------------------------------------------------ | ----------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Sway, Hyprland, niri, River, Wayfire, labwc                                                                        | `wayland-wlroots` | Supported                                                                                                              |
| Any compositor tagging `XDG_CURRENT_DESKTOP` with `:wlroots`, or leaving it unset (dwl, cage, SwayFX, scroll, ...) | `wayland-wlroots` | Supported when it implements the wlroots protocols, see [wlroots compositors](./linux-desktops.md#wlroots-compositors) |
| KDE Plasma (Wayland)                                                                                               | `wayland-kde`     | Supported, see [KDE Plasma](./linux-desktops.md#kde-plasma-wayland)                                                    |
| COSMIC (Wayland)                                                                                                   | `wayland-cosmic`  | Supported, see [COSMIC](./linux-desktops.md#cosmic-wayland)                                                            |
| X11 / XOrg, i3, GNOME on X11                                                                                       | `x11`             | Supported                                                                                                              |
| GNOME (Wayland), and Mutter-based desktops such as Budgie                                                          | `wayland-gnome`   | Supported with Xwayland and the Neru GNOME Shell extension, see [GNOME](./linux-desktops.md#gnome-wayland)             |
| Cinnamon and Pantheon on Wayland, Weston, Mir shells (miracle-wm), any other compositor                            | `wayland-other`   | Not supported, the daemon refuses to start                                                                             |

## Install-time environment adjustments

| #   | Adjustment                                                  | Why                                                                                                  | Backends  | Persists?               |
| --- | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | --------- | ----------------------- |
| 1   | Install the [runtime libraries](#runtime-libraries)         | The Neru binary loads them at startup                                                                | All Linux | Yes                     |
| 2   | Add user to `input` group: `sudo usermod -aG input "$USER"` | Read `/dev/input`: keyboard capture and Neru's own `[hotkeys]` on Wayland                            | Wayland   | Yes (re-login required) |
| 3   | Make `/dev/uinput` writable (udev rule below)               | Write `/dev/uinput`: the keyboard proxy that makes capture instant, the scroll wheel, and `neru key` | Wayland   | Yes (udev rule)         |
| 4   | Bind `neru <mode>` in compositor keybindings                | Only if you skip item 2                                                                              | Wayland   | Yes (user config)       |

- X11 needs only item 1. Neru's `[hotkeys]` register through `XGrabKey`.
- Item 2 takes effect after a full logout and login, or a reboot.
- Without item 3, modes capture keys through the overlay's keyboard focus, so a
  hotkey chord also reaches the focused app. Scrolling falls back to the
  compositor seat, which is the virtual pointer on wlroots and libei on KDE.
  Chromium and Electron apps on Hyprland ignore the virtual pointer.
  `neru doctor` reports the downgrade under `scroll`, and the daemon warns at
  the first fallback scroll. Modified clicks may degrade, and
  `general.passthrough_unbounded_keys` has no injection path.
- Item 4 is a fallback. See
  [Global hotkeys on Wayland](./linux-desktops.md#global-hotkeys-on-wayland).

## Runtime libraries

The binary links these dynamically, and the daemon stops before any Neru code
runs if one is missing. The install script lists each missing library first,
and on apt, dnf and pacman prints the command that finds its package.

- **tesseract** reads screen text for `hints.strategy = "vision"`, and is
  required whatever the strategy is.
- **tesseract English language data** is a separate package. Without it the
  `vision` strategy reports that `eng.traineddata` is missing. Set
  `TESSDATA_PREFIX` to use language data from elsewhere, such as `tessdata_fast`.
- **pipewire** carries screen capture on KDE, but a missing
  `libpipewire-0.3.so` stops the daemon on every desktop.
- **libei** injects input on KDE, COSMIC and GNOME.
- **cairo**, **fontconfig**, **xkbcommon**, the Wayland client library and
  `libX11`, `libXtst`, `libXrandr`, `libXrender`, `libXext`, `libXfixes` draw
  overlays and talk to the display server.
- **DejaVu fonts** are the overlay fonts when `font_family` is unset, and
  carry the sticky modifier symbols `❖⇧⌥⌃`.

### Debian / Ubuntu

```bash
sudo apt-get install -y tesseract-ocr tesseract-ocr-eng fonts-dejavu-core
```

Find the package for any other missing library with
`apt-file search <library>` after `sudo apt-file update`.

### Fedora

The library is `tesseract-libs`, not `tesseract`, and it needs
[a compatibility link](#error-while-loading-shared-libraries-libtesseractso5).

```bash
sudo dnf install -y \
  tesseract-libs tesseract-langpack-eng libei pipewire-libs cairo libxkbcommon \
  libX11 libXtst libXrandr libXrender libXext libXfixes \
  dejavu-sans-fonts dejavu-serif-fonts dejavu-sans-mono-fonts
```

### Arch Linux

```bash
sudo pacman -S --needed \
  cairo wayland libx11 libxtst libxrandr libxrender libxext libxfixes \
  libxkbcommon libei fontconfig tesseract tesseract-data-eng libpipewire ttf-dejavu
```

Building from source needs the development packages too, see the
[development guide](../contributing/development.md). On KDE, approve
[screen sharing](./linux-desktops.md#screen-sharing-consent) once, the first
time a vision-strategy hint activation needs it.

## Wayland keyboard capture permissions

On Wayland, Neru grabs every keyboard with `EVIOCGRAB` and re-emits keys
through its own uinput keyboard, `neru-keyboard-proxy`. A mode captures keys
the instant it opens, a hotkey chord never reaches the focused app, and keys
pass straight through between modes. This needs items 2 and 3 from the table
above. After item 2, log out and back in, then confirm `id` lists `input`. The
group allows reading
system-wide keyboard events, so use a tighter distro `udev` or ACL setup if
that is too broad.

On success Neru logs `Evdev keyboard proxy running` at startup and
`Using Wayland evdev keyboard capture` when a mode opens. Without
`/dev/uinput`, hotkeys work but modes use overlay-focused capture, where
modified clicks may degrade. Without `/dev/input` there is no proxy.

### Key remappers (kanata, keyd)

A remapper's auto-detect grabs Neru's devices too, so exclude them. For kanata:

```lisp
(defcfg
  linux-dev-names-exclude ("neru-keyboard-proxy" "neru-pointer-proxy" "neru-keyboard")
)
```

Or list your physical keyboards in `linux-dev-names-include`. For keyd, exclude
`-1234:567a`, `-1234:567b` and `-1234:5679` under `[ids]`.

- If a remapper grabs a Neru device anyway, Neru releases every keyboard and
  logs why. Mode key capture stays off until the daemon restarts.
- With kanata, start order does not matter. Neru leaves a keyboard the
  remapper holds to it and captures its output keyboard. A remapper that starts
  later is handed the keyboards, and Neru takes back any it has not claimed
  after three seconds.
- keyd and `kanata --nodelay` grab at once, so start them before Neru.
- Write Neru hotkeys as the keys the remapper emits.
- A remapper output device that also moves the pointer is re-emitted through
  `neru-pointer-proxy`.
- Neru takes back the keyboards if the remapper quits.

### Other effects of holding the keyboards

- Neru never grabs a keyboard that reports touch or pen position axes, such as
  a built-in trackpad on the same node. A volume knob does not count.
- Per-device compositor settings, such as a Sway or Hyprland `input` block or a
  KDE per-device layout, apply to `neru-keyboard-proxy` while the daemon runs.

## Wayland scroll injection permissions

Neru scrolls through a virtual wheel on `/dev/uinput`, which most distros ship
root-only. The same rule lets the keyboard proxy re-emit keys and gives
`neru key` its fast path.

```bash
echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-neru-uinput.rules
sudo udevadm control --reload && sudo udevadm trigger
```

Confirm with `ls -l /dev/uinput` that the group is `input` and the mode is
`0660`, then restart the daemon. If the node is missing, run
`sudo modprobe uinput`.

## Systemd user service

`neru services install` writes `neru.service` to
`$XDG_CONFIG_HOME/systemd/user` (default `~/.config/systemd/user`), enables it
at login, and starts it. `ExecStart` is the resolved path of the binary you ran,
so reinstall after moving it. Other subcommands are in the
[CLI reference](../reference/cli.md#neru-services).

The unit is anchored on `graphical-session.target` and needs your session's
display variables. Desktop environments such as KDE Plasma and wrappers such as
`uwsm` export them. A bare compositor started from a TTY needs this first in
its config:

```sway
# ~/.config/sway/config
exec systemctl --user import-environment \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec dbus-update-activation-environment --systemd \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec systemctl --user start graphical-session.target
```

Hyprland, niri and River use their own socket variable, such as
`HYPRLAND_INSTANCE_SIGNATURE` or `NIRI_SOCKET`, in place of `SWAYSOCK`. If the
service does not start, check `systemctl --user status neru.service` and
`systemctl --user is-active graphical-session.target`. If the target stays
inactive, run `neru launch` from your compositor's autostart instead.

- **Other init systems.** On runit, OpenRC or s6 every `neru services`
  subcommand reports `ERR_NOT_SUPPORTED`. Run `neru launch` from your session.
- **A unit Neru did not write.** Neru manages only units starting with
  ``# Installed by `neru services install` ``. `install` and `uninstall` refuse
  any other `neru.service`, such as one from Nix or your distribution. To
  switch, remove yours, for example with
  `systemctl --user disable --now neru.service`, then run `neru services install`.
- **Relocated `$XDG_CONFIG_HOME`.** Set it in your session, not only a shell
  rc, because the user manager fixes its unit search path at login.
  `neru services install` refuses a directory outside that path.

## Known limitations

- **Hints need AT-SPI**, and coverage varies by app. Chromium and Electron
  apps need `--force-renderer-accessibility`. The `vision` and `contour`
  strategies work from a screen capture instead. Grid and scroll do not need
  AT-SPI.
- **Alerts are not modal.** They go over `org.freedesktop.Notifications`, so a
  notification daemon such as mako or dunst must run or be D-Bus activatable.
  Without one the two startup alerts go to stderr.
- **Monitor hotplug** is tracked live. On Wayland, a resolution or scale change
  to an existing monitor needs a [daemon restart](./troubleshooting.md#restart-the-daemon).

## Troubleshooting

General problems are in [Troubleshooting](./troubleshooting.md).

### "error while loading shared libraries: libtesseract.so.5"

Release binaries expect `libtesseract.so.5`, and Fedora ships
`libtesseract.so.5.5`. The installer detects this and points here. Add a link,
using the name `ls` prints:

```bash
sudo dnf install -y tesseract-libs tesseract-langpack-eng
ls /usr/lib64/libtesseract.so.5.*
sudo ln -s libtesseract.so.5.5 /usr/lib64/libtesseract.so.5
```

Debian, Ubuntu, Arch and builds from source do not need this.

### "WAYLAND_DISPLAY is not set"

Neru runs under X11 or a TTY, and uses the X11 backend when `DISPLAY` is set.
From the systemd service, see [Systemd user service](#systemd-user-service).

### "neru does not recognize this Wayland compositor"

`XDG_CURRENT_DESKTOP` resolved to `wayland-other`. Check it against
[Supported backends](#supported-backends) and
[Checking compositor protocols](./linux-desktops.md#checking-compositor-protocols).

### "failed to connect to Wayland compositor"

Check `echo $WAYLAND_DISPLAY` and that `wayland-info` (package `wayland-utils`)
answers.

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Add your user to the `input` group, log out and back in, and confirm with `id`.
See [Wayland keyboard capture permissions](#wayland-keyboard-capture-permissions).

### "Keyboard capture unavailable: /dev/uinput is not writable"

Modes fall back to the overlay's keyboard focus. Add the udev rule from
[Wayland scroll injection permissions](#wayland-scroll-injection-permissions)
and restart the daemon.

### Sticky modifier indicator shows `[][][][]`

The font lacks the modifier glyphs. Set `[sticky_modifiers.ui].font_family` to
a family that `fc-list : family` lists and that renders `❖⇧⌥⌃`. An unknown
family falls back to DejaVu Sans.
