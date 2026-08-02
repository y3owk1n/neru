//go:build linux

// internal/adapter/platform/linux/system_linux_focus_events.go
// Event-driven focused-application change notifications for Linux. Complements
// FocusedAppID (which reports the current identity) by exposing a file
// descriptor that becomes readable whenever the focused app changes, so the app
// watcher can wake on focus changes instead of polling on a fixed interval.

package linux

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
	case backendWaylandWlroots, backendWaylandKDE:
		return wlrootsFocusEventFD()
	default:
		return -1, false
	}
}
