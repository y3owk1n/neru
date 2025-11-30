package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ActionMode implements the Mode interface for standalone action mode.
// It uses the generic mode implementation with action-specific behavior.
type ActionMode struct {
	*GenericMode
}

// NewActionMode creates a new action mode implementation.
func NewActionMode(handler *Handler) *ActionMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *Handler, action *string) {
			// action parameter intentionally unused - ActionMode handles actions directly
			handler.StartActionMode()
		},
		ExitFunc: func(handler *Handler) {
			// Clear the action highlight overlay specific to action mode
			handler.clearAndHideOverlay()
		},
	}

	return &ActionMode{
		GenericMode: NewGenericMode(handler, domain.ModeAction, "ActionMode", behavior),
	}
}
