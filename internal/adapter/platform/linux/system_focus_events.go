//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"

// SubscribeFocusedApp returns a file descriptor that becomes readable whenever
// the focused application changes on the given backend (as produced by
// platform.LinuxBackend.String()). The app watcher blocks on this fd and
// re-queries FocusedAppID on each wake instead of polling.
//
// ok is false when the backend exposes no such fd — GNOME/Mutter (no
// focused-app source at all), unknown backends, or CGO-disabled builds — in
// which case callers must fall back to polling FocusedAppID. The fd is owned by
// the platform layer for the process lifetime; callers must poll it read-only
// and must not close it.
func SubscribeFocusedApp(backend string) (int, bool) {
	switch backend {
	case backendX11:
		return x11FocusEventFD()
	case backendWaylandWlroots, backendWaylandKDE, backendWaylandCOSMIC:
		return wlrootsFocusEventFD()
	case backendWaylandGNOME:
		return gnomeshell.Shared(nil).FocusEventFD()
	default:
		return -1, false
	}
}
