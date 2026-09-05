# Linux Setup & Testing Guide

Prepare a Linux host to **build, test, and deploy** Neru. This guide covers
dependencies, permissions, building, validation, and generic troubleshooting.

Per-desktop-environment details and DE-specific known issues live in
[LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

**Related:** [Linux desktops](./LINUX_DESKTOPS.md) ·
[Cross-Platform Guide](./CROSS_PLATFORM.md) · [Installation](./INSTALLATION.md)

---

## Table of Contents

- [Supported backends](#supported-backends)
- [Install-time environment adjustments](#install-time-environment-adjustments)
- [Wayland keyboard capture permissions](#wayland-keyboard-capture-permissions)
- [Wayland scroll injection permissions](#wayland-scroll-injection-permissions)
- [Using nix home manager](#using-nix-home-manager)
- [Build dependencies](#build-dependencies)
- [Building](#building)
- [Validation & deployment](#validation--deployment)
- [Known limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)

---

## Supported backends

The backend is detected once at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`.

| Compositor / session                 | Backend           | Status                                                                           |
| ------------------------------------ | ----------------- | -------------------------------------------------------------------------------- |
| Sway, Hyprland, niri, River, Wayfire | `wayland-wlroots` | Supported                                                                        |
| KDE Plasma (Wayland)                 | `wayland-kde`     | Supported, see [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md#kde-plasma-wayland)       |
| X11 / XOrg, i3, GNOME on X11         | `x11`             | Supported                                                                        |
| GNOME (Wayland)                      | `wayland-gnome`   | Not supported, see [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md#gnome-not-supported)  |
| Any other Wayland compositor (COSMIC) | `wayland-other`  | Not supported, the daemon refuses to start                                       |

A Wayland session with `XDG_CURRENT_DESKTOP` unset is treated as wlroots.

---

## Install-time environment adjustments

Host changes required before Neru runs correctly:

| #   | Adjustment                                                  | Why                                                                                                   | Backends  | Persists?               |
| --- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | --------- | ----------------------- |
| 1   | Install [build dependencies](#build-dependencies)           | CGO backends and runtime libs                                                                         | All Linux | Yes                     |
| 2   | Add user to `input` group: `sudo usermod -aG input "$USER"` | Read `/dev/input`: keyboard capture and Neru's own `[hotkeys]` on Wayland                             | Wayland   | Yes (re-login required) |
| 3   | Make `/dev/uinput` writable (udev rule below)               | Write `/dev/uinput`: the keyboard proxy that makes capture instant, the scroll wheel, and `neru key`  | Wayland   | Yes (udev rule)         |
| 4   | Bind `neru <mode>` in compositor keybindings                | Only if you skip item 2                                                                               | Wayland   | Yes (user config)       |

Notes:

- X11 only needs item 1. Global hotkeys work via `XGrabKey` from Neru config.
- Item 2 takes effect after a full logout/login or reboot.
- Without item 3 Neru still runs, with two downgrades: modes capture keys
  through the overlay's keyboard focus (a hotkey chord also reaches the focused
  app), and scrolling falls back from the uinput wheel to the compositor seat:
  the virtual pointer on wlroots, which Chromium and Electron apps on Hyprland
  ignore, or libei through the portal session on KDE. `neru doctor` reports
  the scroll downgrade under `scroll`, and the daemon warns once at the first
  scroll that falls back.
- Item 4 is a **fallback, not a requirement**. With item 2 in place Neru's own
  `[hotkeys]` config works on Wayland. Bind in the compositor only if you would
  rather not grant `/dev/input` access. See
  [Global hotkeys on Wayland](./LINUX_DESKTOPS.md#global-hotkeys-on-wayland).

---

## Wayland keyboard capture permissions

On Wayland, Neru holds every keyboard (`EVIOCGRAB`) for the daemon's lifetime
and re-emits it through a uinput keyboard of its own, `neru-keyboard-proxy`.
The compositor reads that one device, so a mode captures keys the instant it
opens and a hotkey chord never reaches the focused app. Between modes every key
passes straight through. This needs item 2 (read `/dev/input`) and item 3
(write `/dev/uinput`) from the table above.

```bash
sudo usermod -aG input "$USER"
```

Log out and back in, then confirm `id` lists the `input` group.

> Membership in `input` allows reading system-wide keyboard events. Use a
> tighter distro-specific `udev`/ACL setup if the group is too broad for your
> environment.

When capture works, Neru logs `Evdev keyboard proxy running` at startup and
`Using Wayland evdev keyboard capture` when a mode opens. Without `/dev/uinput`
the proxy reads passively: hotkeys still work, but modes fall back to
overlay-focused capture, where basic navigation works and modified clicks may
degrade. Without `/dev/input` access there is no proxy at all.

Three consequences of holding the keyboards:

- A key remapper (kanata, keyd) grabs its input keyboards the same way. Start
  it before Neru: a device that refuses the grab is left to its owner and the
  remapper's virtual output keyboard is captured instead, which is the one that
  carries your keys. That device also advertises mouse motion and buttons, so a
  key can move the pointer; Neru re-emits those through a second device of its
  own, `neru-pointer-proxy`, created the first time such a keyboard is grabbed.
  Quitting the remapper while Neru runs is fine: Neru takes the keyboards it
  released, and hands them back when the remapper's output device reappears.
- A keyboard that reports absolute axes (a built-in trackpad on the same node)
  is never grabbed, so its keys are not captured.
- Compositor settings applied per input device (an `input` block in Sway or
  Hyprland, a per-device keyboard layout in KDE) apply to
  `neru-keyboard-proxy` while the daemon runs, since that is the keyboard the
  compositor sees.

---

## Wayland scroll injection permissions

On Wayland, Neru scrolls through a virtual mouse wheel it creates on
`/dev/uinput`, so the events enter the input stack below the compositor and
reach every client like a physical wheel. Most distros ship that node as
root-only. The `input` group does not cover it. Grant it with a udev rule:

```bash
echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-neru-uinput.rules
sudo udevadm control --reload && sudo udevadm trigger
```

Confirm with `ls -l /dev/uinput` (group `input`, mode `0660`) and restart the
daemon. If the node is missing entirely, load the module: `sudo modprobe uinput`.

The same rule is what lets the keyboard proxy above re-emit keys and what gives
`neru key` its fast path.

---

## Using nix home manager

Minimal flake with Home Manager:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    neru.url = "github:y3owk1n/neru";
  };

  outputs =
    {
      nixpkgs,
      home-manager,
      neru,
      ...
    }:
    {
      homeConfigurations."my-host" = home-manager.lib.homeManagerConfiguration {
        pkgs = nixpkgs.legacyPackages.x86_64-linux; # or aarch64-linux

        modules = [
          (
            { pkgs, ... }:
            {
              nixpkgs.overlays = [ neru.overlays.default ];
              home.username = "youruser";
              home.homeDirectory = "/home/youruser";
              home.stateVersion = "24.05";

              home.packages = [
                pkgs.neru
              ];

              programs.home-manager.enable = true;
            }
          )
        ];
      };
    };
}
```

---

## Build dependencies

Neru links `libei` and `liboeffis` at build time (the KDE input path). Install
the `-dev`/`-devel` packages below even if you only test on wlroots compositors.

Three packages are for reading what is on screen, and all three are required:

- **tesseract** recognizes on-screen text for `hints.strategy = "vision"`.
  Neru links it dynamically, so a missing `libtesseract.so` stops the daemon
  before any Neru code runs, whatever the strategy is set to.
- **tesseract English language data** is a separate package on every
  distribution and is resolved at use: without it Neru starts normally and the
  vision strategy reports that `eng.traineddata` is missing. To use language
  data elsewhere (a `tessdata_fast` checkout, say) point `TESSDATA_PREFIX` at
  it.
- **pipewire** carries screen pixels on KDE Plasma, where capture goes through
  the desktop portal's ScreenCast session. A missing `libpipewire-0.3.so` stops
  the daemon on every desktop, not only on KDE. On KDE you also approve screen
  sharing once, the first time a vision-strategy hint activation needs it.

### Debian / Ubuntu

```bash
sudo apt-get install -y \
  libcairo2-dev \
  libwayland-dev \
  libx11-dev \
  libxtst-dev \
  libxrandr-dev \
  libxinerama-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libei-dev \
  liboeffis-dev \
  libfontconfig-dev \
  libtesseract-dev \
  tesseract-ocr-eng \
  libpipewire-0.3-dev \
  wayland-protocols \
  fonts-dejavu-core
```

### Fedora

```bash
sudo dnf install -y \
  cairo-devel \
  wayland-devel \
  libX11-devel \
  libXtst-devel \
  libXrandr-devel \
  libXinerama-devel \
  libXfixes-devel \
  libxkbcommon-devel \
  libei-devel \
  liboeffis-devel \
  fontconfig-devel \
  tesseract-devel \
  tesseract-langpack-eng \
  pipewire-devel \
  wayland-protocols-devel \
  dejavu-sans-fonts dejavu-serif-fonts dejavu-sans-mono-fonts
```

### Arch Linux

```bash
sudo pacman -S \
  cairo \
  wayland \
  libx11 \
  libxtst \
  libxrandr \
  libxinerama \
  libxfixes \
  libxkbcommon \
  libei \
  fontconfig \
  tesseract \
  tesseract-data-eng \
  libpipewire \
  wayland-protocols \
  ttf-dejavu
```

On Arch, `liboeffis` is bundled in the `libei` package.

`fontconfig` is required at build time. DejaVu fonts are the defaults when
`font_family` is unset, and carry the sticky modifier symbols `❖⇧⌥⌃`.

---

## Building

```bash
# Native build on the host (recommended for local dev and testing)
just build

# Cross-build for a named Linux GOARCH (recipe defaults to amd64)
just build-linux          # amd64
just build-linux arm64    # arm64
```

Cross-compiling from macOS to Linux is not supported (CGO needs Linux headers).
From a macOS host use `just check-cross` for a type-check or `just lint-cross`
for a full CGO build and lint in Docker, see
[CROSS_PLATFORM.md](./CROSS_PLATFORM.md#build-and-test-commands).

Verify the binary matches your target:

```bash
go env GOARCH
file bin/neru
```

Run the [pre-commit checks](../CONTRIBUTING.md#making-changes) before opening a
PR. CI lints with `golangci-lint v2.12.2`, the version `devbox.json` pins, so
match it when validating locally.

---

## Validation & deployment

### Hotkey configuration

**X11:** Hotkeys in `config.toml` work via `XGrabKey`.

**Wayland:** Hotkeys in `config.toml` also work, through the evdev keyboard
proxy, provided the daemon can read `/dev/input` (item 2 above). If you would
rather not grant that access, bind `neru <mode>` in the compositor instead.
Compositor examples:

Sway (`~/.config/sway/config`):

```sway
bindsym $mod+Shift+h exec neru hints
bindsym $mod+Shift+g exec neru grid
bindsym $mod+Shift+s exec neru scroll
```

Hyprland (`~/.config/hypr/hyprland.conf`):

```hyprlang
bind = $mod SHIFT, H, exec, neru hints
bind = $mod SHIFT, G, exec, neru grid
bind = $mod SHIFT, S, exec, neru scroll
```

niri (`~/.config/niri/config.kdl`):

```kdl
binds {
    Mod+Shift+H { spawn-sh "neru hints"; }
    Mod+Shift+G { spawn-sh "neru grid"; }
    Mod+Shift+S { spawn-sh "neru scroll"; }
    Mod+Shift+R { spawn-sh "neru recursive_grid"; }
}
```

KDE Plasma and other desktops: see [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

### Application exclusions

Linux identifies apps by X11 `WM_CLASS` or the Wayland `app_id`:

```toml
[general]
excluded_apps = ["firefox", "chromium-browser", "code"]
```

### systemd user service

`neru services` manages the daemon for you. One command installs a systemd
user unit, enables it for every login, and starts it now:

```bash
neru services install
```

The other subcommands drive the same unit:

```bash
neru services status      # installed? running? enabled at login?
neru services stop        # stop now, still starts on next login
neru services start
neru services restart
neru services uninstall   # disable and remove the unit
```

**What it writes.** `neru.service` under `$XDG_CONFIG_HOME/systemd/user`
(`~/.config/systemd/user` by default, the same base directory `config.toml`
resolves from). `ExecStart` is the resolved path of the `neru` binary you ran
`install` with, so run `neru services uninstall && neru services install` after
moving the binary. The unit is anchored on `graphical-session.target`, see
"Service management on Linux" under the
[Capability Matrix](./CROSS_PLATFORM.md#capability-matrix).

**Your session has to export itself first.** A systemd *user* manager starts
before your compositor and inherits nothing from it. Unless the session imports
its own variables, `neru launch` runs with no `WAYLAND_DISPLAY`, `DISPLAY` or
compositor socket and cannot find a display server. Most desktop environments
(KDE Plasma) and session wrappers (`uwsm`) do this for you. A bare compositor
started from a TTY does not. Add this to your compositor config before anything
that depends on it:

```
# sway (~/.config/sway/config)
exec systemctl --user import-environment \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec dbus-update-activation-environment --systemd \
  WAYLAND_DISPLAY DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP XDG_SESSION_TYPE
exec systemctl --user start graphical-session.target
```

Hyprland, niri and River take the same three lines with their own socket
variable (`HYPRLAND_INSTANCE_SIGNATURE`, `NIRI_SOCKET`) in place of `SWAYSOCK`.

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
autostart. This is a stated boundary, see
[ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md).

**Installed through a package manager, or wrote the unit yourself?** If Nix,
home-manager or your distribution already ships a `neru.service`, or you wrote
one by hand, manage it there. `install` refuses to overwrite a unit it did not
write, and `uninstall` refuses to disable or delete one. Ownership is read out
of the file: every unit Neru installs opens with
``# Installed by `neru services install` ``, and a `neru.service` without that
line is one Neru will not touch. Remove yours the way you created it
(`systemctl --user disable --now neru.service` and delete the file), then
`neru services install` writes Neru's own in its place.

**Relocated `$XDG_CONFIG_HOME`?** Set it in your session, not only in a shell
rc. The user manager fixed its unit search path at login, so a directory it
never heard of is one it will never read. `neru services install` checks the
manager's own search path and says so rather than writing a unit that would sit
there unloaded.

---

## Known limitations

1. **Wayland global hotkeys** need `input`-group access and a CGO build.
   Otherwise bind the modes in your compositor. See
   [Global hotkeys on Wayland](./LINUX_DESKTOPS.md#global-hotkeys-on-wayland).
2. **Hints need AT-SPI.** Grid and scroll work without it. Hints coverage
   varies by app, and Chromium and Electron apps need
   `--force-renderer-accessibility`. Where the tree is too thin, the `vision`
   and `contour` strategies work from a screen capture instead.
3. **Wayland modified clicks and passthrough** need the keyboard proxy, so
   `/dev/uinput` must be writable. Without it modified clicks may degrade and
   `general.passthrough_unbounded_keys` has no injection path.
4. **Native alerts are not modal.** Notifications and alerts go over
   `org.freedesktop.Notifications`, so a notification daemon (mako, dunst, or
   your desktop's own) must be running or D-Bus activatable. Without one the
   two startup alerts fall back to stderr.
5. **Monitor hotplug** is tracked live (RandR on X11, `wl_output` on Wayland).
   A relaunch is only needed after a resolution or scale change to an existing
   monitor on Wayland.
6. **DE-specific limits** (portal consent, protocol gaps):
   [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

---

## Troubleshooting

### "WAYLAND_DISPLAY is not set"

Running under X11 or a TTY. Neru uses the X11 backend when `DISPLAY` is set.

### "neru does not recognize this Wayland compositor"

`XDG_CURRENT_DESKTOP` names a compositor outside the supported set, so the
backend resolved to `wayland-other` and the daemon refused to start. Check the
variable, and see [Checking compositor protocols](./LINUX_DESKTOPS.md#checking-compositor-protocols)
before trying to add the compositor.

### "KWin does not implement zwlr_virtual_pointer_v1"

Expected on KDE, where input routes through libei and the message accompanies
a failed portal session. Approve the "Remote Control" prompt, see
[KDE troubleshooting](./LINUX_DESKTOPS.md#kde-plasma-wayland).

### Overlay or hints wrong size after display change

Monitor hotplug (add/remove) is tracked live. If the overlay is still wrong
after a resolution or scale change to an existing monitor, relaunch:
`neru stop` then `neru launch`.

### "failed to connect to Wayland compositor"

```bash
echo $WAYLAND_DISPLAY
wayland-info   # wayland-utils package
```

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Add the user to `input`, re-login, confirm with `id`. See
[keyboard permissions](#wayland-keyboard-capture-permissions).

### "Keyboard capture unavailable: /dev/uinput is not writable"

The proxy has keyboards to read but nothing to re-emit them through, so it
reads passively and modes fall back to the overlay's keyboard focus. Add the
udev rule from
[scroll injection permissions](#wayland-scroll-injection-permissions) and
restart the daemon.

### Sticky modifier indicator shows `[][][][]`

Set a font with modifier glyphs:

```toml
[sticky_modifiers.ui]
font_family = "Your installed symbol-capable font"
```

The family has to be one `fc-list` reports. A family fontconfig does not have
falls back to DejaVu Sans rather than to fontconfig's substitute for the name
(footnote 1 of the [Capability Matrix](CROSS_PLATFORM.md#capability-matrix)):

```bash
fc-list : family | grep -i "your font"
```

Paste `❖⇧⌥⌃` into a text editor to confirm the font renders before relying on
it in Neru.

DE-specific troubleshooting: [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).
