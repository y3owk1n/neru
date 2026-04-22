# Linux Setup & Testing Guide

Neru provides native Linux support through two display server backends:

- **X11** — works with any X11-based session (XOrg, i3, etc.)
- **Wayland (wlroots)** — works with wlroots-based compositors (Sway, Hyprland, niri, River)

> **GNOME and KDE Wayland** are not yet supported. These compositors use their own private protocols instead of the wlroots protocols. See the placeholder files in `internal/core/infra/platform/linux/wayland_gnome/` and `wayland_kde/` for contribution guidance.

---

## Supported Compositors & Backends

| Compositor | Backend         | Status           | Notes                                                   |
| ---------- | --------------- | ---------------- | ------------------------------------------------------- |
| Sway       | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| Hyprland   | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| niri       | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| River      | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| X11 / XOrg | x11             | ✅ Supported     | XTest for input, XRandR for screens                     |
| i3         | x11             | ✅ Supported     | Runs under X11                                          |
| GNOME      | wayland-gnome   | 🔲 Not Supported | Needs libei + GNOME Shell extension; see PLACEHOLDER.md |
| KDE Plasma | wayland-kde     | 🔲 Not Supported | Needs KDE-specific protocols; see PLACEHOLDER.md        |

---

## Wayland Keyboard Capture Permissions

On wlroots-based Wayland compositors, Neru uses direct `evdev` keyboard capture
during active modes so modified clicks like `Ctrl+click` and sticky modifiers
work reliably against the app underneath the overlay.

This requires permission to open and grab `/dev/input/event*` keyboard devices.
On many distros those devices are owned by `root:input` with mode `0660`, so
your user must be in the `input` group.

```bash
sudo usermod -aG input "$USER"
```

Then fully log out and back in, or reboot, and confirm:

```bash
id
```

You should see `input` in the printed group list.

> **Security note:** Membership in `input` allows reading system-wide keyboard
> events. If that access is too broad for your environment, use a tighter
> distro-specific `udev`/ACL setup instead of the group-based approach.

When the backend is active, Neru logs:

```text
Using Wayland evdev keyboard capture
```

If Neru cannot access the devices, it falls back to overlay-focused keyboard
capture. Basic mode navigation still works, but modified clicks and sticky
modifier behavior may be degraded under Wayland.

---

## Using nix home manager

Below is a minimal single flake with home manager setup.

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

## Build Dependencies

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
  wayland-protocols
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
  wayland-protocols-devel
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
  wayland-protocols
```

---

## Building

```bash
# Build for current platform
just build

# Build specifically for Linux
just build-linux

# Cross-compilation from macOS is NOT supported for Linux targets
# because the native backends require CGo and Linux system headers.
```

---

## Under The Hood: Wayland (wlroots) Architecture Details for Testers

As a tester on Linux Wayland environments, here are key implementation details you should be aware of to effectively test edge cases.

### Global Cursor Discovery ("Agitation" Trick)

Unlike X11 or macOS, Wayland completely hides the global mouse coordinate from clients for security reasons. Before Neru can successfully navigate matrices or grids, it must learn the current mouse position.
**How we solved it:** Upon startup, Neru spawns invisible full-screen `layer-shell` surfaces across all outputs. It then "wiggles" the virtual pointer natively. This forces the compositor to send a `pointer_enter` event to our transparent surface, allowing us to map local surface coordinates to global screen dimensions and capture exactly where your mouse is.

> **Testing Tip:** Ensure the cursor accurately discovers its initial position across multi-monitor setups (especially asymmetric resolutions).

### Proper Key Modifier Routing

Wayland passes key codes independently of modifiers. Translating `Ctrl+K` relies entirely on your compositor's active XKB map.
**How we solved it:** We leverage `xkb_state_mod_name_is_active` to explicitly inspect `Shift`, `Control`, `Mod1` (Alt), and `Mod4` (Super). Printable UTF-8 characters are correctly resolved utilizing `xkb_state_key_get_utf8` (fixing cases where `,` or `/` evaluate incorrectly to "comma").

> **Testing Tip:** Rapidly tap modifiers alongside character keys (like `Shift`, `Ctrl`, and complex symbols like `+` or `,`) to verify hotkeys trigger successfully.

### Click-Through Layer Shell Overlays

To draw overlay UI (like Grid Mode) without stealing physical mouse clicks, Wayland requires precise protocol negotiations.
**How we solved it:** Neru sets an explicit, empty `wl_region` as the `input_region` for its layer-shell surfaces. This forces the compositor to ignore the overlay for pointer intersection testing, enabling true click-through capability where your synthetic clicks land exactly on the app under the overlay grid.

> **Testing Tip:** While in Recursive Grid mode, executing a synthetic click (e.g., `u` for left click) should seamlessly pass straight through the overlay into your browser or terminal underneath.

### Wayland Smooth Scrolling

Rather than sending redundant, chunky fractional scroll loops, Neru directly pipes raw pixel `deltaY` and `deltaX` into the standard Wayland continuous `axis` event without discretizing them. This affords ultra-smooth precise scrolling behaviors equivalent to macOS behavior.

> **Testing Tip:** Enter Scroll mode and verify scrolling operates smoothly, cleanly maps to your configured scroll increments, and does not overwhelm/lag the compositor event queues.

### Dynamic Rendering Buffer Lifecycles

When UI modes rapidly open, exit, and re-open, the underlying Cairo buffers require clean state management so they don't unexpectedly disappear.
**How we solved it:** Wayland buffers are dynamically and lazily initialized immediately prior to any stroke or draw commands—ensuring they reliably exist, even if a previous mode exit forcefully destroyed the window canvas.

---

## Validation & Setup Guide

### 1. Hotkey Configuration

**X11:**
On X11, Neru registers global hotkeys natively using `XGrabKey`. Hotkeys specified in your `config.toml` "just work".

**Wayland (wlroots):**
Wayland does not have a standard protocol for global hotkey registration. You **must** bind `neru <mode>` via your compositor's own keybinding config!

#### Sway Example

```sway
# ~/.config/sway/config
bindsym $mod+Shift+h exec neru hints
bindsym $mod+Shift+g exec neru grid
bindsym $mod+Shift+s exec neru scroll
```

#### Hyprland Example

```hyprlang
# ~/.config/hypr/hyprland.conf
bind = $mod SHIFT, H, exec, neru hints
bind = $mod SHIFT, G, exec, neru grid
bind = $mod SHIFT, S, exec, neru scroll
```

#### niri Example

```kdl
// ~/.config/niri/config.kdl
binds {
    Mod+Shift+H repeat=false { spawn-sh "neru hints"; }
    Mod+Shift+G repeat=false { spawn-sh "neru grid"; }
    Mod+Shift+S repeat=false { spawn-sh "neru scroll"; }
    Mod+Shift+R repeat=false { spawn-sh "neru recursive_grid"; }
}
```

`niri` binds repeat by default. Use `repeat=false` for Neru mode launchers so
holding the activation key does not continuously relaunch the mode.

### 2. Application Exclusions

On Linux, applications are identified by their X11 `WM_CLASS` (X11) or process name from `/proc/<pid>/cmdline` (Wayland). Use these exact identifiers in your `excluded_apps` list.

```toml
[general]
excluded_apps = ["firefox", "chromium-browser", "code"]
```

### 3. Service Management

`neru services install/start/stop` uses `launchctl` on macOS. For Linux, use standard `systemd`:

```bash
# Create a systemd user service
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neru.service << EOF
[Unit]
Description=Neru keyboard navigation daemon

[Service]
ExecStart=%h/.local/bin/neru daemon
Restart=on-failure

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now neru
```

---

## Known Limitations

1. **Wayland global hotkeys**: Must be configured in the compositor, not in Neru's config. See [Hotkey Configuration](#1-hotkey-configuration).
2. **Accessibility (AT-SPI)**: Full AT-SPI integration for clickable element discovery (hints mode) is currently unavailable natively under Wayland without relying on experimental plugins. Grid mode and scroll mode both work perfectly without AT-SPI.
3. **Dark mode / Theme polling detection**: Not yet implemented. Output will fall back to default theme definitions.
4. **Notifications**: Desktop notifications (`org.freedesktop.Notifications`) will log to stdout/file instead of pushing to DBus.
5. **Wayland modified clicks need evdev access**: On wlroots compositors, reliable
   modified pointer actions depend on the `evdev` keyboard-capture path described
   above. Without `/dev/input/event*` access, Neru falls back to a less capable
   overlay-focused path.

---

## Troubleshooting

### "WAYLAND_DISPLAY is not set"

You're running under X11 or a TTY. Neru will automatically use the X11 backend when `DISPLAY` is set. If you're in a purely headless TTY wrapper, Neru cannot hook inputs.

### "compositor does not support zwlr_virtual_pointer_v1"

Your Wayland compositor does not currently implement `wlr` unstable protocols. This typically occurs under strictly isolated GNOME or KDE sessions. Check the placeholder docs to learn how libei implementations will govern GNOME support in the future.

### "failed to connect to Wayland compositor"

Check that `WAYLAND_DISPLAY` is set correctly and the Wayland socket permissions map appropriately (especially useful if testing behind Flatpaks or tight sandbox boundaries).

```bash
echo $WAYLAND_DISPLAY
wl-info  # from wayland-utils package
```

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Neru could not open and grab the keyboard devices needed for reliable Wayland
modifier handling.

Common fix:

```bash
sudo usermod -aG input "$USER"
```

Then log out and back in, and confirm:

```bash
id
```

If the setup is correct, Neru should log:

```text
Using Wayland evdev keyboard capture
```

### Sticky modifier indicator shows `[][][][]`

The sticky modifier overlay uses Unicode modifier symbols on Linux:
`❖⇧⌥⌃`. If you see square boxes instead, the configured font does not include
those glyphs.

Set a font explicitly in your config:

```toml
[sticky_modifiers.ui]
font_family = "Your installed symbol-capable font"
```

An empty `font_family` uses the system default, which may not have the required
symbol coverage. A quick way to verify a candidate font is to paste `❖⇧⌥⌃` into
a text editor and confirm the symbols render there first.
