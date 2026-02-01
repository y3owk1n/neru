package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// BaseService provides common functionality for services.
// It contains shared dependencies and methods used across different services.
type BaseService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
}

// NewBaseService creates a new base service with the given dependencies.
func NewBaseService(accessibility ports.AccessibilityPort, overlay ports.OverlayPort) BaseService {
	return BaseService{
		accessibility: accessibility,
		overlay:       overlay,
	}
}

// Health checks the health of the service's dependencies.
// Returns a map of dependency names to their health status errors.
func (s *BaseService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"accessibility": s.accessibility.Health(ctx),
		"overlay":       s.overlay.Health(ctx),
	}
}
