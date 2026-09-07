//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// cosmicOriginSource reads the focused window's origin from cosmic-comp's
// zcosmic_toplevel_info_v1, which the shared wlroots client already tracks for
// the focused app_id. No query leaves the process: the geometry is whatever the
// compositor last pushed.
type cosmicOriginSource struct {
	logger *zap.Logger
}

func newCosmicOriginSource(logger *zap.Logger) *cosmicOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &cosmicOriginSource{logger: logger.Named("accessibility.cosmic")}
}

func (s *cosmicOriginSource) start() {}

func (s *cosmicOriginSource) originFor(frame windowFrame) (image.Point, bool, error) {
	bounds, found, err := linux.WaylandFocusedWindowGeometry()
	if err != nil {
		return image.Point{}, false, err
	}

	if !found {
		return image.Point{}, false, nil
	}

	return cosmicOrigin(bounds, frame, s.logger)
}

// cosmicOrigin accepts the compositor's rectangle as the frame's origin only
// when the two describe a window of the same size, as the niri and Sway
// sources do: a focus change that raced the AT-SPI read would otherwise offset
// one window's hints by another's position.
func cosmicOrigin(
	bounds image.Rectangle,
	frame windowFrame,
	logger *zap.Logger,
) (image.Point, bool, error) {
	if absInt(bounds.Dx()-frame.Width) > windowOriginSizeTolerance ||
		absInt(bounds.Dy()-frame.Height) > windowOriginSizeTolerance {
		logger.Debug("cosmic origin rejected: window size does not match AT-SPI frame",
			zap.Int("windowW", bounds.Dx()), zap.Int("windowH", bounds.Dy()),
			zap.Int("frameW", frame.Width), zap.Int("frameH", frame.Height))

		return image.Point{}, false, nil
	}

	return bounds.Min, true, nil
}
