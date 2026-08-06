package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance check.
var _ Mode = (*GridMode)(nil)

// GridMode implements the Mode interface for grid-based navigation.
type GridMode struct {
	baseMode
}

// NewGridMode creates a new grid mode implementation.
func NewGridMode(handler *handlerState) *GridMode {
	return &GridMode{
		baseMode: newBaseMode(handler, domain.ModeGrid, "GridMode"),
	}
}

// Activate enters grid mode with the flags the activation carries.
func (m *GridMode) Activate(activation modecmd.Activation) {
	m.handler.activateGridModeWithAction(activation)
}

// HandleKey processes a key press within grid mode.
func (m *GridMode) HandleKey(key string) {
	m.handler.handleGridModeKey(key)
}

// Exit tears grid mode down.
func (m *GridMode) Exit() {
	m.handler.cleanupGridMode()
}
