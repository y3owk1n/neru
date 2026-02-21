package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// ScrollDirection represents the direction of a scrolling operation.
type ScrollDirection int

const (
	// ScrollDirectionUp represents upward scrolling.
	ScrollDirectionUp ScrollDirection = iota
	// ScrollDirectionDown represents downward scrolling.
	ScrollDirectionDown
	// ScrollDirectionLeft represents leftward scrolling.
	ScrollDirectionLeft
	// ScrollDirectionRight represents rightward scrolling.
	ScrollDirectionRight
)

// ScrollAmount represents the magnitude of a scrolling operation.
type ScrollAmount int

const (
	// ScrollAmountChar represents character-level scrolling.
	ScrollAmountChar ScrollAmount = iota
	// ScrollAmountHalfPage represents half-page scrolling.
	ScrollAmountHalfPage
	// ScrollAmountEnd represents scrolling to the end.
	ScrollAmountEnd
)

// ScrollService orchestrates scrolling operations.
type ScrollService struct {
	BaseService

	config config.ScrollConfig
	logger *zap.Logger
}

// NewScrollService creates a new scroll service.
func NewScrollService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	config config.ScrollConfig,
	logger *zap.Logger,
) *ScrollService {
	return &ScrollService{
		BaseService: NewBaseService(accessibility, overlay),
		config:      config,
		logger:      logger,
	}
}

// Scroll performs a scrolling operation in the specified direction and magnitude.
func (s *ScrollService) Scroll(
	ctx context.Context,
	direction ScrollDirection,
	amount ScrollAmount,
) error {
	deltaX, deltaY := s.calculateDelta(direction, amount)

	s.logger.Debug("Scrolling",
		zap.Int("dir", int(direction)),
		zap.Int("amount", int(amount)),
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := s.accessibility.Scroll(ctx, deltaX, deltaY)
	if scrollErr != nil {
		return core.WrapActionFailed(scrollErr, "scroll")
	}

	return nil
}

// Show displays the scroll overlay with a highlight.
func (s *ScrollService) Show(ctx context.Context) error {
	// Show overlay window first
	s.overlay.Show()

	// Get cursor position to draw initial indicator
	point, err := s.accessibility.CursorPosition(ctx)
	if err != nil {
		s.logger.Warn("Failed to get cursor position for scroll indicator", zap.Error(err))
		// Fallback to center screen if cursor position fails?
		// For now, just don't draw the indicator if we can't find the cursor
		return nil
	}

	// Draw indicator
	s.overlay.DrawModeIndicator(point.X, point.Y)

	return nil
}

// Hide hides the scroll overlay.
func (s *ScrollService) Hide(ctx context.Context) error {
	return s.HideOverlay(ctx, "hide scroll")
}

// GetCursorPosition returns the current cursor position.
func (s *ScrollService) GetCursorPosition(ctx context.Context) (int, int, error) {
	point, err := s.accessibility.CursorPosition(ctx)
	if err != nil {
		return 0, 0, core.WrapAccessibilityFailed(err, "get cursor position")
	}

	return point.X, point.Y, nil
}

// UpdateIndicatorPosition updates the mode indicator position.
func (s *ScrollService) UpdateIndicatorPosition(x, y int) {
	s.overlay.DrawModeIndicator(x, y)
}

// UpdateConfig updates the scroll configuration.
// This allows changing scroll behavior at runtime.
func (s *ScrollService) UpdateConfig(_ context.Context, config config.ScrollConfig) {
	s.config = config
	s.logger.Info("Scroll configuration updated",
		zap.Int("scroll_step", config.ScrollStep),
		zap.Int("scroll_step_full", config.ScrollStepFull))
}

// calculateDelta computes the scroll delta values based on direction and magnitude.
func (s *ScrollService) calculateDelta(direction ScrollDirection, amount ScrollAmount) (int, int) {
	var (
		deltaX, deltaY int
		baseScroll     int
	)

	switch amount {
	case ScrollAmountChar:
		baseScroll = s.config.ScrollStep
	case ScrollAmountHalfPage:
		baseScroll = s.config.ScrollStepHalf
	case ScrollAmountEnd:
		baseScroll = s.config.ScrollStepFull
	}

	switch direction {
	case ScrollDirectionUp:
		deltaY = baseScroll
	case ScrollDirectionDown:
		deltaY = -baseScroll
	case ScrollDirectionLeft:
		deltaX = baseScroll
	case ScrollDirectionRight:
		deltaX = -baseScroll
	}

	return deltaX, deltaY
}
