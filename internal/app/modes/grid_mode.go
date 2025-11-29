package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// GridMode implements the Mode interface for grid-based navigation.
type GridMode struct {
	handler *Handler
}

// NewGridMode creates a new grid mode implementation.
func NewGridMode(handler *Handler) *GridMode {
	if handler == nil {
		panic("GridMode: handler cannot be nil")
	}

	return &GridMode{handler: handler}
}

// ModeType returns the domain mode type.
func (m *GridMode) ModeType() domain.Mode {
	return domain.ModeGrid
}

// Activate activates grid mode with optional action parameter.
func (m *GridMode) Activate(action *string) {
	m.handler.activateGridModeWithAction(action)
}

// HandleKey processes key presses for grid mode.
func (m *GridMode) HandleKey(key string) {
	m.handler.handleGridModeKey(key)
}

// HandleActionKey processes action keys when in grid action mode.
func (m *GridMode) HandleActionKey(key string) {
	m.handler.handleGridActionKey(key)
}

// Exit performs grid mode cleanup.
func (m *GridMode) Exit() {
	m.handler.cleanupGridMode()
}

// ToggleActionMode toggles between overlay and action modes for grid.
func (m *GridMode) ToggleActionMode() {
	m.handler.toggleActionModeForGrid()
}
