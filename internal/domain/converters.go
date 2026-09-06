package domain

import "github.com/y3owk1n/neru/internal/domain/action"

// Mode names as strings.
const (
	ModeNameIdle          = "idle"
	ModeNameHints         = "hints"
	ModeNameGrid          = "grid"
	ModeNameScroll        = "scroll"
	ModeNameRecursiveGrid = "recursive_grid"
	ModeNameMonitorSelect = "monitor_select"
	// ModeNameCustom is the command word that enters a user-declared mode:
	// "mode <name>". It is the word wherever a built-in mode is called by its
	// name, so the CLI command, the IPC action and the binding step all agree;
	// the declared name travels beside it as the first argument.
	ModeNameCustom = "mode"
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
	case ModeRecursiveGrid:
		return ModeNameRecursiveGrid
	case ModeMonitorSelect:
		return ModeNameMonitorSelect
	case ModeCustom:
		return ModeNameCustom
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
