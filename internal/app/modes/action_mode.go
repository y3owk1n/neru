package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ActionMode implements the Mode interface for standalone action mode.
type ActionMode struct {
	baseMode
}

// NewActionMode creates a new action mode implementation.
func NewActionMode(handler *Handler) *ActionMode {
	return &ActionMode{
		baseMode: newBaseMode(handler, domain.ModeAction, "ActionMode"),
	}
}

// Activate activates action mode.
func (m *ActionMode) Activate(action *string) {
	// action parameter intentionally unused - ActionMode handles actions directly
	m.handler.StartActionMode()
}

// HandleActionKey processes action keys when in action mode.
func (m *ActionMode) HandleActionKey(key string) {
	m.handler.handleActionKey(key, ModeNameAction)
}

// Exit performs action mode cleanup.
func (m *ActionMode) Exit() {
	// Clear the action highlight overlay specific to action mode
	m.handler.clearAndHideOverlay()
}
