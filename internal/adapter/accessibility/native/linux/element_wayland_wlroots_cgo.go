//go:build linux && cgo

package linux

import (
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

func wlrootsFocusedApplicationIdentity() (string, int) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", 0
	}

	// wlr-foreign-toplevel (wlroots and KWin) reports the focused app_id, the
	// per-app config key, but no PID. A caller needing a PID falls back to
	// XWayland when DISPLAY is set.
	appID, ok := linux.FocusedAppID(platform.DetectLinuxBackend().String())
	if ok && appID != "" {
		return appID, 0
	}

	// Nothing focused yet, or the GNOME extension is not running.
	return "", 0
}

func wlrootsApplicationBundleIdentifier(pid int) string {
	_ = pid

	return ""
}
