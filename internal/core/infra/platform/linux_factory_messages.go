package platform

import (
	"fmt"
	"os"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func unsupportedLinuxBackendError(backend LinuxBackend) error {
	switch backend {
	case BackendWaylandGNOME:
		return derrors.New(
			derrors.CodeNotSupported,
			"neru does not yet support GNOME Wayland. See docs/LINUX_SETUP.md and internal/core/infra/platform/linux/wayland_gnome/PLACEHOLDER.md.",
		)
	case BackendWaylandKDE:
		return derrors.New(
			derrors.CodeNotSupported,
			"neru does not yet support KDE Wayland. See docs/LINUX_SETUP.md and internal/core/infra/platform/linux/wayland_kde/PLACEHOLDER.md.",
		)
	case BackendWaylandOther:
		return derrors.Newf(
			derrors.CodeNotSupported,
			"neru does not recognise this Wayland compositor (XDG_CURRENT_DESKTOP=%q). Supported target backends are wlroots-based compositors such as Sway, Hyprland, niri, and River. See docs/LINUX_SETUP.md.",
			os.Getenv("XDG_CURRENT_DESKTOP"),
		)
	case BackendUnknown:
		return derrors.New(
			derrors.CodeNotSupported,
			"neru could not detect a Linux display server. Ensure WAYLAND_DISPLAY or DISPLAY is set.",
		)
	default:
		return fmt.Errorf("unsupported linux backend: %s", backend.String())
	}
}
