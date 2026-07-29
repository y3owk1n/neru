//go:build linux

// internal/core/infra/accessibility/window_origin_linux.go
// Window-origin sources for Wayland. A Wayland client cannot know its own
// on-screen position, so AT-SPI reports element coordinates relative to the
// window. To turn those into true screen coordinates for the hint overlay, the
// focused window's screen origin is supplied by a compositor-specific source:
// KWin (KDE) pushes it over D-Bus; the wlroots family (niri, Sway, Hyprland)
// exposes it via their IPC. Every source is best-effort — when the origin
// cannot be determined, callers fall back to unoffset (window-relative)
// coordinates.

package accessibility

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"go.uber.org/zap"
)

// compositorQueryTimeout bounds each compositor IPC call so a wedged
// compositor cannot stall hint activation.
const compositorQueryTimeout = 500 * time.Millisecond

// coordPairLen is the length of a [x,y] / [w,h] pair in compositor JSON.
const coordPairLen = 2

// compositorJSON runs a compositor CLI (name + args) and decodes its JSON
// stdout into dst. Returns false on spawn error, non-zero exit, timeout, or
// malformed JSON — callers then fall back to unoffset coordinates.
func compositorJSON(dst any, name string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), compositorQueryTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return false
	}

	return json.Unmarshal(out, dst) == nil
}

// windowOriginSizeTolerance is the max per-dimension difference (px) allowed
// between the compositor-reported window size and the AT-SPI frame extents
// before the reported origin is treated as belonging to a different window
// (focus can change between the AT-SPI walk and the geometry query). Qt apps
// match exactly; GTK/CSD apps can differ by a title bar's worth of pixels.
const windowOriginSizeTolerance = 32

// windowOriginSource supplies the focused window's on-screen origin so AT-SPI
// window-relative element coordinates can be offset into screen coordinates.
type windowOriginSource interface {
	// start performs any one-time setup. Best-effort; failures are logged.
	start()
	// originFor returns the focused window's screen origin, but only when its
	// size matches the given AT-SPI frame extents within
	// windowOriginSizeTolerance. ok is false when the origin is unknown or
	// stale, in which case callers use unoffset coordinates.
	originFor(frameW, frameH int) (x, y int, ok bool)
}

// newWindowOriginSource selects the geometry source for the running compositor.
// The wlroots-family IPC environment variables are checked first because they
// are only set under their respective compositor; otherwise the KWin bridge is
// used (KDE, or a harmless no-op that never reports an origin elsewhere).
func newWindowOriginSource(logger *zap.Logger) windowOriginSource {
	switch {
	case os.Getenv("NIRI_SOCKET") != "":
		return newNiriOriginSource(logger)
	case os.Getenv("SWAYSOCK") != "":
		return newSwayOriginSource(logger)
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		return newHyprlandOriginSource(logger)
	default:
		return newKWinBridge(logger)
	}
}
