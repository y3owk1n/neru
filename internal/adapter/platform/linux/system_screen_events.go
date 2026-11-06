//go:build linux

package linux

// SubscribeScreenChange returns an fd that becomes readable when the display
// configuration changes on the given backend (RandR on X11, wl_output
// add/remove on Wayland). ok is false when there is no such fd, and the overlay
// still follows the cursor. The platform layer owns the fd for the process
// lifetime, so callers poll it read-only and never close it.
func SubscribeScreenChange(backend string) (int, bool) {
	switch backend {
	case backendX11:
		return x11ScreenEventFD()
	case backendWaylandWlroots, backendWaylandKDE, backendWaylandCOSMIC, backendWaylandGNOME:
		return wlrootsScreenEventFD()
	default:
		return -1, false
	}
}

// RefreshScreens re-reads the display layout into the wlroots client's screen
// cache after a screen-change event. It is a no-op elsewhere, since X11
// enumerates live.
func RefreshScreens(backend string) {
	switch backend {
	case backendWaylandWlroots, backendWaylandKDE, backendWaylandCOSMIC, backendWaylandGNOME:
		wlrootsRefreshScreens()
	}
}
