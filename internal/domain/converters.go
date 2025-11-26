package domain

// ModeString converts a Mode to its string representation.
func ModeString(mode Mode) string {
	switch mode {
	case ModeIdle:
		return "idle"
	case ModeHints:
		return "hints"
	case ModeGrid:
		return "grid"
	default:
		return UnknownMode
	}
}

// ActionString converts an Action to its string representation.
func ActionString(action Action) string {
	switch action {
	case ActionLeftClick:
		return "left_click"
	case ActionRightClick:
		return "right_click"
	case ActionMouseUp:
		return "mouse_up"
	case ActionMouseDown:
		return "mouse_down"
	case ActionMiddleClick:
		return "middle_click"
	case ActionMoveMouse:
		return "move_mouse"
	case ActionScroll:
		return "scroll"
	default:
		return UnknownAction
	}
}

// ActionFromString converts a string to its Action representation.
func ActionFromString(actionStr string) (Action, bool) {
	switch actionStr {
	case "left_click":
		return ActionLeftClick, true
	case "right_click":
		return ActionRightClick, true
	case "mouse_up":
		return ActionMouseUp, true
	case "mouse_down":
		return ActionMouseDown, true
	case "middle_click":
		return ActionMiddleClick, true
	case "move_mouse":
		return ActionMoveMouse, true
	case "scroll":
		return ActionScroll, true
	default:
		return ActionMoveMouse, false
	}
}
