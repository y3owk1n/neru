package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
)

// baseMode carries what every mode implementation shares: the inner handler
// state a mode runs against and the domain mode it represents. It answers
// ModeType and nothing else — activate, key handling and exit belong to the
// mode's own type, so that reading that type answers what the mode does.
type baseMode struct {
	handler  *handlerState
	modeType domain.Mode
}

// newBaseMode creates a new base mode with the given handler and mode type.
func newBaseMode(handler *handlerState, modeType domain.Mode, modeName string) baseMode {
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
