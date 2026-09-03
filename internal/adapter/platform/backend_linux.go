//go:build linux

package platform

import (
	"os"
	"strings"
	"sync"
)

// LinuxBackend identifies the Linux runtime backend family Neru should target.
type LinuxBackend int

const (
	// BackendUnknown means no supported display backend could be detected.
	BackendUnknown LinuxBackend = iota
	// BackendX11 targets the classic X11 desktop stack.
	BackendX11
	// BackendWaylandWlroots targets wlroots-based compositors on Wayland.
	BackendWaylandWlroots
	// BackendWaylandGNOME targets GNOME Wayland, which is not implemented yet.
	BackendWaylandGNOME
	// BackendWaylandKDE targets KDE Plasma Wayland (KWin exposes the same
	// wlr-style layer-shell and virtual-pointer protocols Neru uses on wlroots).
	BackendWaylandKDE
	// BackendWaylandOther means a non-wlroots Wayland compositor was detected.
	BackendWaylandOther
)

const unknownBackendString = "unknown"

// String returns a stable backend label for logs and capability details.
func (b LinuxBackend) String() string {
	switch b {
	case BackendX11:
		return "x11"
	case BackendWaylandWlroots:
		return "wayland-wlroots"
	case BackendWaylandGNOME:
		return "wayland-gnome"
	case BackendWaylandKDE:
		return "wayland-kde"
	case BackendWaylandOther:
		return "wayland-other"
	case BackendUnknown:
		return unknownBackendString
	default:
		return unknownBackendString
	}
}

// IsWayland reports whether this backend runs under a Wayland compositor, the
// split every subsystem with two implementations dispatches on.
func (b LinuxBackend) IsWayland() bool {
	switch b {
	case BackendWaylandWlroots, BackendWaylandKDE, BackendWaylandGNOME, BackendWaylandOther:
		return true
	case BackendX11, BackendUnknown:
		return false
	default:
		return false
	}
}

// displayServer returns the display-stack label for this backend: what the
// daemon is driving, said in the vocabulary `neru info` and `neru doctor`
// report. It is derived rather than detected on purpose — a second read of the
// environment answered "wayland" for a session whose backend was X11 (#1429).
func (b LinuxBackend) displayServer() DisplayServer {
	switch b {
	case BackendX11:
		return DisplayServerX11
	case BackendWaylandKDE:
		return DisplayServerWaylandKDE
	case BackendWaylandWlroots, BackendWaylandGNOME, BackendWaylandOther:
		return DisplayServerWayland
	case BackendUnknown:
		return DisplayServerUnknown
	default:
		return DisplayServerUnknown
	}
}

var (
	cachedBackend      LinuxBackend
	cachedBackendOnce  sync.Once
	cachedHyprland     bool
	cachedHyprlandOnce sync.Once
)

// resetLinuxBackendCache resets the cached backend detection result.
// This is only intended for use in tests that manipulate environment variables.
func resetLinuxBackendCache() {
	cachedBackendOnce = sync.Once{}
	cachedBackend = BackendUnknown
	cachedHyprlandOnce = sync.Once{}
	cachedHyprland = false
}

// detectLinuxBackend inspects the process environment and determines which
// Linux backend family Neru should target. The result is cached because
// display-server environment variables do not change at runtime, and this
// function is called on hot paths (cursor movement, clicks, scrolling).
func detectLinuxBackend() LinuxBackend {
	cachedBackendOnce.Do(func() {
		cachedBackend = detectLinuxBackendFromEnv(
			os.Getenv("XDG_CURRENT_DESKTOP"),
			os.Getenv("WAYLAND_DISPLAY"),
			os.Getenv("DISPLAY"),
		)
	})

	return cachedBackend
}

// DetectLinuxBackend returns the detected Linux backend family for the current
// process environment.
func DetectLinuxBackend() LinuxBackend {
	return detectLinuxBackend()
}

// IsHyprlandSession reports whether the wlroots session this process runs under
// is Hyprland.
//
// LinuxBackend deliberately does not carry this. One value covers Sway,
// Hyprland, niri, River and Wayfire, because every subsystem dispatching on the
// backend wants the protocol family rather than the compositor's name, and
// splitting the enum would make each of them answer a question it does not ask.
// A quirk belonging to one compositor still has to be named somewhere, and it is
// named here rather than at the call site because the compositor family is
// decided in this file and nowhere else (internal/adapter/platform/AGENTS.md).
//
// It reads XDG_CURRENT_DESKTOP, the identity variable the backend is already
// detected from, and not HYPRLAND_INSTANCE_SIGNATURE: the socket says which
// compositor is reachable, not which one this session runs, and a unit that
// imported it into another session would answer yes
// (internal/architecture/compositor_socket_test.go). So a Hyprland session
// exporting no XDG_CURRENT_DESKTOP reads as plain wlroots here, which leaves it
// on the path every wlroots compositor took before this predicate existed.
func IsHyprlandSession() bool {
	cachedHyprlandOnce.Do(func() {
		cachedHyprland = isHyprlandFromEnv(
			os.Getenv("XDG_CURRENT_DESKTOP"),
			os.Getenv("WAYLAND_DISPLAY"),
		)
	})

	return cachedHyprland
}

// isHyprlandFromEnv answers behind the backend rather than beside it: a desktop
// naming Hyprland on a session the detector did not call wlroots is not a
// Hyprland session, it is a stale variable.
func isHyprlandFromEnv(currentDesktop string, waylandDisplay string) bool {
	if detectLinuxBackendFromEnv(currentDesktop, waylandDisplay, "") != BackendWaylandWlroots {
		return false
	}

	return strings.Contains(strings.ToUpper(currentDesktop), "HYPRLAND")
}

func detectLinuxBackendFromEnv(
	currentDesktop string,
	waylandDisplay string,
	xDisplay string,
) LinuxBackend {
	if waylandDisplay != "" {
		desktop := strings.ToUpper(currentDesktop)

		switch {
		case strings.Contains(desktop, "GNOME"):
			return BackendWaylandGNOME
		case strings.Contains(desktop, "KDE"):
			return BackendWaylandKDE
		case desktop == "":
			return BackendWaylandWlroots
		case strings.Contains(desktop, "SWAY"),
			strings.Contains(desktop, "HYPRLAND"),
			strings.Contains(desktop, "NIRI"),
			strings.Contains(desktop, "RIVER"),
			strings.Contains(desktop, "WAYFIRE"):
			return BackendWaylandWlroots
		default:
			return BackendWaylandOther
		}
	}

	if xDisplay != "" {
		return BackendX11
	}

	return BackendUnknown
}
