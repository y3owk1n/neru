package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

const (
	// KeyTab represents the tab key.
	KeyTab = "\t"
	// KeyEscape represents the escape key (primary representation).
	KeyEscape = "\x1b"
	// KeyEscape2 represents the escape key (alternative representation).
	KeyEscape2 = "escape"

	// ModeNameAction is the name for action mode.
	ModeNameAction = "Action"
	// ModeNameHints is the name for hints mode.
	ModeNameHints = "Hints"
	// ModeNameGrid is the name for grid mode.
	ModeNameGrid = "Grid"
	// ModeNameScroll is the name for scroll mode.
	ModeNameScroll = "Scroll"
)

// DefaultModeExitKeys returns the default exit keys when config is empty or validation is bypassed.
func DefaultModeExitKeys() []string {
	return []string{KeyEscape2}
}

// baseMode provides common functionality for all mode implementations.
// It contains the shared handler dependency and mode type.
type baseMode struct {
	handler  *Handler
	modeType domain.Mode
}

// newBaseMode creates a new base mode with the given handler and mode type.
func newBaseMode(handler *Handler, modeType domain.Mode, modeName string) baseMode {
	if handler == nil {
		panic(modeName + ": handler cannot be nil")
	}

	return baseMode{
		handler:  handler,
		modeType: modeType,
	}
}

// ModeType returns the domain mode type.
func (m *baseMode) ModeType() domain.Mode {
	return m.modeType
}

// Activate provides a default empty implementation for modes that don't need activation logic.
func (m *baseMode) Activate(action *string) {
	// Default empty implementation - modes can override if needed
}

// HandleKey provides a default empty implementation for modes that handle keys differently.
func (m *baseMode) HandleKey(key string) {
	// Default empty implementation - modes can override if needed
}

// Exit provides a default empty implementation for modes that don't need specific cleanup.
func (m *baseMode) Exit() {
	// Default empty implementation - modes can override if needed
}
