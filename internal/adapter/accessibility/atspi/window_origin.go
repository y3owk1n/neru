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
// position, so AT-SPI reports window-relative coordinates and a
// compositor-specific source supplies the focused window's screen origin. KWin
// pushes it over D-Bus. niri, Sway and Hyprland answer through their CLI via
// [github.com/y3owk1n/neru/internal/adapter/platform/compositorcli], the same
// query SystemPort.FocusedWindowBounds uses.
//
// Every source degrades to window-relative coordinates rather than refusing,
// but keeps a compositor with no origin to give (routine) apart from a query
// that never happened (a fault a person can fix).

// coordPairLen is the length of a [x,y] / [w,h] pair in compositor JSON.
const coordPairLen = 2

// reportOriginFailure warns that the focused window's position could not be
// read, and warns again only when the reason changes. Hints are about to land
// window-relative, a screenful in the wrong place, so it must be visible. But
// the cause is almost always a session fact and this runs every activation,
// so repeats go to debug.
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

// clearOriginFailure forgets the last reported reason, so a source that
// recovers and breaks again is heard again. Any answer clears it, even one
// with no origin.
func (c *Client) clearOriginFailure() { c.originFailure.Store(nil) }

// windowOriginSizeTolerance is the max per-dimension difference (px) between
// the compositor's window size and the AT-SPI frame before the origin is taken
// for a different window, since focus can change in between. Qt matches
// exactly, and GTK/CSD can differ by a title bar.
const windowOriginSizeTolerance = 32

// absInt is the magnitude every source compares against that tolerance with.
func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// windowFrame describes the AT-SPI frame an origin is asked for, so a source
// can check that its "focused window" is the one being walked.
type windowFrame struct {
	// Width and Height are the frame's AT-SPI extents, compared against the
	// compositor's size within windowOriginSizeTolerance.
	Width, Height int

	// FocusedAppID and FocusedTitle are the focused-toplevel identity read when
	// this frame was selected. Both are empty on X11 and GNOME, which lack the
	// protocol. A missing identity is not a mismatch, so a source falls through
	// to the size check rather than refuse.
	FocusedAppID string
	FocusedTitle string
}

// windowOriginSource supplies the focused window's on-screen origin so AT-SPI
// window-relative element coordinates can be offset into screen coordinates.
type windowOriginSource interface {
	// start performs any one-time setup. It is best-effort and logs failures.
	start()
	// originFor returns the focused window's screen origin, only when that
	// window is the one frame describes. ok=false with no error is the
	// compositor having no origin to give (nothing focused, another window's
	// rectangle, a tiled niri window). A non-nil error is the source failing to
	// answer, which callers must not read as the second. Both fall back to
	// unoffset coordinates, and only the error is something a person can fix.
	originFor(frame windowFrame) (origin image.Point, ok bool, err error)
}

// noWindowOrigin is the source for X11, where AT-SPI already reports screen
// coordinates, and for Wayland compositors with no geometry source. It never
// reports an origin and starts nothing. It logs on start so the session says
// hints stay window-relative.
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

// originFor answers differently for the two sessions that share this type. On
// X11 no origin is the correct, complete answer. On a Wayland compositor with
// no source, such as River or Wayfire, there is nobody to ask, which the
// platform contract says to report (internal/adapter/platform/AGENTS.md,
// "Stubs are loud"), as SystemPort.FocusedWindowBounds does. Hints land
// window-relative either way.
func (s noWindowOrigin) originFor(_ windowFrame) (image.Point, bool, error) {
	if s.backend == platform.BackendX11 {
		return image.Point{}, false, nil
	}

	return image.Point{}, false, derrors.New(
		derrors.CodeNotSupported,
		"no window-origin source on linux backend "+s.backend.String()+
			": this compositor exposes no way to ask where the focused window is",
	)
}

// newWindowOriginSource selects the geometry source for the detected backend.
// The backend decides first because starting a source is not free. The KWin
// bridge opens the session bus, owns a name and writes a script, which it once
// did on plain X11 sessions (#1430). Compositor sockets are read only for
// "wlroots", to pick among niri, Sway and Hyprland. A wlroots compositor with
// no source of its own reports no origin rather than borrowing KDE's.
func newWindowOriginSource(backend platform.LinuxBackend, logger *zap.Logger) windowOriginSource {
	switch backend {
	case platform.BackendWaylandKDE:
		return newKWinOriginSource(logger)
	case platform.BackendWaylandWlroots:
		return newWlrootsOriginSource(logger)
	case platform.BackendWaylandCOSMIC:
		return newCosmicOriginSource(logger)
	case platform.BackendWaylandGNOME:
		return newGNOMEOriginSource(logger)
	case platform.BackendX11,
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
