package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance check.
var _ Mode = (*HintsMode)(nil)

// HintsMode implements the Mode interface for hints-based navigation.
type HintsMode struct {
	baseMode
}

// NewHintsMode creates a new hints mode implementation.
func NewHintsMode(handler *handlerState) *HintsMode {
	return &HintsMode{
		baseMode: newBaseMode(handler, domain.ModeHints, "HintsMode"),
	}
}

// Activate enters hints mode with the flags the activation carries.
func (m *HintsMode) Activate(activation modecmd.Activation) {
	m.handler.activateHintModeWithAction(activation)
}

// HandleKey processes a key press within hints mode.
func (m *HintsMode) HandleKey(key string) {
	m.handler.handleHintsModeKey(key)
}

// Exit tears hints mode down.
func (m *HintsMode) Exit() {
	m.handler.cleanupHintsMode()
}
