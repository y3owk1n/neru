package modeindicator

import (
	"context"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Service manages the mode indicator overlay.
type Service struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	logger        *zap.Logger
}

// NewService creates a new mode indicator service.
func NewService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	logger *zap.Logger,
) *Service {
	return &Service{
		accessibility: accessibility,
		overlay:       overlay,
		logger:        logger,
	}
}

// GetCursorPosition returns the current cursor position.
func (s *Service) GetCursorPosition(ctx context.Context) (int, int, error) {
	point, err := s.accessibility.CursorPosition(ctx)
	if err != nil {
		return 0, 0, core.WrapAccessibilityFailed(err, "get cursor position")
	}

	return point.X, point.Y, nil
}

// UpdateIndicatorPosition updates the mode indicator position.
func (s *Service) UpdateIndicatorPosition(x, y int) {
	s.overlay.DrawModeIndicator(x, y)
}
