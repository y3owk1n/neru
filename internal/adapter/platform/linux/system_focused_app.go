//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"

// FocusedAppID returns the focused application's identifier for the given
// backend (a platform.LinuxBackend.String() value), the value per-app config
// keys on. It is WM_CLASS on X11, the foreign-toplevel app_id on wlroots, KDE
// and COSMIC, and the extension-reported WM_CLASS on GNOME. ok is false on
// unknown backends, with nothing focused, without the GNOME extension, or
// without CGO.
func FocusedAppID(backend string) (string, bool) {
	switch backend {
	case backendX11:
		return x11FocusedAppID()
	case backendWaylandWlroots, backendWaylandKDE, backendWaylandCOSMIC:
		return WaylandFocusedAppID()
	case backendWaylandGNOME:
		appID, _, ok := gnomeFocusedAppIdentity()

		return appID, ok
	default:
		return "", false
	}
}

// FocusedAppIdentity returns the focused window's app_id and title as one
// snapshot, so they describe the same window. The title tells windows of one
// application apart. The bool is false where there is no live source.
func FocusedAppIdentity(backend string) (string, string, bool) {
	switch backend {
	case backendX11:
		return x11FocusedAppIdentity()
	case backendWaylandGNOME:
		return gnomeFocusedAppIdentity()
	default:
		return WaylandFocusedAppIdentity()
	}
}

// gnomeFocusedAppIdentity reads the extension's cache. EnsureStarted runs on
// every read because the shell can reload the extension.
func gnomeFocusedAppIdentity() (string, string, bool) {
	bridge := gnomeshell.Shared(nil)
	bridge.EnsureStarted()

	window, found, err := bridge.Focused()
	if err != nil || !found {
		return "", "", false
	}

	return window.AppID, window.Title, true
}
