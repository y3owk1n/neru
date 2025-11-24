package services

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/application/ports"
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
func (s *GridService) ShowGrid(context context.Context, rows, cols int) error {
	s.logger.Info("Showing grid", zap.Int("rows", rows), zap.Int("cols", cols))

	// Show grid overlay
	showGridErr := s.overlay.ShowGrid(context, rows, cols)
	if showGridErr != nil {
		s.logger.Error("Failed to show grid overlay", zap.Error(showGridErr))

		return fmt.Errorf("failed to show grid overlay: %w", showGridErr)
	}

	s.logger.Info("Grid displayed successfully")

	return nil
}

// HideGrid hides the grid overlay.
func (s *GridService) HideGrid(context context.Context) error {
	s.logger.Info("Hiding grid")

	hideGridErr := s.overlay.Hide(context)
	if hideGridErr != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(hideGridErr))

		return fmt.Errorf("failed to hide overlay: %w", hideGridErr)
	}

	return nil
}
