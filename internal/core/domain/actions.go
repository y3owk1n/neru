package domain

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
	// ActionNameScroll represents the scroll action.
	ActionNameScroll ActionName = "scroll"

	// ActionPrefixExec is the prefix for shell command actions.
	ActionPrefixExec = "exec"
)

// KnownActionNames returns a slice containing all supported action names.
func KnownActionNames() []ActionName {
	return []ActionName{
		ActionNameLeftClick,
		ActionNameRightClick,
		ActionNameMiddleClick,
		ActionNameMouseDown,
		ActionNameMouseUp,
		ActionNameScroll,
	}
}

// IsKnownActionName determines whether the specified action name is supported.
func IsKnownActionName(action ActionName) bool {
	switch action {
	case ActionNameLeftClick,
		ActionNameRightClick,
		ActionNameMiddleClick,
		ActionNameMouseDown,
		ActionNameMouseUp,
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
