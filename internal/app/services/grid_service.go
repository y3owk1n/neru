package services

import (
	"context"

	"github.com/y3owk1n/neru/internal/ports"
)

// GridService reports the health of the overlay that grid mode draws through.
// Grid drawing itself runs in the mode handler, not here, so this deliberately
// does not embed BaseService — there is no accessibility or system dependency
// to hold, and a nil one would panic through a promoted method.
type GridService struct {
	overlay ports.OverlayPort
}

// NewGridService creates a new grid service.
func NewGridService(overlay ports.OverlayPort) *GridService {
	return &GridService{overlay: overlay}
}

// Health checks the health of the service's dependencies.
func (s *GridService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"overlay": s.overlay.Health(ctx),
	}
}
