//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"
	"github.com/y3owk1n/neru/internal/derrors"
)

// gnomeGeometry is the part of the bridge this source uses, named so the
// accept-or-reject rules can be tested without a GNOME Shell: the bridge is
// process-wide by design, and a test that pushed into it would push into every
// other test's cache too.
type gnomeGeometry interface {
	EnsureStarted()
	Focused() (gnomeshell.Window, bool, error)
}

// gnomeOriginSource offsets AT-SPI's window-relative coordinates by the frame
// the Neru GNOME Shell extension reports for the focused window.
type gnomeOriginSource struct {
	logger   *zap.Logger
	geometry gnomeGeometry
}

func newGNOMEOriginSource(logger *zap.Logger) *gnomeOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &gnomeOriginSource{
		logger:   logger.Named("accessibility.gnome"),
		geometry: gnomeshell.Shared(logger),
	}
}

func (s *gnomeOriginSource) start() { s.geometry.EnsureStarted() }

// originFor returns the cached origin only when the cached window is the
// window the frame describes. The frame's app_id and title came from the same
// bridge, so identity disagrees only when focus moved between the two reads,
// which is the race the check exists for; size is the check that catches a
// transient the shell reported and AT-SPI did not.
func (s *gnomeOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	// Asked on every activation: the shell can reload the extension, and a
	// source that only tried at startup would spend the rest of the session
	// placing hints window-relative. It never blocks.
	s.geometry.EnsureStarted()

	window, cached, err := s.geometry.Focused()
	if err != nil {
		return image.Point{}, false, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"no GNOME window-origin source",
		)
	}

	if !cached {
		return image.Point{}, false, nil
	}

	return gnomeOrigin(window, frame, s.logger)
}

// gnomeOrigin applies the identity and size checks to a cached window.
func gnomeOrigin(
	window gnomeshell.Window,
	frame windowFrame,
	logger *zap.Logger,
) (image.Point, bool, error) {
	if !gnomeAppMatches(window.AppID, frame.FocusedAppID) ||
		!kwinTitleMatches(window.Title, frame.FocusedTitle) {
		logger.Debug("GNOME origin rejected: cached window is not the AT-SPI frame's window")

		return image.Point{}, false, nil
	}

	rect := window.Rect
	if absInt(rect.Dx()-frame.Width) > windowOriginSizeTolerance ||
		absInt(rect.Dy()-frame.Height) > windowOriginSizeTolerance {
		logger.Debug("GNOME origin rejected: cached size does not match AT-SPI frame",
			zap.Int("cachedW", rect.Dx()), zap.Int("cachedH", rect.Dy()),
			zap.Int("frameW", frame.Width), zap.Int("frameH", frame.Height))

		return image.Point{}, false, nil
	}

	return rect.Min, true, nil
}

// gnomeAppMatches is the tolerant comparison frame selection already uses; an
// identity either side did not report is not a mismatch.
func gnomeAppMatches(windowAppID, frameAppID string) bool {
	if windowAppID == "" || frameAppID == "" {
		return true
	}

	return appMatchesFocusedID(windowAppID, frameAppID)
}
