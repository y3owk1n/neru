//go:build linux

// internal/adapter/platform/linux/system_linux_screen_events.go
// Event-driven display-configuration change notifications for Linux. Exposes a
// file descriptor that becomes readable whenever the monitor layout changes
// (outputs added/removed/resized/moved) so the app watcher can wake and
// regenerate overlays for the new layout instead of never noticing hotplug.
// Mirrors system_linux_focus_events.go's SubscribeFocusedApp.

package linux

// SubscribeScreenChange returns a file descriptor that becomes readable whenever
// the display configuration changes on the given backend (as produced by
// platform.LinuxBackend.String()): RandR RRScreenChangeNotify on X11, wl_output
// registry add/remove on the wlroots/KWin Wayland stack.
//
// ok is false when the backend exposes no such fd — GNOME/Mutter, unknown
// backends, RandR-less X servers, or CGO-disabled builds — in which case there
// are simply no live hotplug events (the overlay still follows the cursor). The
// fd is owned by the platform layer for the process lifetime; callers must poll
// it read-only and must not close it.
func SubscribeScreenChange(backend string) (int, bool) {
	switch backend {
	case backendX11:
		return x11ScreenEventFD()
	case backendWaylandWlroots, backendWaylandKDE:
		return wlrootsScreenEventFD()
	default:
		return -1, false
	}
}

// RefreshScreens re-reads the display layout into any Go-side cache after a
// screen-change event. X11 enumerates live on every ScreenBounds call, so this
// is a Wayland-only concern (the wlroots client caches its screen list); it is a
// no-op on other backends.
func RefreshScreens(backend string) {
	switch backend {
	case backendWaylandWlroots, backendWaylandKDE:
		wlrootsRefreshScreens()
	}
}
