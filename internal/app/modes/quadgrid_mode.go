package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// QuadGridMode implements the Mode interface for quad-grid navigation.
type QuadGridMode struct {
	*GenericMode
}

// NewQuadGridMode creates a new quad-grid mode instance.
func NewQuadGridMode(handler *Handler) *QuadGridMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *Handler, action *string) {
			handler.activateQuadGridModeWithAction(action)
		},
		HandleKeyFunc: func(handler *Handler, key string) {
			handler.handleQuadGridKey(key)
		},
		ExitFunc: func(handler *Handler) {
			handler.cleanupQuadGridMode()
		},
	}

	return &QuadGridMode{
		GenericMode: NewGenericMode(handler, domain.ModeQuadGrid, "QuadGridMode", behavior),
	}
}
