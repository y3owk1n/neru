package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance check.
var _ Mode = (*RecursiveGridMode)(nil)

// RecursiveGridMode implements the Mode interface for recursive-grid navigation.
type RecursiveGridMode struct {
	baseMode
}

// NewRecursiveGridMode creates a new recursive-grid mode instance.
func NewRecursiveGridMode(handler *handlerState) *RecursiveGridMode {
	return &RecursiveGridMode{
		baseMode: newBaseMode(handler, domain.ModeRecursiveGrid, "RecursiveGridMode"),
	}
}

// Activate enters recursive-grid mode with the flags the activation carries.
func (m *RecursiveGridMode) Activate(activation modecmd.Activation) {
	m.handler.activateRecursiveGridModeWithAction(activation)
}

// HandleKey processes a key press within recursive-grid mode.
func (m *RecursiveGridMode) HandleKey(key string) {
	m.handler.handleRecursiveGridKey(key)
}

// Exit tears recursive-grid mode down.
func (m *RecursiveGridMode) Exit() {
	m.handler.cleanupRecursiveGridMode()
}
