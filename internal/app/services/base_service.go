package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// BaseService provides common functionality for services.
// It contains shared dependencies and methods used across different services.
type BaseService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	system        ports.SystemPort
}

// NewBaseService creates a new base service with the given dependencies.
func NewBaseService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
) BaseService {
	return BaseService{
		accessibility: accessibility,
		overlay:       overlay,
		system:        system,
	}
}

// Health checks the health of the service's dependencies.
// Returns a map of dependency names to their health status errors.
func (s *BaseService) Health(ctx context.Context) map[string]error {
	result := map[string]error{
		"accessibility": s.accessibility.Health(ctx),
		"overlay":       s.overlay.Health(ctx),
	}

	if s.system != nil {
		result["system"] = s.system.Health(ctx)
	}

	return result
}

// HideOverlay hides the overlay and returns any error that occurred.
// This is a helper method used by services that need to hide overlays.
func (s *BaseService) HideOverlay(ctx context.Context, operation string) error {
	err := s.overlay.Hide(ctx)
	if err != nil {
		return core.WrapOverlayFailed(err, operation)
	}

	return nil
}
