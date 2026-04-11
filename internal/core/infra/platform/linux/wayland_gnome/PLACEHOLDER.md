# GNOME Wayland Backend (Tier 2 - Not Implemented)

This directory is reserved for a future GNOME Wayland backend.

GNOME does not expose the wlroots protocols used by the planned
`wayland_wlroots` backend, including `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. A separate implementation is required.

## What a GNOME backend would need

Input injection:
`libei` (Emulated Input), likely through a dedicated CGo wrapper.
Reference: https://gitlab.freedesktop.org/libinput/libei

Overlay rendering:
Either a GNOME Shell extension, Mutter-specific APIs, or a carefully-scoped
fallback through XWayland when present.

Global hotkeys:
`org.freedesktop.portal.GlobalShortcuts` via D-Bus.

Active app detection:
GNOME Shell or Mutter D-Bus signals, with `_NET_ACTIVE_WINDOW` as a fallback
for XWayland-hosted apps.

## Detection

The Linux platform factory detects GNOME through `XDG_CURRENT_DESKTOP`
containing `"GNOME"` and currently returns a clear `CodeNotSupported` error
that points contributors to this placeholder and `docs/LINUX_SETUP.md`.
