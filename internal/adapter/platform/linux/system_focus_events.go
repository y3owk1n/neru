//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"

// SubscribeFocusedApp returns an fd that becomes readable when the focused
// application changes on the given backend. ok is false when there is no such
// fd, and callers poll FocusedAppID instead. The platform layer owns the fd for
// the process lifetime, so callers poll it read-only and never close it.
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
