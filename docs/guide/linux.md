# Linux setup

What a Linux machine needs before Neru runs. X11 needs only the libraries.
Wayland also needs two device permissions. Per-desktop notes are in
[Linux desktops](linux-desktops.md).

## Supported desktops

Neru picks a backend once at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`. `neru doctor` shows it as `display_server`.

| Desktop                                                          | Backend           | Status                                                     |
| ---------------------------------------------------------------- | ----------------- | ---------------------------------------------------------- |
| Sway, Hyprland, niri, River, Wayfire, labwc                      | `wayland-wlroots` | Supported                                                  |
| Other wlroots compositors (dwl, cage, SwayFX, scroll, ...)       | `wayland-wlroots` | [If it has the protocols](linux-desktops.md#wlroots-compositors) |
| KDE Plasma (Wayland)                                             | `wayland-kde`     | [Supported](linux-desktops.md#kde-plasma-wayland)          |
| COSMIC (Wayland)                                                 | `wayland-cosmic`  | [Supported](linux-desktops.md#cosmic-wayland)              |
| GNOME (Wayland), Budgie                                          | `wayland-gnome`   | [Supported with Xwayland and an extension](linux-desktops.md#gnome-wayland) |
| X11: XOrg, i3, GNOME on X11, other window managers               | `x11`             | Supported                                                  |
| Cinnamon or Pantheon on Wayland, Weston, Mir shells, any other   | `wayland-other`   | Not supported, the daemon refuses to start                 |

## Setup steps

1. Install the [runtime libraries](#runtime-libraries) for your distribution.
   On X11, you are done.
2. **Wayland:** join the `input` group, so Neru can read the keyboard and run
   its own `[hotkeys]`:

   ```bash
   sudo usermod -aG input "$USER"
   ```

3. **Wayland:** make `/dev/uinput` writable, so Neru can re-emit keys and
   scroll:

   ```bash
   echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-neru-uinput.rules
   sudo udevadm control --reload && sudo udevadm trigger
   ```

4. **Log out and back in**, or reboot. Group membership does not change
   until you do. Then check that `id` lists `input`, and that
   `ls -l /dev/uinput` shows group `input` and mode `crw-rw----`. If
   `/dev/uinput` is missing, run `sudo modprobe uinput`.
5. Run `neru doctor`. Every row should be healthy.

The install script offers step 2 for you. If you skip steps 2 and 3, Neru
still runs with less, see [Running without the permissions](#running-without-the-permissions).

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

On KDE, approve [screen sharing](linux-desktops.md#screen-sharing-consent)
once, the first time a hint strategy needs a screen capture.

## Wayland keyboard capture permissions

On Wayland, Neru grabs every keyboard with `EVIOCGRAB` and re-emits keys
through its own uinput keyboard, `neru-keyboard-proxy`. A mode captures keys
the instant it opens, a hotkey chord never reaches the focused app, and keys
pass straight through between modes. This needs [steps 2 and 3](#setup-steps).
The `input` group can read every keyboard on the system, so use a tighter
distro `udev` or ACL setup if that is too broad.

On success Neru logs `Evdev keyboard proxy running` at startup and
`Using Wayland evdev keyboard capture` when a mode opens.

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

Neru scrolls through a virtual wheel on `/dev/uinput`, which most distros make
root-only. The udev rule in [step 3](#setup-steps) opens it to the `input`
group. The same device lets the keyboard proxy re-emit keys and gives
`neru key` its fast path. Restart the daemon after adding the rule.

## Running without the permissions

Without the `input` group, Neru's `[hotkeys]` do nothing on Wayland. Bind
`neru hints` and the other modes in your compositor instead, see
[Global hotkeys on Wayland](linux-desktops.md#global-hotkeys-on-wayland).

Without a writable `/dev/uinput`:

- Modes capture keys through the overlay's keyboard focus, so a hotkey chord
  also reaches the focused app.
- Scrolling goes through the compositor instead. Chromium and Electron apps on
  Hyprland ignore it. `neru doctor` reports this under `scroll`.
- Modified clicks may degrade, and `general.passthrough_unbounded_keys` does
  nothing.

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

- **Other init systems.** On runit, OpenRC or s6, `neru services` reports that
  it is not supported. Run `neru launch` from your session instead.
- **A unit Neru did not write.** Neru manages only units starting with
  ``# Installed by `neru services install` ``. `install` and `uninstall` refuse
  any other `neru.service`, such as one from Nix or your distribution. To
  switch, remove yours, for example with
  `systemctl --user disable --now neru.service`, then run `neru services install`.
- **Relocated `$XDG_CONFIG_HOME`.** Set it in your session, not only a shell
  rc, because the user manager fixes its unit search path at login.
  `neru services install` refuses a directory outside that path.

## Known limitations

Hint coverage, alerts and the other limits Linux shares with Windows are in
[Platform support](../reference/platform-support.md#notes). Specific to Linux:

- **Monitor hotplug** is tracked live. On Wayland, a resolution or scale change
  to an existing monitor needs a [daemon restart](troubleshooting.md#restart-the-daemon).

## Troubleshooting

General problems are in [Troubleshooting](troubleshooting.md).

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
[Supported backends](#supported-desktops) and
[Checking compositor protocols](linux-desktops.md#checking-compositor-protocols).

### "failed to connect to Wayland compositor"

Check `echo $WAYLAND_DISPLAY` and that `wayland-info` (package `wayland-utils`)
answers.

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Add your user to the `input` group, log out and back in, and confirm with `id`.
See [Wayland keyboard capture permissions](#wayland-keyboard-capture-permissions).

### "Keyboard capture unavailable: /dev/uinput is not writable"

Modes fall back to the overlay's keyboard focus. Add the udev rule from
[step 3](#setup-steps) and restart the daemon.

### Sticky modifier indicator shows `[][][][]`

The font lacks the modifier glyphs. Set `[sticky_modifiers.ui].font_family` to
a family that `fc-list : family` lists and that renders `❖⇧⌥⌃`. An unknown
family falls back to DejaVu Sans.
