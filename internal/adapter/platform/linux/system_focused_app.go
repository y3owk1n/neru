//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"

// FocusedAppID returns the focused application's identifier for the given
// backend (as produced by platform.LinuxBackend.String()) and whether one is
// available. The identifier is the WM_CLASS on X11 and the app_id on Wayland,
// the same value per-app configuration is keyed on: the foreign-toplevel
// protocol's on wlroots, KDE and COSMIC, and the Neru GNOME Shell extension's
// on GNOME, where Mutter offers no protocol and the extension reports Mutter's
// own WM_CLASS for the window instead. It is empty (ok == false) on unknown
// backends, when nothing is focused yet, when the GNOME extension is not
// running, or on CGO-disabled builds.
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

// FocusedAppIdentity returns the focused window's app_id and title together,
// read as one snapshot so they describe the same window, for the given
// backend. The title disambiguates multiple windows of one application. On
// X11 the pair is the active window's WM_CLASS and _NET_WM_NAME, read over one
// connection; on Wayland it is the toplevel's app_id and title. The bool is
// false where there is no live source: an unknown backend, nothing focused,
// the GNOME extension not running, or a CGO-disabled build on X11.
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

// gnomeFocusedAppIdentity reads the extension's cache. EnsureStarted is asked
// on every read for the reason the origin source asks on every activation:
// the shell can reload the extension, and a reader that only tried at startup
// would spend the rest of the session with nothing.
func gnomeFocusedAppIdentity() (string, string, bool) {
	bridge := gnomeshell.Shared(nil)
	bridge.EnsureStarted()

	window, found, err := bridge.Focused()
	if err != nil || !found {
		return "", "", false
	}

	return window.AppID, window.Title, true
}
