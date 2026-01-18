package action

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// Type represents the type of action to perform on a UI element.
type Type int

const (
	// TypeLeftClick performs a left mouse click.
	TypeLeftClick Type = iota
	// TypeRightClick performs a right mouse click.
	TypeRightClick
	// TypeMiddleClick performs a middle mouse click.
	TypeMiddleClick
	// TypeMouseDown performs a mouse down event.
	TypeMouseDown
	// TypeMouseUp performs a mouse up event.
	TypeMouseUp
	// TypeMoveMouse moves the mouse cursor to a specific point (absolute).
	TypeMoveMouse
	// TypeMoveMouseRelative moves the mouse cursor relative to current position.
	TypeMoveMouseRelative
	// TypeScroll performs a scroll action.
	TypeScroll
)

// String returns the string representation of the action type.
func (t Type) String() string {
	switch t {
	case TypeLeftClick:
		return "left_click"
	case TypeRightClick:
		return "right_click"
	case TypeMiddleClick:
		return "middle_click"
	case TypeMouseDown:
		return "mouse_down"
	case TypeMouseUp:
		return "mouse_up"
	case TypeMoveMouse:
		return "move_mouse"
	case TypeMoveMouseRelative:
		return "move_mouse_relative"
	case TypeScroll:
		return "scroll"
	default:
		return "unknown"
	}
}

// ParseType parses a string into an action type.
func ParseType(actionString string) (Type, error) {
	switch actionString {
	case "left_click":
		return TypeLeftClick, nil
	case "right_click":
		return TypeRightClick, nil
	case "middle_click":
		return TypeMiddleClick, nil
	case "mouse_down":
		return TypeMouseDown, nil
	case "mouse_up":
		return TypeMouseUp, nil
	case "move_mouse":
		return TypeMoveMouse, nil
	case "move_mouse_relative":
		return TypeMoveMouseRelative, nil
	case "scroll":
		return TypeScroll, nil
	default:
		return 0, derrors.Newf(derrors.CodeInvalidInput, "unknown action type: %s", actionString)
	}
}

// IsClick returns true if the action is a click type.
func (t Type) IsClick() bool {
	return t == TypeLeftClick || t == TypeRightClick || t == TypeMiddleClick
}

// IsMouseButton returns true if the action involves a mouse button.
func (t Type) IsMouseButton() bool {
	return t == TypeLeftClick || t == TypeRightClick || t == TypeMiddleClick ||
		t == TypeMouseDown || t == TypeMouseUp
}

// IsMoveMouse returns true if the action moves the mouse cursor.
func (t Type) IsMoveMouse() bool {
	return t == TypeMoveMouse || t == TypeMoveMouseRelative
}

// allTypes is the cached slice of all valid action types to avoid heap allocation.
var allTypes = []Type{
	TypeLeftClick,
	TypeRightClick,
	TypeMiddleClick,
	TypeMouseDown,
	TypeMouseUp,
	TypeMoveMouse,
	TypeMoveMouseRelative,
	TypeScroll,
}

// AllTypes returns all valid action types.
func AllTypes() []Type {
	result := make([]Type, len(allTypes))
	copy(result, allTypes)

	return result
}
