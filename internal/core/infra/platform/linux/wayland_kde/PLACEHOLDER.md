# KDE Plasma Wayland Backend (Not Implemented)

This directory is reserved for a future KDE Plasma Wayland backend.

KDE uses KWayland and does not expose the primary wlroots protocols used by the
`wayland_wlroots` backend, most notably `zwlr_layer_shell_v1` and
`zwlr_virtual_pointer_v1`. A separate implementation is required.

## What a KDE backend would need

Input injection:
`libei` (Emulated Input), KWayland-specific protocols, or potentially D-Bus
interfaces exposed by KWin.

Overlay rendering:
Either a KWin script, KWayland specific surface roles, or an XWayland fallback.
A native Wayland overlay without wlroots layer-shell is the primary challenge.

Global hotkeys:
`org.freedesktop.portal.GlobalShortcuts` via D-Bus or KGlobalAccel (`org.kde.kglobalaccel`).

Active app detection:
KWin D-Bus interface (`org.kde.KWin`), with `_NET_ACTIVE_WINDOW` as a fallback
for XWayland apps.

## Status

The Linux platform factory detects KDE through `XDG_CURRENT_DESKTOP`
containing `"KDE"` and returns a clear `CodeNotSupported` error.

For more information, see [Linux Setup](../../../../../../docs/LINUX_SETUP.md).
