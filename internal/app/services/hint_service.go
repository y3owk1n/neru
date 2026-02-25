package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// HintService orchestrates hint generation and display.
// It coordinates between the accessibility system, hint generator, and overlay.
type HintService struct {
	BaseService

	generator hint.Generator
	config    config.HintsConfig
	logger    *zap.Logger
}

// NewHintService creates a new hint service with the given dependencies.
func NewHintService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	generator hint.Generator,
	config config.HintsConfig,
	logger *zap.Logger,
) *HintService {
	return &HintService{
		BaseService: NewBaseService(accessibility, overlay),
		generator:   generator,
		config:      config,
		logger:      logger,
	}
}

// ShowHints displays hints for clickable elements on the screen.
func (s *HintService) ShowHints(
	ctx context.Context,
) ([]*hint.Interface, error) {
	s.logger.Info("Showing hints")

	filter := ports.DefaultElementFilter()

	// Populate filter with configuration
	filter.IncludeMenubar = s.config.IncludeMenubarHints
	filter.AdditionalMenubarTargets = s.config.AdditionalMenubarHintsTargets
	filter.IncludeDock = s.config.IncludeDockHints
	filter.IncludeNotificationCenter = s.config.IncludeNCHints
	filter.IncludeStageManager = s.config.IncludeStageManagerHints

	// Get clickable elements
	elements, elementsErr := s.accessibility.ClickableElements(ctx, filter)
	if elementsErr != nil {
		s.logger.Error("Failed to get clickable elements", zap.Error(elementsErr))

		return nil, core.WrapAccessibilityFailed(elementsErr, "get clickable elements")
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

		return nil, core.WrapInternalFailed(elementsErr, "generate hints")
	}

	s.logger.Info("Generated hints", zap.Int("count", len(hints)))

	// Display hints
	showHintsErr := s.overlay.ShowHints(ctx, hints)
	if showHintsErr != nil {
		s.logger.Error("Failed to show hints overlay", zap.Error(showHintsErr))

		return nil, core.WrapOverlayFailed(showHintsErr, "show hints")
	}

	s.logger.Info("Hints displayed successfully")

	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Info("Hiding hints")

	err := s.HideOverlay(ctx, "hide hints")
	if err != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(err))

		return err
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

		return core.WrapOverlayFailed(refreshOverlayErr, "refresh hints")
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
