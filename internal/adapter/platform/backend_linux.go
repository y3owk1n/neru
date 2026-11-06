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
	// BackendWaylandGNOME targets GNOME Shell (Mutter) on Wayland. Mutter
	// implements none of the wlr protocols beyond xdg-output, so the overlay
	// is an override-redirect window on Xwayland, pointer input goes through
	// libei and screen capture through the portal, the way KDE does. It is
	// served only while Xwayland is reachable (XwaylandAvailable).
	BackendWaylandGNOME
	// BackendWaylandKDE targets KDE Plasma Wayland (KWin exposes the same
	// wlr-style layer-shell and virtual-pointer protocols Neru uses on wlroots).
	BackendWaylandKDE
	// BackendWaylandCOSMIC targets System76 COSMIC (cosmic-comp on Smithay). It
	// implements layer-shell and the virtual keyboard like wlroots, but neither
	// the virtual pointer nor screencopy, so pointer input goes through libei
	// and screen capture through the portal, the way KDE does.
	BackendWaylandCOSMIC
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
	case BackendWaylandCOSMIC:
		return "wayland-cosmic"
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
	case BackendWaylandWlroots,
		BackendWaylandKDE,
		BackendWaylandCOSMIC,
		BackendWaylandGNOME,
		BackendWaylandOther:
		return true
	case BackendX11, BackendUnknown:
		return false
	default:
		return false
	}
}

// displayServer returns the display-stack label `neru info` and `neru doctor`
// report. It is derived from the backend, never re-read from the environment,
// because a second read once said "wayland" for an X11 backend (#1429).
func (b LinuxBackend) displayServer() DisplayServer {
	switch b {
	case BackendX11:
		return DisplayServerX11
	case BackendWaylandKDE:
		return DisplayServerWaylandKDE
	case BackendWaylandCOSMIC:
		return DisplayServerWaylandCOSMIC
	case BackendWaylandGNOME:
		return DisplayServerWaylandGNOME
	case BackendWaylandWlroots, BackendWaylandOther:
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
	cachedXwayland     bool
	cachedXwaylandOnce sync.Once
)

// resetLinuxBackendCache resets the cached backend detection result.
// This is only intended for use in tests that manipulate environment variables.
func resetLinuxBackendCache() {
	cachedBackendOnce = sync.Once{}
	cachedBackend = BackendUnknown
	cachedHyprlandOnce = sync.Once{}
	cachedHyprland = false
	cachedXwaylandOnce = sync.Once{}
	cachedXwayland = false
}

// detectLinuxBackend inspects the process environment and determines which
// Linux backend family Neru should target. The result is cached because the
// environment does not change at runtime and this runs on hot paths (cursor
// movement, clicks, scrolling).
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
// LinuxBackend deliberately does not carry this, because every subsystem
// dispatching on it wants the protocol family, not the compositor. A
// one-compositor quirk is named here because this file alone decides the
// compositor family (internal/adapter/platform/AGENTS.md).
//
// It reads XDG_CURRENT_DESKTOP, not HYPRLAND_INSTANCE_SIGNATURE, since the
// socket says which compositor is reachable, not which one this session runs
// (internal/architecture/compositor_socket_test.go). A Hyprland session
// exporting no XDG_CURRENT_DESKTOP reads as plain wlroots.
func IsHyprlandSession() bool {
	cachedHyprlandOnce.Do(func() {
		cachedHyprland = isHyprlandFromEnv(
			os.Getenv("XDG_CURRENT_DESKTOP"),
			os.Getenv("WAYLAND_DISPLAY"),
		)
	})

	return cachedHyprland
}

// XwaylandAvailable reports whether a Wayland session also exposes an X server
// (Xwayland), which the GNOME backend draws on. Mutter has no layer shell, and
// an override-redirect X window is the one surface it stacks above every
// toplevel without animating (docs/guide/linux-desktops.md). It lives here
// because DISPLAY decides the backend, and a second read elsewhere is the
// disagreement #1429 was (internal/architecture/compositor_detector_test.go).
func XwaylandAvailable() bool {
	cachedXwaylandOnce.Do(func() {
		cachedXwayland = os.Getenv("DISPLAY") != ""
	})

	return cachedXwayland
}

// isHyprlandFromEnv answers behind the backend. A desktop naming Hyprland on a
// session the detector did not call wlroots is a stale variable.
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
		case strings.Contains(desktop, "COSMIC"):
			return BackendWaylandCOSMIC
		case desktop == "":
			return BackendWaylandWlroots
		case strings.Contains(desktop, "SWAY"),
			strings.Contains(desktop, "HYPRLAND"),
			strings.Contains(desktop, "NIRI"),
			strings.Contains(desktop, "RIVER"),
			strings.Contains(desktop, "WAYFIRE"),
			strings.Contains(desktop, "LABWC"),
			// The "<name>:wlroots" tag is how a wlroots compositor tells
			// xdg-desktop-portal-wlr to serve it (labwc sets "labwc:wlroots"),
			// so it answers the same question for compositors not named here.
			strings.Contains(desktop, "WLROOTS"):
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
