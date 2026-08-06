package virtualpointer

import (
	"github.com/y3owk1n/neru/internal/app/services/indicator"
	"github.com/y3owk1n/neru/internal/ports"
)

// Service owns the cursor-following virtual pointer for its whole life:
// whether it is on screen and where it is drawn.
//
// The virtual pointer stands in for the system cursor while the cursor is
// hidden, so it is the one indicator whose visibility follows cursor state
// rather than the active mode.
type Service struct {
	indicator.Base
}

// NewService creates a new virtual pointer service.
func NewService(
	system ports.SystemPort,
	overlay ports.OverlayPort,
) *Service {
	return &Service{
		Base: indicator.NewBase(ports.VirtualPointerIndicator, system, overlay),
	}
}

// UpdateIndicatorPosition draws the virtual pointer at the given position. Its
// size and color come from the overlay's own resolved Style.
func (s *Service) UpdateIndicatorPosition(posX, posY int) {
	overlay := s.Overlay()
	if overlay == nil {
		return
	}

	overlay.DrawVirtualPointer(posX, posY)
}
