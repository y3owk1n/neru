//go:build linux

// internal/adapter/platform/linux/system_linux_focused_app.go
// Focused-application identity for Linux, used by the app watcher to detect
// foreground-app changes and by per-app configuration lookups. Returns the
// app_id (Wayland wlr-foreign-toplevel) or WM_CLASS (X11) that Neru keys
// per-app config on. GNOME/Mutter and unknown backends report no identity.

package linux

// FocusedAppID returns the focused application's identifier for the given
// backend (as produced by platform.LinuxBackend.String()) and whether one is
// available. The identifier is the WM_CLASS on X11 and the app_id on Wayland
// wlroots/KDE — the same value per-app configuration is keyed on. It is empty
// (ok == false) on GNOME/Mutter (no wlr-foreign-toplevel manager), on unknown
// backends, when nothing is focused yet, or on CGO-disabled builds.
func FocusedAppID(backend string) (string, bool) {
	switch backend {
	case backendX11:
		return x11FocusedAppID()
	case backendWaylandWlroots, backendWaylandKDE:
		return WaylandFocusedAppID()
	default:
		return "", false
	}
}
