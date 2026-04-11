# KDE Wayland Backend (Tier 2 - Not Implemented)

This directory is reserved for a future KDE Plasma Wayland backend.

KDE Plasma does not guarantee the wlroots protocol set used by the planned
`wayland_wlroots` backend. In particular, input injection and overlay support
need compositor-aware integration rather than assuming wlroots extensions.

## What a KDE backend would need

Input injection:
`libei`, likely through a CGo wrapper.
Reference: https://gitlab.freedesktop.org/libinput/libei

Overlay rendering:
KWin-specific APIs or runtime-verified layer-shell support where available.

Global hotkeys:
`org.freedesktop.portal.GlobalShortcuts` via D-Bus.

Active app detection:
KWin or portal APIs, with `_NET_ACTIVE_WINDOW` as a fallback for XWayland.

## Detection

The Linux platform factory detects KDE through `XDG_CURRENT_DESKTOP`
containing `"KDE"` and currently returns a clear `CodeNotSupported` error
that points contributors to this placeholder and `docs/LINUX_SETUP.md`.
