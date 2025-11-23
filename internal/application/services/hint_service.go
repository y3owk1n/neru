package services

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"go.uber.org/zap"
)

// HintService orchestrates hint generation and display.
// It coordinates between the accessibility system, hint generator, and overlay.
type HintService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	generator     hint.Generator
	logger        *zap.Logger
}

// NewHintService creates a new hint service with the given dependencies.
func NewHintService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	generator hint.Generator,
	logger *zap.Logger,
) *HintService {
	return &HintService{
		accessibility: accessibility,
		overlay:       overlay,
		generator:     generator,
		logger:        logger,
	}
}

// ShowHints displays hints for clickable elements on the screen.
func (s *HintService) ShowHints(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*hint.Hint, error) {
	s.logger.Info("Showing hints", zap.Any("filter", filter))

	// Get clickable elements
	elements, err := s.accessibility.GetClickableElements(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get clickable elements", zap.Error(err))
		return nil, fmt.Errorf("failed to get clickable elements: %w", err)
	}

	if len(elements) == 0 {
		s.logger.Info("No clickable elements found")
		return nil, nil
	}

	s.logger.Info("Found clickable elements", zap.Int("count", len(elements)))

	// Generate hints
	hints, err := s.generator.Generate(ctx, elements)
	if err != nil {
		s.logger.Error("Failed to generate hints", zap.Error(err))
		return nil, fmt.Errorf("failed to generate hints: %w", err)
	}

	s.logger.Info("Generated hints", zap.Int("count", len(hints)))

	// Display hints
	err = s.overlay.ShowHints(ctx, hints)
	if err != nil {
		s.logger.Error("Failed to show hints overlay", zap.Error(err))
		return nil, fmt.Errorf("failed to show hints: %w", err)
	}

	s.logger.Info("Hints displayed successfully")
	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Info("Hiding hints")

	err := s.overlay.Hide(ctx)
	if err != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(err))
		return fmt.Errorf("failed to hide overlay: %w", err)
	}

	s.logger.Info("Hints hidden successfully")
	return nil
}

// RefreshHints updates the hint display (e.g., after screen changes).
func (s *HintService) RefreshHints(ctx context.Context) error {
	s.logger.Info("Refreshing hints")

	if !s.overlay.IsVisible() {
		s.logger.Debug("Overlay not visible, skipping refresh")
		return nil
	}

	err := s.overlay.Refresh(ctx)
	if err != nil {
		s.logger.Error("Failed to refresh overlay", zap.Error(err))
		return fmt.Errorf("failed to refresh overlay: %w", err)
	}

	s.logger.Info("Hints refreshed successfully")
	return nil
}

// UpdateGenerator updates the hint generator.
// This allows changing the hint generation strategy at runtime.
func (s *HintService) UpdateGenerator(_ context.Context, generator hint.Generator) {
	if generator == nil {
		s.logger.Warn("Attempted to set nil generator, ignoring")
		return
	}
	s.generator = generator
	s.logger.Info("Hint generator updated")
}

// Health checks the health of the service's dependencies.
func (s *HintService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"accessibility": s.accessibility.Health(ctx),
		"overlay":       s.overlay.Health(ctx),
	}
}
