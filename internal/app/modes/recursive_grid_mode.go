package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// RecursiveGridMode implements the Mode interface for recursive-grid navigation.
type RecursiveGridMode struct {
	*GenericMode
}

// NewRecursiveGridMode creates a new recursive-grid mode instance.
func NewRecursiveGridMode(handler *Handler) *RecursiveGridMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *Handler, action *string) {
			handler.activateRecursiveGridModeWithAction(action)
		},
		HandleKeyFunc: func(handler *Handler, key string) {
			handler.handleRecursiveGridKey(key)
		},
		ExitFunc: func(handler *Handler) {
			handler.cleanupRecursiveGridMode()
		},
	}

	return &RecursiveGridMode{
		GenericMode: NewGenericMode(
			handler,
			domain.ModeRecursiveGrid,
			"RecursiveGridMode",
			behavior,
		),
	}
}
