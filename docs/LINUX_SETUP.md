# Linux Setup & Testing Guide

Prepare a Linux host to **build, test, and deploy** Neru. This guide covers
dependencies, permissions, building, validation, and generic troubleshooting.

Per-desktop-environment implementation details and DE-specific known issues live
in [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

**Related:** [Linux desktops](./LINUX_DESKTOPS.md) ·
[Cross-Platform Guide](./CROSS_PLATFORM.md) · [Installation](./INSTALLATION.md)

---

## Table of Contents

- [Supported backends](#supported-backends)
- [Install-time environment adjustments](#install-time-environment-adjustments)
- [Wayland keyboard capture permissions](#wayland-keyboard-capture-permissions)
- [Using nix home manager](#using-nix-home-manager)
- [Build dependencies](#build-dependencies)
- [Building](#building)
- [Validation & deployment](#validation--deployment)
- [Known limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)

---

## Supported backends

| Compositor / session        | Backend         | Status                                                                           |
| --------------------------- | --------------- | -------------------------------------------------------------------------------- |
| Sway, Hyprland, niri, River | wayland-wlroots | Supported                                                                        |
| KDE Plasma (Wayland)        | wayland-kde     | Supported — see [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md#kde-plasma-wayland)      |
| X11 / XOrg, i3              | x11             | Supported                                                                        |
| GNOME (Wayland)             | wayland-gnome   | Not supported — see [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md#gnome-not-supported) |

---

## Install-time environment adjustments

Host changes required before Neru runs correctly (not code changes):

| #   | Adjustment                                                  | Why                                   | Backends  | Persists?               |
| --- | ----------------------------------------------------------- | ------------------------------------- | --------- | ----------------------- |
| 1   | Install [build dependencies](#build-dependencies)           | CGO backends and runtime libs         | All Linux | Yes                     |
| 2   | Add user to `input` group: `sudo usermod -aG input "$USER"` | `evdev` keyboard capture **and Neru's own global hotkeys** on Wayland | Wayland | Yes (re-login required) |
| 3   | Bind `neru <mode>` in compositor keybindings                | Only needed if you skip item 2        | Wayland   | Yes (user config)       |

Notes:

- X11 only needs item 1; global hotkeys work via `XGrabKey` from Neru config.
- Item 2 takes effect after a full logout/login or reboot.
- Item 3 is a **fallback, not a requirement**: with item 2 in place Neru's own
  `[hotkeys]` config works on Wayland through a passive evdev listener. Bind in
  the compositor only if you would rather not grant `/dev/input` access. See
  [Global hotkeys on Wayland](./LINUX_DESKTOPS.md#global-hotkeys-on-wayland).
- Item 3 cannot be automated by a package; ship example snippets where helpful.

---

## Wayland keyboard capture permissions

On Wayland, Neru uses direct `evdev` keyboard capture during active modes so
modified clicks and sticky modifiers work reliably.

```bash
sudo usermod -aG input "$USER"
```

Log out and back in, then confirm `id` lists the `input` group.

> Membership in `input` allows reading system-wide keyboard events. Use a tighter
> distro-specific `udev`/ACL setup if the group is too broad for your environment.

When capture works, Neru logs `Using Wayland evdev keyboard capture`. Without
device access it falls back to overlay-focused capture; basic navigation still
works but modified clicks may degrade.

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

Neru links `libei` and `liboeffis` at build time (KDE and future libei-based
Wayland paths). Install the `-dev`/`-devel` packages below even if you only test
on wlroots compositors today.

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
  wayland-protocols \
  ttf-dejavu
```

On Arch, `liboeffis` (required by the KDE/libei path) is bundled in the `libei`
package, so no separate package is needed.

`fontconfig` is required at build time. DejaVu fonts are recommended defaults
when `font_family` is unset (sticky modifier symbols `❖⇧⌥⌃`).

---

## Building

```bash
# Native build on the host (recommended for local dev and testing)
just build

# Cross-build for a named Linux GOARCH (recipe defaults to amd64)
just build-linux          # amd64
just build-linux arm64    # arm64

# Cross-compilation from macOS to Linux is NOT supported (CGO + Linux headers)
```

Verify the binary matches your target:

```bash
go env GOARCH
file bin/neru
```

Run the [pre-commit checks](../CONTRIBUTING.md#making-changes) before opening a
PR. One Linux-specific note: CI lints with `golangci-lint v2.12.2`, so match
that version when validating locally.

---

## Validation & deployment

### Hotkey configuration

**X11:** Hotkeys in `config.toml` work via `XGrabKey`.

**Wayland:** Hotkeys in `config.toml` also work, through a passive `evdev`
listener, provided the daemon can read `/dev/input` (see item 2 above). If you
would rather not grant that access, bind `neru <mode>` in the compositor
instead — see [Global hotkeys on Wayland](./LINUX_DESKTOPS.md#global-hotkeys-on-wayland).
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

Linux uses X11 `WM_CLASS` or Wayland process name from `/proc/<pid>/cmdline`:

```toml
[general]
excluded_apps = ["firefox", "chromium-browser", "code"]
```

### systemd user service

`neru services install/start/stop` is macOS-only. On Linux, use systemd:

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neru.service << EOF
[Unit]
Description=Neru keyboard navigation daemon
After=graphical-session.target
PartOf=graphical-session.target

[Service]
ExecStart=%h/.local/bin/neru launch
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now neru
```

---

## Known limitations

1. **Wayland global hotkeys** — Neru's own `[hotkeys]` config works via a passive
   evdev listener, which needs `input`-group access and a CGO build; otherwise
   bind the modes in your compositor instead. See
   [Global hotkeys on Wayland](./LINUX_DESKTOPS.md#global-hotkeys-on-wayland).
2. **Hints need AT-SPI** — Grid and scroll work without it; hints coverage varies
   by app. DE-specific coordinate details: [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).
3. **Dark mode** — Via `org.freedesktop.appearance` portal, with session-specific
   fallbacks where the portal is unavailable.
4. **Notifications** — Delivered over `org.freedesktop.Notifications` on the
   session bus, so a notification daemon (mako, dunst, or your desktop's own)
   has to be running. What Neru does when none is, and why an alert is not
   modal here: "Native alerts on Linux" under the
   [Capability Matrix](./CROSS_PLATFORM.md#capability-matrix).
5. **Wayland modified clicks** — Need `evdev` access (see [keyboard permissions](#wayland-keyboard-capture-permissions)).
6. **Monitor hotplug** — Adding/removing a monitor is tracked live (RandR on X11,
   `wl_output` on Wayland) and the overlay follows; a relaunch is only needed for a
   resolution/scale change to an existing monitor on Wayland.
7. **DE-specific limits** (portal consent, protocol gaps): [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

---

## Troubleshooting

### "WAYLAND_DISPLAY is not set"

Running under X11 or a TTY. Neru uses the X11 backend when `DISPLAY` is set.

### "compositor does not support zwlr_virtual_pointer_v1"

Expected on KDE, where input routes through libei instead. On any other
compositor it means there is no usable input path — see
[Checking compositor protocols](./LINUX_DESKTOPS.md#checking-compositor-protocols).

### Overlay or hints wrong size after display change

Monitor hotplug (add/remove) is tracked live. If the overlay is still wrong after
a resolution or scale change to an existing monitor, relaunch: `neru stop` then
`neru launch`.

### "failed to connect to Wayland compositor"

```bash
echo $WAYLAND_DISPLAY
wl-info   # wayland-utils package
```

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Add the user to `input`, re-login, confirm with `id`. See
[keyboard permissions](#wayland-keyboard-capture-permissions).

### Sticky modifier indicator shows `[][][][]`

Set a font with modifier glyphs:

```toml
[sticky_modifiers.ui]
font_family = "Your installed symbol-capable font"
```

Check the family is installed — a family fontconfig does not have falls back to
the DejaVu generic rather than to fontconfig's substitute for it
([font resolution](CROSS_PLATFORM.md#capability-matrix)), so the name has to be
one `fc-list` reports:

```bash
fc-list : family | grep -i "your font"
```

Paste `❖⇧⌥⌃` into a text editor to confirm the font renders before relying on it
in Neru.

DE-specific troubleshooting: [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).
