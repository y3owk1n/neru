package action

import (
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

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

// Name represents a named action that can be performed by the application.
// This is used for configuration and user input, while Type is used for execution.
type Name string

const (
	// NameLeftClick represents the left click action.
	NameLeftClick Name = "left_click"
	// NameRightClick represents the right click action.
	NameRightClick Name = "right_click"
	// NameMiddleClick represents the middle click action.
	NameMiddleClick Name = "middle_click"
	// NameMouseDown represents the mouse down action.
	NameMouseDown Name = "mouse_down"
	// NameMouseUp represents the mouse up action.
	NameMouseUp Name = "mouse_up"
	// NameMoveMouse represents the mouse move action.
	NameMoveMouse Name = "move_mouse"
	// NameMoveMouseRelative represents the relative mouse move action.
	NameMoveMouseRelative Name = "move_mouse_relative"
	// NameScroll represents the scroll action.
	NameScroll Name = "scroll"

	// PrefixExec is the prefix for shell command actions.
	PrefixExec = "exec"
)

// knownNames is the cached slice of all supported action names to avoid heap allocation.
var knownNames = []Name{
	NameLeftClick,
	NameRightClick,
	NameMiddleClick,
	NameMouseDown,
	NameMouseUp,
	NameMoveMouse,
	NameMoveMouseRelative,
	NameScroll,
}

// KnownNames returns a slice containing all supported action names.
func KnownNames() []Name {
	result := make([]Name, len(knownNames))
	copy(result, knownNames)

	return result
}

// SupportedNamesString returns a comma-separated string of supported actions for user messages.
func SupportedNamesString() string {
	names := KnownNames()

	strs := make([]string, len(names))
	for i, name := range names {
		strs[i] = string(name)
	}

	return strings.Join(strs, ", ")
}

// IsKnownName determines whether the specified action name is supported.
func IsKnownName(name Name) bool {
	switch name {
	case NameLeftClick,
		NameRightClick,
		NameMiddleClick,
		NameMouseDown,
		NameMouseUp,
		NameMoveMouse,
		NameMoveMouseRelative,
		NameScroll:
		return true
	default:
		return false
	}
}

// ToName converts a Type to its corresponding Name.
func (t Type) ToName() Name {
	switch t {
	case TypeLeftClick:
		return NameLeftClick
	case TypeRightClick:
		return NameRightClick
	case TypeMiddleClick:
		return NameMiddleClick
	case TypeMouseDown:
		return NameMouseDown
	case TypeMouseUp:
		return NameMouseUp
	case TypeMoveMouse:
		return NameMoveMouse
	case TypeMoveMouseRelative:
		return NameMoveMouseRelative
	case TypeScroll:
		return NameScroll
	default:
		return ""
	}
}

// ToType converts a Name to its corresponding Type.
func (n Name) ToType() (Type, error) {
	switch n {
	case NameLeftClick:
		return TypeLeftClick, nil
	case NameRightClick:
		return TypeRightClick, nil
	case NameMiddleClick:
		return TypeMiddleClick, nil
	case NameMouseDown:
		return TypeMouseDown, nil
	case NameMouseUp:
		return TypeMouseUp, nil
	case NameMoveMouse:
		return TypeMoveMouse, nil
	case NameMoveMouseRelative:
		return TypeMoveMouseRelative, nil
	case NameScroll:
		return TypeScroll, nil
	default:
		return 0, derrors.Newf(derrors.CodeInvalidInput, "unknown action name: %s", n)
	}
}
