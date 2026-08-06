package modeindicator

import (
	"github.com/y3owk1n/neru/internal/app/services/indicator"
	"github.com/y3owk1n/neru/internal/ports"
)

// Service owns the mode indicator for its whole life: whether it is on screen,
// how big its surface is, and where it is drawn.
type Service struct {
	indicator.Base
}

// NewService creates a new mode indicator service.
func NewService(
	system ports.SystemPort,
	overlay ports.OverlayPort,
) *Service {
	return &Service{
		Base: indicator.NewBase(ports.ModeIndicator, system, overlay),
	}
}

// UpdateIndicatorPosition draws the mode indicator at the given position.
func (s *Service) UpdateIndicatorPosition(posX, posY int) {
	overlay := s.Overlay()
	if overlay == nil {
		return
	}

	overlay.DrawModeIndicator(posX, posY)
}
