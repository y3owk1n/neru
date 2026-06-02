package services

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
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

	mu     sync.RWMutex
	config config.ScrollConfig
	logger *zap.Logger
}

// NewScrollService creates a new scroll service.
func NewScrollService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	config config.ScrollConfig,
	logger *zap.Logger,
) *ScrollService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ScrollService{
		BaseService: NewBaseService(accessibility, overlay, system),
		config:      config,
		logger:      logger.Named("service.scroll"),
	}
}

// Scroll performs a scrolling operation in the specified direction and magnitude.
// If stepOverride is > 0, it overrides the configured scroll step for this invocation.
func (s *ScrollService) Scroll(
	ctx context.Context,
	direction ScrollDirection,
	amount ScrollAmount,
	stepOverride int,
) error {
	deltaX, deltaY := s.calculateDelta(ctx, direction, amount, stepOverride)

	s.logger.Debug("Scrolling",
		zap.Int("dir", int(direction)),
		zap.Int("amount", int(amount)),
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := s.accessibility.Scroll(ctx, deltaX, deltaY)
	if scrollErr != nil {
		return derrors.WrapActionFailed(scrollErr, "scroll")
	}

	return nil
}

// Hide hides the scroll overlay.
func (s *ScrollService) Hide(ctx context.Context) error {
	return s.HideOverlay(ctx, "hide scroll")
}

// UpdateConfig updates the scroll configuration.
// This allows changing scroll behavior at runtime.
func (s *ScrollService) UpdateConfig(config config.ScrollConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	s.logger.Debug("Scroll configuration updated",
		zap.Int("scroll_step", config.ScrollStep),
		zap.Int("scroll_step_full", config.ScrollStepFull))
}

// IsScrollInverted returns whether scroll direction inversion is currently enabled.
func (s *ScrollService) IsScrollInverted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.InvertScroll
}

// SetInvertScroll sets the scroll direction inversion state.
func (s *ScrollService) SetInvertScroll(inverted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.InvertScroll = inverted
	s.logger.Debug("Scroll invert set", zap.Bool("invert_scroll", s.config.InvertScroll))
}

// calculateDelta computes the scroll delta values based on direction and magnitude.
// If stepOverride is > 0, it takes precedence over the configured value.
func (s *ScrollService) calculateDelta(
	ctx context.Context,
	direction ScrollDirection,
	amount ScrollAmount,
	stepOverride int,
) (int, int) {
	var (
		deltaX, deltaY int
		baseScroll     int
		invertScroll   bool
	)

	if stepOverride > 0 {
		baseScroll = stepOverride
	} else {
		// Snapshot config under lock, then release before IPC call
		s.mu.RLock()
		scrollStep := s.config.ScrollStep
		scrollStepHalf := s.config.ScrollStepHalf
		scrollStepFull := s.config.ScrollStepFull
		invertScroll = s.config.InvertScroll
		configSnapshot := s.config
		s.mu.RUnlock()

		// Only perform IPC lookup if there are app-specific overrides configured
		if len(configSnapshot.AppConfigs) > 0 {
			bundleID, err := s.accessibility.FocusedAppBundleID(ctx)

			if err == nil && bundleID != "" {
				if appConfig := configSnapshot.AppConfigForBundleID(bundleID); appConfig != nil {
					if appConfig.ScrollStep != nil {
						scrollStep = *appConfig.ScrollStep
					}

					if appConfig.ScrollStepHalf != nil {
						scrollStepHalf = *appConfig.ScrollStepHalf
					}

					if appConfig.ScrollStepFull != nil {
						scrollStepFull = *appConfig.ScrollStepFull
					}
				}
			}
		}

		switch amount {
		case ScrollAmountChar:
			baseScroll = scrollStep
		case ScrollAmountHalfPage:
			baseScroll = scrollStepHalf
		case ScrollAmountEnd:
			baseScroll = scrollStepFull
		}
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

	if invertScroll {
		deltaX = -deltaX
		deltaY = -deltaY
	}

	return deltaX, deltaY
}
