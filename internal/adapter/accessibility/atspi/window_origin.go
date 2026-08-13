//go:build linux

package atspi

import (
	"image"
	"os"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Window-origin sources for Wayland. A Wayland client cannot know its own
// on-screen position, so AT-SPI reports element coordinates relative to the
// window. To turn those into true screen coordinates for the hint overlay, the
// focused window's screen origin is supplied by a compositor-specific source:
// KWin (KDE) pushes it over D-Bus; the wlroots family (niri, Sway, Hyprland)
// exposes it through their CLI, asked through
// [github.com/y3owk1n/neru/internal/adapter/platform/compositorcli] — the same
// query SystemPort.FocusedWindowBounds shells out with, because it is the same
// process answering the same question.
//
// Every source degrades rather than refuses: when no origin is available,
// callers fall back to unoffset (window-relative) coordinates. What a source
// must still separate is a compositor that answered and has no origin to give
// from a query that never happened, because the second is a fault a person can
// fix and the first is the ordinary case this fallback exists for.

// coordPairLen is the length of a [x,y] / [w,h] pair in compositor JSON.
const coordPairLen = 2

// reportOriginFailure says out loud that the focused window's position could
// not be read, and says it again only when the reason changes.
//
// The failure has to be visible: hints in this window are about to be drawn at
// window-relative coordinates, which on Wayland means a screenful of them in
// the wrong place, and until now that happened silently. But it is almost
// always a property of the session rather than of this activation — a
// compositor CLI that is not installed stays not installed — and this runs on
// every hint activation. So the reason is warned once and recorded at debug
// after that: repeating one fact hundreds of times a day is how a log stops
// being read, which costs the same as saying nothing.
func (c *Client) reportOriginFailure(err error) {
	reason := derrors.Message(err)

	if previous := c.originFailure.Load(); previous != nil && *previous == reason {
		c.logger.Debug("Focused window position still unavailable", zap.Error(err))

		return
	}

	c.originFailure.Store(&reason)

	c.logger.Warn(
		"Could not read the focused window's position; hints stay window-relative",
		zap.Error(err),
	)
}

// clearOriginFailure forgets the last reported reason, so a source that starts
// working and later breaks again is heard the second time too.
func (c *Client) clearOriginFailure() { c.originFailure.Store(nil) }

// windowOriginSizeTolerance is the max per-dimension difference (px) allowed
// between the compositor-reported window size and the AT-SPI frame extents
// before the reported origin is treated as belonging to a different window
// (focus can change between the AT-SPI walk and the geometry query). Qt apps
// match exactly; GTK/CSD apps can differ by a title bar's worth of pixels.
const windowOriginSizeTolerance = 32

// absInt is the magnitude every source compares against that tolerance with.
func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// windowOriginSource supplies the focused window's on-screen origin so AT-SPI
// window-relative element coordinates can be offset into screen coordinates.
type windowOriginSource interface {
	// start performs any one-time setup. Best-effort; failures are logged.
	start()
	// originFor returns the focused window's screen origin, but only when its
	// size matches the given AT-SPI frame extents within
	// windowOriginSizeTolerance.
	//
	// The three answers are distinct. An origin with ok is what the compositor
	// said. ok=false with no error is the compositor answering that it has no
	// origin to give — nothing focused, a stale cached rectangle, or a niri
	// window whose tiling gives it no on-screen position. A non-nil error is
	// the source failing to answer at all, which callers must not read as the
	// second: both fall back to unoffset coordinates, and only one of them is
	// something a person can fix.
	originFor(frameW, frameH int) (origin image.Point, ok bool, err error)
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

// originFor answers differently for the two sessions that share this type,
// because they are not the same fact.
//
// On X11 nothing failed and nothing is missing: AT-SPI already reports screen
// coordinates, so having no origin to add is the correct and complete answer.
// On a Wayland compositor Neru has no source for — GNOME, River, Wayfire — the
// fact is that there is nobody to ask, which the platform contract says to
// report rather than dress up as an answer (internal/adapter/platform/AGENTS.md,
// "Stubs are loud"). It is the same refusal SystemPort.FocusedWindowBounds
// gives such a session, for the same reason: hints land window-relative either
// way, and only one of the two says so.
func (s noWindowOrigin) originFor(_, _ int) (image.Point, bool, error) {
	if s.backend == platform.BackendX11 {
		return image.Point{}, false, nil
	}

	return image.Point{}, false, derrors.New(
		derrors.CodeNotSupported,
		"no window-origin source on linux backend "+s.backend.String()+
			": this compositor exposes no way to ask where the focused window is",
	)
}

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
		return newKWinOriginSource(logger)
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
