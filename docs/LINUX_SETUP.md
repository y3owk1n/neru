# Linux Setup Guide

Neru provides native Linux support through two display server backends:

- **X11** — works with any X11-based session (XOrg, i3, etc.)
- **Wayland (wlroots)** — works with wlroots-based compositors (Sway, Hyprland, niri, River)

> **GNOME and KDE Wayland** are not yet supported (Tier 2). These compositors
> use their own private protocols instead of the wlroots protocols. See the
> placeholder files in `internal/core/infra/platform/linux/wayland_gnome/` and
> `wayland_kde/` for contribution guidance.

---

## Supported Compositors

| Compositor  | Backend           | Status        | Notes                                                              |
| ----------- | ----------------- | ------------- | ------------------------------------------------------------------ |
| Sway        | wayland-wlroots   | ✅ Supported  | Full virtual-pointer and layer-shell support                       |
| Hyprland    | wayland-wlroots   | ✅ Supported  | Full virtual-pointer and layer-shell support                       |
| niri        | wayland-wlroots   | ✅ Supported  | Full virtual-pointer and layer-shell support                       |
| River       | wayland-wlroots   | ✅ Supported  | Full virtual-pointer and layer-shell support                       |
| X11 / XOrg  | x11               | ✅ Supported  | XTest for input, XRandR for screens                                |
| i3          | x11               | ✅ Supported  | Runs under X11                                                     |
| GNOME       | wayland-gnome     | 🔲 Tier 2    | Needs libei + GNOME Shell extension; see PLACEHOLDER.md            |
| KDE Plasma  | wayland-kde       | 🔲 Tier 2    | Needs KDE-specific protocols; see PLACEHOLDER.md                   |

---

## Build Dependencies

### Debian / Ubuntu

```bash
sudo apt-get install -y \
  libcairo2-dev \
  libwayland-dev \
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
  libxtst \
  libxrandr \
  libxinerama \
  libxfixes \
  libxkbcommon \
  wayland-protocols
```

### NixOS / Devbox

The project's `devbox.json` already includes the necessary dependencies for
macOS. For Linux, ensure the following are available in your environment or
`devbox.json` `packages`:

```
cairo
wayland
libXtst
libXrandr
libXinerama
libXfixes
libxkbcommon
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

## Hotkey Configuration

### X11

On X11, Neru registers global hotkeys natively using `XGrabKey`. The hotkey
configuration in your `config.toml` works exactly as documented:

```toml
[keybindings]
activate_hints = "ctrl+shift+h"
activate_grid = "ctrl+shift+g"
```

### Wayland (wlroots)

Wayland does not have a standard protocol for global hotkey registration.
On wlroots-based compositors, you must bind `neru trigger <mode>` in your
compositor's own keybinding config.

#### Sway Example

```sway
# ~/.config/sway/config
bindsym $mod+Shift+h exec neru trigger hints
bindsym $mod+Shift+g exec neru trigger grid
bindsym $mod+Shift+s exec neru trigger scroll
```

#### Hyprland Example

```hyprlang
# ~/.config/hypr/hyprland.conf
bind = $mod SHIFT, H, exec, neru trigger hints
bind = $mod SHIFT, G, exec, neru trigger grid
bind = $mod SHIFT, S, exec, neru trigger scroll
```

#### niri Example

```kdl
// ~/.config/niri/config.kdl
binds {
    Mod+Shift+H { spawn "neru" "trigger" "hints"; }
    Mod+Shift+G { spawn "neru" "trigger" "grid"; }
    Mod+Shift+S { spawn "neru" "trigger" "scroll"; }
}
```

---

## Application Exclusions

On Linux, applications are identified by their X11 `WM_CLASS` (X11) or
process name from `/proc/<pid>/cmdline` (Wayland). Use these identifiers in
the `excluded_apps` list:

```toml
[general]
excluded_apps = ["firefox", "chromium-browser", "code"]
```

---

## Known Limitations

1. **Wayland global hotkeys**: Must be configured in the compositor, not in
   Neru's config. See [Hotkey Configuration](#hotkey-configuration).

2. **Wayland cursor position**: The initial cursor position is not known until
   the first virtual pointer move. Neru defaults to (0,0) until a move occurs.

3. **Accessibility (AT-SPI)**: Full AT-SPI integration for clickable element
   discovery (hints mode) is not yet implemented. Grid mode and scroll mode
   work without AT-SPI.

4. **Dark mode detection**: Not yet implemented. The overlay will use the
   default theme.

5. **Notifications**: Desktop notifications (`org.freedesktop.Notifications`)
   are not yet implemented. Errors are logged instead.

7. **Service management**: `neru services install/start/stop` uses `launchctl`
   on macOS. For Linux, use systemd:

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

## Troubleshooting

### "WAYLAND_DISPLAY is not set"

You're running under X11 or a TTY. Neru will automatically use the X11
backend when `DISPLAY` is set. If you're in a TTY, Neru cannot run.

### "compositor does not support zwlr_virtual_pointer_v1"

Your Wayland compositor does not implement the wlroots virtual pointer
protocol. This typically means you're running GNOME or KDE, which are not
yet supported. See the Tier 2 placeholder docs.

### "failed to connect to Wayland compositor"

Check that `WAYLAND_DISPLAY` is set correctly and the Wayland compositor is
running. You can verify with:

```bash
echo $WAYLAND_DISPLAY
wl-info  # from wayland-utils package
```

### Build Fails: Missing Headers

Install the build dependencies listed above for your distribution.
