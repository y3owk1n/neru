# Linux setup

What a Linux host needs before Neru runs: the libraries it loads, the device
permissions it uses on Wayland, the systemd user service, and fixes for
Linux-specific problems. Desktop notes for KDE, COSMIC, wlroots compositors, X11
and GNOME, and compositor hotkey bindings, are in
[Linux desktops](./linux-desktops.md). Installing Neru itself is covered in
[Installation](./installation.md).

## Supported backends

Neru picks a backend once at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`.

| Compositor / session                                                  | Backend           | Status                                                                                  |
| --------------------------------------------------------------------- | ----------------- | --------------------------------------------------------------------------------------- |
| Sway, Hyprland, niri, River, Wayfire, labwc                            | `wayland-wlroots` | Supported                                                                               |
| Any compositor tagging `XDG_CURRENT_DESKTOP` with `:wlroots`, or leaving it unset (dwl, cage, SwayFX, scroll, ...) | `wayland-wlroots` | Supported when it implements the wlroots protocols, see [wlroots compositors](./linux-desktops.md#wlroots-compositors) |
| KDE Plasma (Wayland)                                                  | `wayland-kde`     | Supported, see [KDE Plasma](./linux-desktops.md#kde-plasma-wayland)                     |
| COSMIC (Wayland)                                                      | `wayland-cosmic`  | Supported, see [COSMIC](./linux-desktops.md#cosmic-wayland)                             |
| X11 / XOrg, i3, GNOME on X11                                          | `x11`             | Supported                                                                               |
| GNOME (Wayland), and Mutter-based desktops such as Budgie             | `wayland-gnome`   | Supported with Xwayland and the Neru GNOME Shell extension, see [GNOME](./linux-desktops.md#gnome-wayland) |
| Cinnamon and Pantheon on Wayland, Weston, Mir shells (miracle-wm), any other compositor | `wayland-other`   | Not supported, the daemon refuses to start                                              |

## Install-time environment adjustments

Host changes required before Neru runs correctly:

| #   | Adjustment                                                  | Why                                                                                                   | Backends  | Persists?               |
| --- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | --------- | ----------------------- |
| 1   | Install the [runtime libraries](#runtime-libraries)         | The Neru binary loads them at startup                                                                 | All Linux | Yes                     |
| 2   | Add user to `input` group: `sudo usermod -aG input "$USER"` | Read `/dev/input`: keyboard capture and Neru's own `[hotkeys]` on Wayland                             | Wayland   | Yes (re-login required) |
| 3   | Make `/dev/uinput` writable (udev rule below)               | Write `/dev/uinput`: the keyboard proxy that makes capture instant, the scroll wheel, and `neru key`  | Wayland   | Yes (udev rule)         |
| 4   | Bind `neru <mode>` in compositor keybindings                | Only if you skip item 2                                                                               | Wayland   | Yes (user config)       |

Notes:

- X11 only needs item 1. Global hotkeys work through `XGrabKey` from Neru's
  config.
- Item 2 takes effect after a full logout and login, or a reboot.
- Without item 3 Neru still runs, with two downgrades. Modes capture keys
  through the overlay's keyboard focus, so a hotkey chord also reaches the
  focused app. Scrolling falls back from the uinput wheel to the compositor
  seat. On wlroots that is the virtual pointer, which Chromium and Electron
  apps on Hyprland ignore. On KDE it is libei through the portal session. `neru doctor`
  reports the scroll downgrade under `scroll`, and the daemon warns once at the
  first scroll that falls back.
- Item 4 is a fallback, not a requirement. With item 2 in place Neru's own
  `[hotkeys]` config works on Wayland. See
  [Global hotkeys on Wayland](./linux-desktops.md#global-hotkeys-on-wayland).

## Runtime libraries

The Linux binary links these libraries dynamically. If one is missing, the
daemon stops before any Neru code runs. The install script checks this before
it copies anything, lists each missing library, and on apt, dnf and pacman
systems prints the command that finds the package providing it.

- **tesseract** recognizes on-screen text for `hints.strategy = "vision"`. It is
  required whatever the strategy is set to.
- **tesseract English language data** is a separate package on every
  distribution. Without it Neru starts, but the vision strategy reports that
  `eng.traineddata` is missing. To use language data from elsewhere, such
  as a `tessdata_fast` checkout, point `TESSDATA_PREFIX` at it.
- **pipewire** carries screen pixels on KDE Plasma, where capture goes through
  the desktop portal's ScreenCast session. A missing `libpipewire-0.3.so` stops
  the daemon on every desktop, not only on KDE.
- **libei** injects pointer and keyboard input on KDE, COSMIC and GNOME.
- **cairo**, **fontconfig**, **xkbcommon**, the Wayland client library and the
  X11 libraries (`libX11`, `libXtst`, `libXrandr`, `libXrender`, `libXext`,
  `libXfixes`) draw the overlays and talk to the display server.
- **DejaVu fonts** are the default overlay fonts when `font_family` is unset,
  and carry the sticky modifier symbols `❖⇧⌥⌃`.

### Debian / Ubuntu

```bash
sudo apt-get install -y tesseract-ocr tesseract-ocr-eng fonts-dejavu-core
```

If the binary still names a missing library, find its package with
`apt-file search <library>`. Install `apt-file` and run
`sudo apt-file update` first.

### Fedora

```bash
sudo dnf install -y \
  tesseract-libs tesseract-langpack-eng libei pipewire-libs cairo libxkbcommon \
  libX11 libXtst libXrandr libXrender libXext libXfixes \
  dejavu-sans-fonts dejavu-serif-fonts dejavu-sans-mono-fonts
```

The `tesseract` package is the command-line tool alone. The library is
`tesseract-libs`. Fedora also names that library differently from what the
release binary expects. See
[the libtesseract error](#error-while-loading-shared-libraries-libtesseractso5).

### Arch Linux

```bash
sudo pacman -S --needed \
  cairo wayland libx11 libxtst libxrandr libxrender libxext libxfixes \
  libxkbcommon libei fontconfig tesseract tesseract-data-eng libpipewire ttf-dejavu
```

Building from source needs the development packages as well. See the
[development guide](../contributing/development.md).

On KDE you also approve screen sharing once, the first time a vision-strategy
hint activation needs it. See
[Screen-sharing consent](./linux-desktops.md#screen-sharing-consent).

## Wayland keyboard capture permissions

On Wayland, Neru holds every keyboard with `EVIOCGRAB` for the daemon's lifetime
and re-emits it through a uinput keyboard of its own, `neru-keyboard-proxy`.
The compositor reads that one device, so a mode captures keys the instant it
opens and a hotkey chord never reaches the focused app. Between modes every key
passes straight through. This needs item 2, read access to `/dev/input`, and
item 3, write access to `/dev/uinput`, from the table above.

```bash
sudo usermod -aG input "$USER"
```

Log out and back in, then confirm `id` lists the `input` group.

> [!NOTE]
> Membership in `input` allows reading system-wide keyboard events. Use a
> tighter distro-specific `udev`/ACL setup if the group is too broad for your
> environment.

When capture works, Neru logs `Evdev keyboard proxy running` at startup and
`Using Wayland evdev keyboard capture` when a mode opens. Without `/dev/uinput`
the proxy reads passively. Hotkeys still work, but modes fall back to
overlay-focused capture, where basic navigation works and modified clicks may
degrade. Without `/dev/input` access there is no proxy at all.

### Key remappers (kanata, keyd)

A key remapper grabs its input keyboards the same way, and its device
auto-detect will grab Neru's own devices too, since they look like keyboards
to it. Tell the remapper to leave them alone. For kanata:

```lisp
(defcfg
  linux-dev-names-exclude ("neru-keyboard-proxy" "neru-pointer-proxy" "neru-keyboard")
)
```

or list your physical keyboards in `linux-dev-names-include`, which bypasses
auto-detect. For keyd, exclude Neru's ids under `[ids]`: `-1234:567a`,
`-1234:567b` and `-1234:5679`.

- If a remapper grabs a Neru device anyway, Neru releases every keyboard to
  the compositor and logs why. Your keys and remaps keep working, and mode key
  capture is off until the daemon restarts.
- Start order does not matter with kanata. A keyboard the remapper already
  holds is left to it and its virtual output keyboard is captured instead. A
  remapper that starts while Neru holds the keyboards is handed them as soon as
  its output device appears, and Neru takes back any keyboard it has not
  claimed three seconds later.
- keyd and `kanata --nodelay` grab the instant they start, before Neru can see
  their output device, so start those before Neru.
- Neru's hotkeys match the keys the remapper emits, so write them as it sends
  them.
- A remapper output device that also moves the pointer is re-emitted through a
  second Neru device, `neru-pointer-proxy`.
- Quitting the remapper while Neru runs is fine. Neru takes the keyboards it
  released.

### Other effects of holding the keyboards

- A keyboard that reports touch or pen position axes, such as a built-in
  trackpad on the same node, is never grabbed, so its keys are not captured. A volume knob
  or another non-position axis, common on Bluetooth keyboards, does not count.
- Compositor settings applied per input device, such as an `input` block in
  Sway or Hyprland or a per-device keyboard layout in KDE, apply to
  `neru-keyboard-proxy` while the daemon runs, since that is the keyboard the
  compositor sees.

## Wayland scroll injection permissions

On Wayland, Neru scrolls through a virtual mouse wheel it creates on
`/dev/uinput`, so the events reach every client like a physical wheel. Most
distros ship that node as root-only, and the `input` group does not cover it.
Grant it with a udev rule:

```bash
echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-neru-uinput.rules
sudo udevadm control --reload && sudo udevadm trigger
```

Confirm with `ls -l /dev/uinput` that the group is `input` and the mode is
`0660`, then restart the daemon. If the node is missing, load the module with
`sudo modprobe uinput`.

The same rule lets the keyboard proxy re-emit keys and gives `neru key` its
fast path.

## Nix

The Nix flake, the NixOS module and the home-manager module are covered in
[Installation](./installation.md#nix).

## Systemd user service

`neru services install` writes a systemd user unit, enables it for every
login, and starts it now. The other subcommands are in the
[CLI reference](../reference/cli.md#neru-services).

**What it writes.** `neru.service` under `$XDG_CONFIG_HOME/systemd/user`,
which is `~/.config/systemd/user` by default. `ExecStart` is the resolved path of the
`neru` binary you ran `install` with, so run
`neru services uninstall && neru services install` after moving the binary. The
unit is anchored on `graphical-session.target`.

**Your session has to export itself first.** A systemd user manager starts
before your compositor and inherits nothing from it. Unless the session imports
its own variables, `neru launch` runs with no `WAYLAND_DISPLAY`, `DISPLAY` or
compositor socket and cannot find a display server. Most desktop environments,
such as KDE Plasma, and session wrappers such as `uwsm` do this for you. A bare compositor
started from a TTY does not. Add this to your compositor config before anything
that depends on it:

```sway
# ~/.config/sway/config
exec systemctl --user import-environment \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec dbus-update-activation-environment --systemd \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec systemctl --user start graphical-session.target
```

Hyprland, niri and River take the same three lines with their own socket
variable, such as `HYPRLAND_INSTANCE_SIGNATURE` or `NIRI_SOCKET`, in place of
`SWAYSOCK`.

**If it does not start.** `graphical-session.target` is reached only if
something in your session activates it. The third `exec` above does that for a
bare compositor. Check both:

```bash
systemctl --user status neru.service
systemctl --user is-active graphical-session.target
```

If the target is inactive and you would rather not wire the session up, run
`neru launch` from your compositor's autostart instead.

**Other init systems.** Service management covers systemd only. On a machine
booted by runit, OpenRC or s6 every `neru services` subcommand reports
`ERR_NOT_SUPPORTED`. Run `neru launch` from your session's own supervisor or
autostart.

**A unit you did not get from Neru.** If Nix, home-manager or your distribution
already ships a `neru.service`, or you wrote one by hand, manage it there.
`install` refuses to overwrite a unit it did not write, and `uninstall` refuses
to disable or delete one. Every unit Neru installs opens with
``# Installed by `neru services install` ``, and Neru leaves any `neru.service`
without that line alone. To switch to Neru's own, remove yours the way you
created it. For example, run `systemctl --user disable --now neru.service` and
delete the file. Then run `neru services install`.

**Relocated `$XDG_CONFIG_HOME`?** Set it in your session, not only in a shell
rc. The user manager fixes its unit search path at login and never reads a
directory it did not know about then. `neru services install` checks the
manager's search path and says so rather than writing a unit that would sit
there unloaded.

## Known limitations

1. **Wayland global hotkeys** need `input` group access. Otherwise bind the
   modes in your compositor. See
   [Global hotkeys on Wayland](./linux-desktops.md#global-hotkeys-on-wayland).
2. **Hints need AT-SPI.** Grid and scroll work without it. Hints coverage
   varies by app, and Chromium and Electron apps need
   `--force-renderer-accessibility`. Where the tree is too thin, the `vision`
   and `contour` strategies work from a screen capture instead.
3. **Wayland modified clicks and passthrough** need the keyboard proxy, so
   `/dev/uinput` must be writable. Without it modified clicks may degrade and
   `general.passthrough_unbounded_keys` has no injection path.
4. **Native alerts are not modal.** Notifications and alerts go over
   `org.freedesktop.Notifications`, so a notification daemon such as mako, dunst
   or your desktop's own must be running or D-Bus activatable. Without one the
   two startup alerts fall back to stderr.
5. **Monitor hotplug** is tracked live, through RandR on X11 and `wl_output` on
   Wayland.
   A relaunch is only needed after a resolution or scale change to an existing
   monitor on Wayland.
6. **Desktop-specific limits**, such as portal consent and protocol gaps, are in
   [Linux desktops](./linux-desktops.md).

## Troubleshooting

Problems that are not Linux-specific are in
[Troubleshooting](./troubleshooting.md). Problems specific to one desktop are
under that desktop in [Linux desktops](./linux-desktops.md).

### "error while loading shared libraries: libtesseract.so.5"

The release binaries are built on Ubuntu, where tesseract's library is named
`libtesseract.so.5`. Fedora names the same library `libtesseract.so.5.5`, so
the binary refuses to start even with `tesseract-libs` installed. The installer
detects this and points here.

Add a compatibility link next to the library Fedora ships:

```bash
sudo dnf install -y tesseract-libs tesseract-langpack-eng
ls /usr/lib64/libtesseract.so.5.*          # note the exact name, e.g. libtesseract.so.5.5
sudo ln -s libtesseract.so.5.5 /usr/lib64/libtesseract.so.5
```

Use the name `ls` printed if it differs. Debian, Ubuntu and Arch already
provide `libtesseract.so.5` and do not need this. A build from source links
against the local name and does not need it either.

### "WAYLAND_DISPLAY is not set"

Neru is running under X11 or a TTY. It uses the X11 backend when `DISPLAY` is
set. If this comes from the systemd service, see
[Systemd user service](#systemd-user-service).

### "neru does not recognize this Wayland compositor"

`XDG_CURRENT_DESKTOP` names a compositor outside the supported set, so the
backend resolved to `wayland-other` and the daemon refused to start. Check the
variable against [Supported backends](#supported-backends), and see
[Checking compositor protocols](./linux-desktops.md#checking-compositor-protocols).

### "could not establish a libei input session via the RemoteDesktop portal"

KDE, COSMIC and GNOME route pointer input through the portal. Approve the
"Remote Control" prompt. See
[KDE troubleshooting](./linux-desktops.md#kde-plasma-wayland).

### "failed to connect to Wayland compositor"

Check that the session variable is set and the compositor answers:

```bash
echo $WAYLAND_DISPLAY
wayland-info   # wayland-utils package
```

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Add your user to the `input` group, log out and back in, and confirm with `id`.
See [Wayland keyboard capture permissions](#wayland-keyboard-capture-permissions).

### "Keyboard capture unavailable: /dev/uinput is not writable"

The proxy has keyboards to read but nothing to re-emit them through, so it
reads passively and modes fall back to the overlay's keyboard focus. Add the
udev rule from
[Wayland scroll injection permissions](#wayland-scroll-injection-permissions)
and restart the daemon.

### Overlay or hints wrong size after a display change

Adding or removing a monitor is tracked live. If the overlay is still wrong
after a resolution or scale change to an existing monitor,
[restart the daemon](./troubleshooting.md#restart-the-daemon).

### Sticky modifier indicator shows `[][][][]`

Set a font with modifier glyphs:

```toml
[sticky_modifiers.ui]
font_family = "Your installed symbol-capable font"
```

The family has to be one `fc-list` reports. A family fontconfig does not have
falls back to DejaVu Sans:

```bash
fc-list : family | grep -i "your font"
```

Paste `❖⇧⌥⌃` into a text editor to confirm the font renders before relying on
it in Neru.
