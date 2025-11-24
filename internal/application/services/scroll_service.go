package services

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
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
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	config        config.ScrollConfig
	logger        *zap.Logger
}

// NewScrollService creates a new scroll service.
func NewScrollService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	config config.ScrollConfig,
	logger *zap.Logger,
) *ScrollService {
	return &ScrollService{
		accessibility: accessibility,
		overlay:       overlay,
		config:        config,
		logger:        logger,
	}
}

// Scroll performs a scrolling operation in the specified direction and magnitude.
func (s *ScrollService) Scroll(
	context context.Context,
	direction ScrollDirection,
	amount ScrollAmount,
) error {
	deltaX, deltaY := s.calculateDelta(direction, amount)

	s.logger.Debug("Scrolling",
		zap.Int("dir", int(direction)),
		zap.Int("amount", int(amount)),
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := s.accessibility.Scroll(context, deltaX, deltaY)
	if scrollErr != nil {
		return fmt.Errorf("failed to scroll: %w", scrollErr)
	}

	return nil
}

// ShowScrollOverlay displays the scroll overlay with a highlight.
func (s *ScrollService) ShowScrollOverlay(context context.Context) error {
	// Get screen screenBounds to draw highlight around active screen
	screenBounds, screenBoundsErr := s.accessibility.GetScreenBounds(context)
	if screenBoundsErr != nil {
		return fmt.Errorf("failed to get screen bounds: %w", screenBoundsErr)
	}

	// Draw highlight
	drawScrollHighlightErr := s.overlay.DrawScrollHighlight(
		context,
		screenBounds,
		s.config.HighlightColor,
		s.config.HighlightWidth,
	)
	if drawScrollHighlightErr != nil {
		return fmt.Errorf("failed to draw scroll highlight: %w", drawScrollHighlightErr)
	}

	return nil
}

// HideScrollOverlay hides the scroll overlay.
func (s *ScrollService) HideScrollOverlay(context context.Context) error {
	hideOverlayErr := s.overlay.Hide(context)
	if hideOverlayErr != nil {
		return fmt.Errorf("failed to hide overlay: %w", hideOverlayErr)
	}

	return nil
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
