package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// HintsMode implements the Mode interface for hints-based navigation.
type HintsMode struct {
	handler *Handler
}

// NewHintsMode creates a new hints mode implementation.
func NewHintsMode(handler *Handler) *HintsMode {
	if handler == nil {
		panic("HintsMode: handler cannot be nil")
	}

	return &HintsMode{handler: handler}
}

// ModeType returns the domain mode type.
func (m *HintsMode) ModeType() domain.Mode {
	return domain.ModeHints
}

// Activate activates hints mode with optional action parameter.
func (m *HintsMode) Activate(action *string) {
	m.handler.activateHintModeWithAction(action)
}

// HandleKey processes key presses for hints mode.
func (m *HintsMode) HandleKey(key string) {
	m.handler.handleHintsModeKey(key)
}

// HandleActionKey processes action keys when in hints action mode.
func (m *HintsMode) HandleActionKey(key string) {
	m.handler.handleHintsActionKey(key)
}

// Exit performs hints mode cleanup.
func (m *HintsMode) Exit() {
	m.handler.cleanupHintsMode()
}

// ToggleActionMode toggles between overlay and action modes for hints.
func (m *HintsMode) ToggleActionMode() {
	m.handler.toggleActionModeForHints()
}
