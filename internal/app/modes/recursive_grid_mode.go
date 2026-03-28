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
		ActivateFunc: func(handler *Handler, opts ModeActivationOptions) {
			handler.activateRecursiveGridModeWithAction(
				opts.Action,
				opts.Repeat,
				opts.CursorFollowSelection,
			)
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
