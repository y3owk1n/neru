//go:build linux

// Package gnomeshell holds the one GNOME window source in the tree.
//
// Mutter tells a Wayland client neither which window is focused nor where it
// is, and GNOME Shell's own introspection interface answers only the portals.
// What GNOME does offer is extensions: this package carries a small GNOME
// Shell extension that owns org.neru.Shell on the session bus, answers
// FocusedWindow with the focused window's app id, title and frame, and emits
// FocusedWindowChanged on every focus change and on the focused window
// moving, resizing or retitling. The bridge here caches the last answer.
//
// Three callers read that one cache, as on KDE: the AT-SPI client offsets
// window-relative element coordinates by the window's origin,
// SystemPort.FocusedWindowBounds reports the rectangle, and the app watcher
// keys per-app configuration on the app id. All of them read the process-wide
// bridge returned by Shared.
//
// The shell loads a new extension only at login, so the bridge installs the
// extension files into the user's extensions directory when it finds the
// shell on the bus and the extension not, enables it, and says so once; the
// session it is installed into starts serving after the next login.
package gnomeshell
