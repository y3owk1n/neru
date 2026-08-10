//go:build linux

package atspi

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// Window-origin sources for Wayland. A Wayland client cannot know its own
// on-screen position, so AT-SPI reports element coordinates relative to the
// window. To turn those into true screen coordinates for the hint overlay, the
// focused window's screen origin is supplied by a compositor-specific source:
// KWin (KDE) pushes it over D-Bus; the wlroots family (niri, Sway, Hyprland)
// exposes it via their IPC. Every source is best-effort — when the origin
// cannot be determined, callers fall back to unoffset (window-relative)
// coordinates.
//
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

// noWindowOrigin is the source for a session no compositor bridge belongs on:
// X11, where AT-SPI already reports screen coordinates, and any Wayland
// compositor Neru has no geometry source for. It never reports an origin, so
// callers use AT-SPI's coordinates unchanged — and, unlike a bridge that merely
// fails to report one, it starts nothing.
//
// It logs on start because the bridge it replaces used to: on a compositor with
// no geometry source the KWin bridge warned that its script install failed,
// which was the wrong reason but did say hints would be unoffset here.
type noWindowOrigin struct {
	logger  *zap.Logger
	backend platform.LinuxBackend
}

func newNoWindowOrigin(logger *zap.Logger, backend platform.LinuxBackend) noWindowOrigin {
	if logger == nil {
		logger = zap.NewNop()
	}

	return noWindowOrigin{logger: logger.Named("accessibility.windoworigin"), backend: backend}
}

func (s noWindowOrigin) start() {
	s.logger.Debug(
		"No window-origin source for this backend; hint coordinates stay window-relative",
		zap.String("backend", s.backend.String()),
	)
}

func (noWindowOrigin) originFor(_, _ int) (int, int, bool) { return 0, 0, false }

// newWindowOriginSource selects the geometry source for the backend
// platform.DetectLinuxBackend identified. The backend decides first and the
// environment only refines it, because starting a source is not free: the KWin
// bridge opens the session bus, owns a name and writes a script into
// $XDG_RUNTIME_DIR, which it used to do on plain X11 sessions that set none of
// the wlroots sockets and fell through to it (#1430).
//
// The compositor sockets are read only once the backend has already answered
// "wlroots", and they answer a question LinuxBackend cannot: which of niri,
// Sway and Hyprland to query, where the backend has one value covering all of
// them plus River and Wayfire. A wlroots compositor with no source of its own
// reports no origin rather than borrowing KDE's.
func newWindowOriginSource(backend platform.LinuxBackend, logger *zap.Logger) windowOriginSource {
	switch backend {
	case platform.BackendWaylandKDE:
		return newKWinBridge(logger)
	case platform.BackendWaylandWlroots:
		return newWlrootsOriginSource(logger)
	case platform.BackendX11,
		platform.BackendWaylandGNOME,
		platform.BackendWaylandOther,
		platform.BackendUnknown:
		return newNoWindowOrigin(logger, backend)
	}

	return newNoWindowOrigin(logger, backend)
}

// newWlrootsOriginSource picks the wlroots-family source by the IPC socket its
// compositor exports, each of which is set only under that compositor.
func newWlrootsOriginSource(logger *zap.Logger) windowOriginSource {
	switch {
	case os.Getenv("NIRI_SOCKET") != "":
		return newNiriOriginSource(logger)
	case os.Getenv("SWAYSOCK") != "":
		return newSwayOriginSource(logger)
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		return newHyprlandOriginSource(logger)
	default:
		return newNoWindowOrigin(logger, platform.BackendWaylandWlroots)
	}
}
