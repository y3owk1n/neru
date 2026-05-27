package services

import (
	"context"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// GridService orchestrates grid navigation.
type GridService struct {
	BaseService

	logger *zap.Logger
}

// NewGridService creates a new grid service.
func NewGridService(
	overlay ports.OverlayPort,
	system ports.SystemPort,
	logger *zap.Logger,
) *GridService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &GridService{
		BaseService: NewBaseService(nil, overlay, system),
		logger:      logger.Named("service.grid"),
	}
}

// ShowGrid displays the grid overlay.
func (s *GridService) ShowGrid(ctx context.Context) error {
	s.logger.Debug("Showing grid")

	// Show grid overlay
	showGridErr := s.overlay.ShowGrid(ctx)
	if showGridErr != nil {
		s.logger.Error("Failed to show grid overlay", zap.Error(showGridErr))

		return derrors.WrapOverlayFailed(showGridErr, "show grid")
	}

	s.logger.Debug("Grid displayed successfully")

	return nil
}

// HideGrid hides the grid overlay.
func (s *GridService) HideGrid(ctx context.Context) error {
	s.logger.Debug("Hiding grid")

	err := s.HideOverlay(ctx, "hide grid")
	if err != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(err))

		return err
	}

	return nil
}

// Health checks the health of the service's dependencies.
func (s *GridService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"overlay": s.overlay.Health(ctx),
	}
}
