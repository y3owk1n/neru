package services

import (
	"context"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// GridService orchestrates grid navigation.
type GridService struct {
	overlay ports.OverlayPort
	logger  *zap.Logger
}

// NewGridService creates a new grid service.
func NewGridService(overlay ports.OverlayPort, logger *zap.Logger) *GridService {
	return &GridService{
		overlay: overlay,
		logger:  logger,
	}
}

// ShowGrid displays the grid overlay.
func (s *GridService) ShowGrid(ctx context.Context, rows, cols int) error {
	s.logger.Info("Showing grid", zap.Int("rows", rows), zap.Int("cols", cols))

	// Show grid overlay
	showGridErr := s.overlay.ShowGrid(ctx, rows, cols)
	if showGridErr != nil {
		s.logger.Error("Failed to show grid overlay", zap.Error(showGridErr))

		return derrors.Wrap(showGridErr, derrors.CodeOverlayFailed, "failed to show grid overlay")
	}

	s.logger.Info("Grid displayed successfully")

	return nil
}

// HideGrid hides the grid overlay.
func (s *GridService) HideGrid(ctx context.Context) error {
	s.logger.Info("Hiding grid")

	hideGridErr := s.overlay.Hide(ctx)
	if hideGridErr != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(hideGridErr))

		return derrors.Wrap(hideGridErr, derrors.CodeOverlayFailed, "failed to hide overlay")
	}

	return nil
}
