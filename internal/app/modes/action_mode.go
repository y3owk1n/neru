package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ActionMode implements the Mode interface for standalone action mode.
type ActionMode struct {
	handler *Handler
}

// NewActionMode creates a new action mode implementation.
func NewActionMode(handler *Handler) *ActionMode {
	return &ActionMode{handler: handler}
}

// ModeType returns the domain mode type.
func (m *ActionMode) ModeType() domain.Mode {
	return domain.ModeAction
}

// Activate activates action mode.
func (m *ActionMode) Activate(action *string) {
	// action parameter intentionally unused - ActionMode handles actions directly
	m.handler.StartActionMode()
}

// HandleKey processes key presses for action mode.
func (m *ActionMode) HandleKey(key string) {
	// Action mode handles keys directly through handleActionKey
}

// HandleActionKey processes action keys when in action mode.
func (m *ActionMode) HandleActionKey(key string) {
	m.handler.handleActionKey(key, "Action")
}

// Exit performs action mode cleanup.
func (m *ActionMode) Exit() {
	// Action mode cleanup is handled by exiting to idle
}

// ToggleActionMode toggles between overlay and action modes for action.
// Action mode doesn't have sub-modes.
func (m *ActionMode) ToggleActionMode() {
	// Action mode doesn't support toggling
}
