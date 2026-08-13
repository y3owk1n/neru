//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// KDE window-origin source. KWin exposes no CLI the way the wlroots family
// does, so the geometry comes from the KWin script
// [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs — the same
// bridge SystemPort.FocusedWindowBounds reads, because it is the same fact.
// This file is only the AT-SPI half of it: turning the cached rectangle into
// the origin that offsets window-relative element coordinates.
type kwinOriginSource struct {
	logger   *zap.Logger
	geometry *kwin.Geometry
}

func newKWinOriginSource(logger *zap.Logger) *kwinOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &kwinOriginSource{
		logger:   logger.Named("accessibility.kwin"),
		geometry: kwin.Shared(logger),
	}
}

func (s *kwinOriginSource) start() { s.geometry.EnsureStarted() }

// originFor returns the cached origin only when the cached client size matches
// the given AT-SPI frame size within windowOriginSizeTolerance. The cache is fed
// by KWin focus events; if KWin missed a transition (or deliberately ignored a
// transient surface that became the active AT-SPI frame) the cached origin
// belongs to a different window, and offsetting by it would land every hint at
// the previous window's screen position. A size mismatch is the cheapest
// reliable staleness signal, so callers then fall back to unoffset
// (window-relative) coordinates.
//
// A bridge that could not be installed is reported rather than folded into that
// same answer. The degradation is identical — an unoffset hint either way — but
// one of them is a KDE session that will never place a hint correctly until
// something is fixed, and telling a person that is the whole reason this source
// carries a reason at all.
//
// It is reported in the same words the focused-window arm uses for the same
// failure, because it is the same bridge: CodeNotSupported, because what a
// person has to do about it is not check a permission but install a script that
// is missing. One geometry source answering one way here and another there is
// what having one geometry source is for.
func (s *kwinOriginSource) originFor(frameW, frameH int) (image.Point, bool, error) {
	rect, cached, err := s.geometry.Bounds()
	if err != nil {
		return image.Point{}, false, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"the KWin focused-window geometry script is not installed",
		)
	}

	if !cached {
		return image.Point{}, false, nil
	}

	if absInt(rect.Dx()-frameW) > windowOriginSizeTolerance ||
		absInt(rect.Dy()-frameH) > windowOriginSizeTolerance {
		s.logger.Debug("KWin origin rejected: cached size does not match AT-SPI frame",
			zap.Int("cachedW", rect.Dx()), zap.Int("cachedH", rect.Dy()),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH))

		return image.Point{}, false, nil
	}

	return rect.Min, true, nil
}
