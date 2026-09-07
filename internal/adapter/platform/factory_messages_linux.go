//go:build linux

package platform

import (
	"os"

	"github.com/y3owk1n/neru/internal/derrors"
)

func unsupportedLinuxBackendError(backend LinuxBackend) error {
	switch backend {
	case BackendWaylandGNOME:
		return derrors.New(
			derrors.CodeNotSupported,
			"neru on GNOME Wayland draws its overlay on Xwayland, and this session exposes no X server (DISPLAY is unset). Enable Xwayland in the GNOME session, or use a GNOME X11 session. See docs/LINUX_DESKTOPS.md.",
		)
	case BackendWaylandOther:
		return derrors.Newf(
			derrors.CodeNotSupported,
			"neru does not recognize this Wayland compositor (XDG_CURRENT_DESKTOP=%q). Supported Wayland compositors are wlroots-based ones such as Sway, Hyprland, niri, River, Wayfire and labwc (or any that tags XDG_CURRENT_DESKTOP with :wlroots), plus KDE Plasma and COSMIC. See docs/LINUX_SETUP.md.",
			os.Getenv("XDG_CURRENT_DESKTOP"),
		)
	case BackendUnknown:
		return derrors.New(
			derrors.CodeNotSupported,
			"neru could not detect a Linux display server. Ensure WAYLAND_DISPLAY or DISPLAY is set.",
		)
	case BackendX11, BackendWaylandWlroots, BackendWaylandKDE, BackendWaylandCOSMIC:
		return derrors.Newf(
			derrors.CodeInternal,
			"unsupportedLinuxBackendError called on supported backend: %s",
			backend.String(),
		)
	default:
		return derrors.Newf(
			derrors.CodeNotSupported,
			"unsupported linux backend: %s",
			backend.String(),
		)
	}
}
