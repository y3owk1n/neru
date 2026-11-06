//go:build linux

// Package gnomeshell holds the one GNOME window source in the tree.
//
// Mutter tells a Wayland client neither which window is focused nor where it
// is. This package carries a small GNOME Shell extension that owns
// org.neru.Shell on the session bus, answers FocusedWindow with the focused
// window's app id, title and frame, and emits FocusedWindowChanged when focus
// changes or the focused window moves, resizes or retitles. The bridge caches
// the last answer.
//
// Three callers read that one cache, as on KDE: the AT-SPI client offsets
// window-relative element coordinates by the window's origin,
// SystemPort.FocusedWindowBounds reports the rectangle, and the app watcher
// keys per-app configuration on the app id. All of them read the process-wide
// bridge returned by Shared.
//
// The shell loads a new extension only at login, so when the shell is on the
// bus and the extension is not, the bridge installs and enables it and says so
// once. It starts serving after the next login.
package gnomeshell
