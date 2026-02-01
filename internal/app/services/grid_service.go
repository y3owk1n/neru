package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// GridService orchestrates grid navigation.
type GridService struct {
	BaseService

	logger *zap.Logger
}

// NewGridService creates a new grid service.
func NewGridService(overlay ports.OverlayPort, logger *zap.Logger) *GridService {
	return &GridService{
		BaseService: NewBaseService(nil, overlay),
		logger:      logger,
	}
}

// ShowGrid displays the grid overlay.
func (s *GridService) ShowGrid(ctx context.Context) error {
	s.logger.Info("Showing grid")

	// Show grid overlay
	showGridErr := s.overlay.ShowGrid(ctx)
	if showGridErr != nil {
		s.logger.Error("Failed to show grid overlay", zap.Error(showGridErr))

		return core.WrapOverlayFailed(showGridErr, "show grid")
	}

	s.logger.Info("Grid displayed successfully")

	return nil
}

// HideGrid hides the grid overlay.
func (s *GridService) HideGrid(ctx context.Context) error {
	s.logger.Info("Hiding grid")

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
