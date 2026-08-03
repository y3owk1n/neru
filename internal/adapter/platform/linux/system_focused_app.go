//go:build linux

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
