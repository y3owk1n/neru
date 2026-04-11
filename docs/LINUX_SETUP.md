# Linux Setup

Linux support in Neru is still in progress. The current tree now detects the
active Linux backend family and returns actionable guidance for unsupported
Wayland desktops instead of silently falling back.

## Planned Backend Matrix

| Compositor / Session | Backend | Current repo state |
| --- | --- | --- |
| Sway | Wayland / wlroots | Target Tier 1 |
| Hyprland | Wayland / wlroots | Target Tier 1 |
| niri | Wayland / wlroots | Target Tier 1 |
| River | Wayland / wlroots | Target Tier 1 |
| X11 (any WM) | X11 | Target Tier 1 |
| GNOME | Wayland / GNOME | Tier 2 placeholder |
| KDE Plasma | Wayland / KDE | Tier 2 placeholder |

## Build Dependencies

When native Linux backends land, the expected system packages are:

- Cairo: `libcairo2-dev` on Debian/Ubuntu, `cairo` on Arch, `pkgs.cairo` on NixOS
- Wayland: `libwayland-dev` and `wayland-protocols`
- X11: `libxtst-dev`, `libxrandr-dev`, `libxinerama-dev`, `libxfixes-dev`

NixOS `buildInputs` will need equivalents such as:

- `cairo`
- `wayland`
- `xorg.libXtst`
- `xorg.libXrandr`
- `xorg.libXinerama`
- `xorg.libXfixes`

## Wayland Hotkeys

Wayland compositors generally do not permit apps to register global hotkeys
directly. The intended Neru workflow is to bind compositor shortcuts to IPC
commands such as:

```bash
neru trigger recursive-grid
neru trigger grid
neru trigger hints
```

Example compositor bindings:

```text
# Sway / i3
bindsym $mod+Shift+c exec neru trigger recursive-grid
bindsym $mod+Shift+g exec neru trigger grid
bindsym $mod+Shift+space exec neru trigger hints

# Hyprland
bind = $mainMod SHIFT, C, exec, neru trigger recursive-grid

# niri
bind { key "Super+Shift+C" { spawn "neru" "trigger" "recursive-grid"; } }

# River
riverctl map normal Super+Shift C spawn 'neru trigger recursive-grid'
```

## Per-App Exclusions On Linux

Linux app exclusions use desktop identifiers such as `WM_CLASS`, not macOS
bundle IDs.

- X11: run `xprop WM_CLASS` and click the target window
- niri: `niri msg windows | grep app_id`
- Hyprland: `hyprctl clients | grep class`
- Sway: `swaymsg -t get_tree | grep app_id`

## Known Limitations

- Native Linux backends are not fully implemented in this repository yet
- Wayland global hotkeys require compositor bindings
- Per-app exclusions for native Wayland apps will need compositor-specific or portal-backed detection
- Hints mode on Linux still needs a real AT-SPI backend
- GNOME Wayland and KDE Wayland are intentionally detected as unsupported for now
