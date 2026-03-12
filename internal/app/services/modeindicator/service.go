package modeindicator

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// Service manages the mode indicator overlay.
type Service struct {
	system  ports.SystemPort
	overlay ports.OverlayPort
	logger  *zap.Logger
}

// NewService creates a new mode indicator service.
func NewService(
	system ports.SystemPort,
	overlay ports.OverlayPort,
	logger *zap.Logger,
) *Service {
	return &Service{
		system:  system,
		overlay: overlay,
		logger:  logger,
	}
}

// GetCursorPosition returns the current cursor position.
func (s *Service) GetCursorPosition(ctx context.Context) (int, int, error) {
	if s.system == nil {
		return 0, 0, derrors.New(derrors.CodeActionFailed, "system port not available")
	}

	point, err := s.system.CursorPosition(ctx)
	if err != nil {
		return 0, 0, core.WrapAccessibilityFailed(err, "get cursor position")
	}

	return point.X, point.Y, nil
}

// UpdateIndicatorPosition updates the mode indicator position.
func (s *Service) UpdateIndicatorPosition(x, y int) {
	s.overlay.DrawModeIndicator(x, y)
}
