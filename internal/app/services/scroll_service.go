package services

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ScrollService orchestrates scrolling operations.
type ScrollService struct {
	BaseService

	logger *zap.Logger
}

// NewScrollService creates a new scroll service.
func NewScrollService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	logger *zap.Logger,
) *ScrollService {
	return &ScrollService{
		BaseService: NewBaseService(accessibility, overlay, system),
		logger:      logger,
	}
}

// ScrollDelta performs a scrolling operation with the given deltas.
// deltaX positive scrolls left, negative scrolls right.
// deltaY positive scrolls up, negative scrolls down.
func (s *ScrollService) ScrollDelta(
	ctx context.Context,
	deltaX, deltaY int,
) error {
	s.logger.Debug("Scrolling",
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := s.accessibility.Scroll(ctx, deltaX, deltaY)
	if scrollErr != nil {
		return core.WrapActionFailed(scrollErr, "scroll")
	}

	return nil
}

// Hide hides the scroll overlay.
func (s *ScrollService) Hide(ctx context.Context) error {
	return s.HideOverlay(ctx, "hide scroll")
}
