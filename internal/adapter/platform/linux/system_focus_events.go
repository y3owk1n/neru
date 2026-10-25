//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"

// SubscribeFocusedApp returns a file descriptor that becomes readable whenever
// the focused application changes on the given backend (as produced by
// platform.LinuxBackend.String()). The app watcher blocks on this fd and
// re-queries FocusedAppID on each wake instead of polling.
//
// ok is false when the backend exposes no such fd, as on unknown backends or
// CGO-disabled builds, and callers then fall back to polling FocusedAppID. The
// platform layer owns the fd for the process lifetime. Callers poll it
// read-only and never close it.
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
