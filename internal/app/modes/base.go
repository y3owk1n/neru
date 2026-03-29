package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

const (
	// ModeNameAction is the name for action mode.
	ModeNameAction = "Action"
	// ModeNameHints is the name for hints mode.
	ModeNameHints = "Hints"
	// ModeNameGrid is the name for grid mode.
	ModeNameGrid = "Grid"
	// ModeNameScroll is the name for scroll mode.
	ModeNameScroll = "Scroll"
)

// baseMode provides common functionality for all mode implementations.
// It contains the shared handler dependency and mode type.
type baseMode struct {
	handler  *Handler
	modeType domain.Mode
}

// newBaseMode creates a new base mode with the given handler and mode type.
func newBaseMode(handler *Handler, modeType domain.Mode, _ string) baseMode {
	if handler == nil {
		panic("mode handler cannot be nil")
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
func (m *baseMode) Activate(_ ModeActivationOptions) {
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
