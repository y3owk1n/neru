package domain

import "github.com/y3owk1n/neru/internal/core/domain/action"

// Mode names as strings.
const (
	ModeNameIdle   = "idle"
	ModeNameHints  = "hints"
	ModeNameGrid   = "grid"
	ModeNameScroll = "scroll"
)

// ModeString converts a Mode to its string representation.
func ModeString(mode Mode) string {
	switch mode {
	case ModeIdle:
		return ModeNameIdle
	case ModeHints:
		return ModeNameHints
	case ModeGrid:
		return ModeNameGrid
	case ModeScroll:
		return ModeNameScroll
	default:
		return UnknownMode
	}
}

// ActionString converts an action.Type to its string representation.
func ActionString(actionType action.Type) string {
	return actionType.String()
}

// ActionFromString converts a string to its action.Type representation.
func ActionFromString(actionStr string) (action.Type, bool) {
	typ, err := action.ParseType(actionStr)
	if err != nil {
		return action.TypeMoveMouse, false
	}

	return typ, true
}
