# GNOME Wayland Backend (Tier 2 - Not Implemented)

This directory is reserved for a future GNOME Wayland backend.

GNOME does not expose the wlroots protocols used by the `wayland_wlroots`
backend, including `zwlr_layer_shell_v1` and `zwlr_virtual_pointer_v1`.
A separate implementation is required.

## What a GNOME backend would need

Input injection:
`libei` (Emulated Input), likely through a dedicated CGo wrapper, or via GNOME
Shell extension.

Overlay rendering:
Either a GNOME Shell extension, Mutter-specific APIs, or a carefully-scoped
fallback through XWayland when present.

Global hotkeys:
`org.freedesktop.portal.GlobalShortcuts` via D-Bus, or a GNOME Shell extension.

Active app detection:
GNOME Shell or Mutter D-Bus signals, with `_NET_ACTIVE_WINDOW` as a fallback
for XWayland-hosted apps.

## Status

The Linux platform factory detects GNOME through `XDG_CURRENT_DESKTOP`
containing `"GNOME"` and returns a clear `CodeNotSupported` error.

For more information, see [Linux Setup](../../../../../../docs/LINUX_SETUP.md).
