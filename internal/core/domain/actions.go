package domain

import "strings"

// ActionName represents a named action that can be performed by the application.
type ActionName string

const (
	// ActionNameLeftClick represents the left click action.
	ActionNameLeftClick ActionName = "left_click"
	// ActionNameRightClick represents the right click action.
	ActionNameRightClick ActionName = "right_click"
	// ActionNameMiddleClick represents the middle click action.
	ActionNameMiddleClick ActionName = "middle_click"
	// ActionNameMouseDown represents the mouse down action.
	ActionNameMouseDown ActionName = "mouse_down"
	// ActionNameMouseUp represents the mouse up action.
	ActionNameMouseUp ActionName = "mouse_up"
	// ActionNameMoveMouse represents the mouse move action.
	ActionNameMoveMouse ActionName = "move_mouse"
	// ActionNameMoveMouseRelative represents the relative mouse move action.
	ActionNameMoveMouseRelative ActionName = "move_mouse_relative"
	// ActionNameScroll represents the scroll action.
	ActionNameScroll ActionName = "scroll"

	// ActionPrefixExec is the prefix for shell command actions.
	ActionPrefixExec = "exec"
)

// knownActionNames is the cached slice of all supported action names to avoid heap allocation.
var knownActionNames = []ActionName{
	ActionNameLeftClick,
	ActionNameRightClick,
	ActionNameMiddleClick,
	ActionNameMouseDown,
	ActionNameMouseUp,
	ActionNameMoveMouse,
	ActionNameMoveMouseRelative,
	ActionNameScroll,
}

// KnownActionNames returns a slice containing all supported action names.
func KnownActionNames() []ActionName {
	result := make([]ActionName, len(knownActionNames))
	copy(result, knownActionNames)

	return result
}

// SupportedActionsString returns a comma-separated string of supported actions for user messages.
func SupportedActionsString() string {
	names := KnownActionNames()

	strs := make([]string, len(names))
	for i, name := range names {
		strs[i] = string(name)
	}

	return strings.Join(strs, ", ")
}

// IsKnownActionName determines whether the specified action name is supported.
func IsKnownActionName(action ActionName) bool {
	switch action {
	case ActionNameLeftClick,
		ActionNameRightClick,
		ActionNameMiddleClick,
		ActionNameMouseDown,
		ActionNameMouseUp,
		ActionNameMoveMouse,
		ActionNameMoveMouseRelative,
		ActionNameScroll:
		return true
	default:
		return false
	}
}

// Action represents the current action of the application.
type Action int

const (
	// ActionLeftClick represents the left click action.
	ActionLeftClick Action = iota
	// ActionRightClick represents the right click action.
	ActionRightClick
	// ActionMouseUp represents the mouse up action.
	ActionMouseUp
	// ActionMouseDown represents the mouse down action.
	ActionMouseDown
	// ActionMiddleClick represents the middle click action.
	ActionMiddleClick
	// ActionMoveMouse represents the mouse move action.
	ActionMoveMouse
	// ActionScroll represents the scroll action.
	ActionScroll
)
