//go:build linux

package atspi

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
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
// (window-relative) coordinates — which is also what a bridge that never
// installed gets, since an unoffset hint is the same degradation either way.
func (s *kwinOriginSource) originFor(frameW, frameH int) (int, int, bool) {
	rect, ok, err := s.geometry.Bounds()
	if !ok {
		if err != nil {
			s.logger.Debug("KWin origin unavailable", zap.Error(err))
		}

		return 0, 0, false
	}

	if absInt(rect.Dx()-frameW) > windowOriginSizeTolerance ||
		absInt(rect.Dy()-frameH) > windowOriginSizeTolerance {
		s.logger.Debug("KWin origin rejected: cached size does not match AT-SPI frame",
			zap.Int("cachedW", rect.Dx()), zap.Int("cachedH", rect.Dy()),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH))

		return 0, 0, false
	}

	return rect.Min.X, rect.Min.Y, true
}
