// Package atspi is the Linux accessibility backend.
//
// It walks the AT-SPI tree over D-Bus to find clickable elements, and asks the
// running compositor for window geometry — AT-SPI reports coordinates that
// wlroots, KWin, niri, Sway and Hyprland each place differently, so the
// per-compositor files here exist to reconcile them.
//
// Everything that is not tree walking — input injection, cursor, permissions —
// is delegated to the embedded native.Client.
//
// The whole directory is linux-only, so the directory carries the platform and
// the filenames do not repeat it. Adding a compositor means adding one file
// here and a case in window_origin.go, not editing a package shared with macOS
// and Windows.
package atspi
