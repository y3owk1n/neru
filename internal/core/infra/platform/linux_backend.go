package platform

import (
	"os"
	"strings"
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
	// BackendWaylandKDE targets KDE Plasma Wayland, which is not implemented yet.
	BackendWaylandKDE
	// BackendWaylandOther means a non-wlroots Wayland compositor was detected.
	BackendWaylandOther
)

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
	default:
		return "unknown"
	}
}

// detectLinuxBackend inspects the process environment and determines which
// Linux backend family Neru should target.
func detectLinuxBackend() LinuxBackend {
	return detectLinuxBackendFromEnv(
		os.Getenv("XDG_CURRENT_DESKTOP"),
		os.Getenv("WAYLAND_DISPLAY"),
		os.Getenv("DISPLAY"),
	)
}

// DetectLinuxBackend returns the detected Linux backend family for the current
// process environment.
func DetectLinuxBackend() LinuxBackend {
	return detectLinuxBackend()
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
