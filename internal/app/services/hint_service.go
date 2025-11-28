package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
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
) ([]*hint.Interface, error) {
	s.logger.Info("Showing hints", zap.Any("filter", filter))

	// Get clickable elements
	elements, elementsErr := s.accessibility.ClickableElements(ctx, filter)
	if elementsErr != nil {
		s.logger.Error("Failed to get clickable elements", zap.Error(elementsErr))

		return nil, derrors.Wrap(
			elementsErr,
			derrors.CodeAccessibilityFailed,
			"failed to get clickable elements",
		)
	}

	if len(elements) == 0 {
		s.logger.Info("No clickable elements found")

		return nil, nil
	}

	s.logger.Info("Found clickable elements", zap.Int("count", len(elements)))

	// Generate hints
	hints, elementsErr := s.generator.Generate(ctx, elements)
	if elementsErr != nil {
		s.logger.Error("Failed to generate hints", zap.Error(elementsErr))

		return nil, derrors.Wrap(
			elementsErr,
			derrors.CodeHintGenerationFailed,
			"failed to generate hints",
		)
	}

	s.logger.Info("Generated hints", zap.Int("count", len(hints)))

	// Display hints
	showHintsErr := s.overlay.ShowHints(ctx, hints)
	if showHintsErr != nil {
		s.logger.Error("Failed to show hints overlay", zap.Error(showHintsErr))

		return nil, derrors.Wrap(showHintsErr, derrors.CodeOverlayFailed, "failed to show hints")
	}

	s.logger.Info("Hints displayed successfully")

	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Info("Hiding hints")

	hideOverlayErr := s.overlay.Hide(ctx)
	if hideOverlayErr != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(hideOverlayErr))

		return derrors.Wrap(hideOverlayErr, derrors.CodeOverlayFailed, "failed to hide overlay")
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

	refreshOverlayErr := s.overlay.Refresh(ctx)
	if refreshOverlayErr != nil {
		s.logger.Error("Failed to refresh overlay", zap.Error(refreshOverlayErr))

		return derrors.Wrap(
			refreshOverlayErr,
			derrors.CodeOverlayFailed,
			"failed to refresh overlay",
		)
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
