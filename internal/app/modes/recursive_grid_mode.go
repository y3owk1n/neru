package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
)

// RecursiveGridMode implements the Mode interface for recursive-grid navigation.
type RecursiveGridMode struct {
	*GenericMode
}

// NewRecursiveGridMode creates a new recursive-grid mode instance.
func NewRecursiveGridMode(handler *handlerState) *RecursiveGridMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *handlerState, opts ModeActivationOptions) {
			handler.activateRecursiveGridModeWithAction(opts)
		},
		HandleKeyFunc: func(handler *handlerState, key string) {
			handler.handleRecursiveGridKey(key)
		},
		ExitFunc: func(handler *handlerState) {
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
