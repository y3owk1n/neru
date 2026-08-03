package action

import (
	"regexp"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// unknownLabel is the String() result for values outside the defined enums.
const unknownLabel = "unknown"

// Type is the type of action to perform on a UI element.
type Type int

const (
	// TypeLeftClick performs a left mouse click.
	TypeLeftClick Type = iota
	// TypeRightClick performs a right mouse click.
	TypeRightClick
	// TypeMiddleClick performs a middle mouse click.
	TypeMiddleClick
	// TypeLeftMouseDown presses and holds the left mouse button.
	TypeLeftMouseDown
	// TypeLeftMouseUp releases the left mouse button.
	TypeLeftMouseUp
	// TypeMoveMouse moves the mouse cursor to a specific point (absolute).
	TypeMoveMouse
	// TypeMoveMouseRelative moves the mouse cursor relative to current position.
	TypeMoveMouseRelative
	// TypeScroll performs a scroll action.
	TypeScroll
	// TypeRightMouseDown presses and holds the right mouse button.
	TypeRightMouseDown
	// TypeRightMouseUp releases the right mouse button.
	TypeRightMouseUp
	// TypeMiddleMouseDown presses and holds the middle mouse button.
	TypeMiddleMouseDown
	// TypeMiddleMouseUp releases the middle mouse button.
	TypeMiddleMouseUp
	// TypeLeftMouseToggle releases the left mouse button when held, presses it otherwise.
	TypeLeftMouseToggle
	// TypeRightMouseToggle releases the right mouse button when held, presses it otherwise.
	TypeRightMouseToggle
	// TypeMiddleMouseToggle releases the middle mouse button when held, presses it otherwise.
	TypeMiddleMouseToggle
)

// MouseButton identifies a physical mouse button.
type MouseButton int

const (
	// ButtonLeft is the left mouse button.
	ButtonLeft MouseButton = iota
	// ButtonRight is the right mouse button.
	ButtonRight
	// ButtonMiddle is the middle mouse button.
	ButtonMiddle
)

// String returns the string representation of the mouse button.
func (b MouseButton) String() string {
	switch b {
	case ButtonLeft:
		return "left"
	case ButtonRight:
		return "right"
	case ButtonMiddle:
		return "middle"
	default:
		return unknownLabel
	}
}

// MouseButtons returns every mouse button in release priority order.
func MouseButtons() []MouseButton {
	return []MouseButton{ButtonLeft, ButtonRight, ButtonMiddle}
}

// MousePhase describes which part of a button press a mouse action performs.
type MousePhase int

const (
	// PhaseClick presses and releases the button in one action.
	PhaseClick MousePhase = iota
	// PhaseDown presses and holds the button.
	PhaseDown
	// PhaseUp releases a held button.
	PhaseUp
	// PhaseToggle presses the button when it is up and releases it when it is held.
	PhaseToggle
)

// String returns the string representation of the mouse phase.
func (p MousePhase) String() string {
	switch p {
	case PhaseClick:
		return "click"
	case PhaseDown:
		return "down"
	case PhaseUp:
		return "up"
	case PhaseToggle:
		return "toggle"
	default:
		return unknownLabel
	}
}

// ParsePhase parses the value of the --state flag into a mouse phase.
// Only the down and up phases are expressible as --state values; click is the
// absence of the flag and toggle has its own --toggle flag.
func ParsePhase(phaseString string) (MousePhase, error) {
	switch phaseString {
	case "down":
		return PhaseDown, nil
	case "up":
		return PhaseUp, nil
	default:
		return 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"unknown mouse button state: %s (expected down or up)",
			phaseString,
		)
	}
}

// MouseButtonPhase decomposes a mouse button action type into the button it
// acts on and the phase it performs. The second return value is false for
// action types that are not mouse button actions.
func (t Type) MouseButtonPhase() (MouseButton, MousePhase, bool) {
	switch t {
	case TypeLeftClick:
		return ButtonLeft, PhaseClick, true
	case TypeRightClick:
		return ButtonRight, PhaseClick, true
	case TypeMiddleClick:
		return ButtonMiddle, PhaseClick, true
	case TypeLeftMouseDown:
		return ButtonLeft, PhaseDown, true
	case TypeLeftMouseUp:
		return ButtonLeft, PhaseUp, true
	case TypeRightMouseDown:
		return ButtonRight, PhaseDown, true
	case TypeRightMouseUp:
		return ButtonRight, PhaseUp, true
	case TypeMiddleMouseDown:
		return ButtonMiddle, PhaseDown, true
	case TypeMiddleMouseUp:
		return ButtonMiddle, PhaseUp, true
	case TypeLeftMouseToggle:
		return ButtonLeft, PhaseToggle, true
	case TypeRightMouseToggle:
		return ButtonRight, PhaseToggle, true
	case TypeMiddleMouseToggle:
		return ButtonMiddle, PhaseToggle, true
	case TypeMoveMouse, TypeMoveMouseRelative, TypeScroll:
		return 0, 0, false
	default:
		return 0, 0, false
	}
}

// MouseButtonName returns the action name that performs the given phase on the
// given button. The second return value is false for combinations that have no
// name (there is no phase outside the four defined ones).
func MouseButtonName(button MouseButton, phase MousePhase) (Name, bool) {
	switch phase {
	case PhaseClick:
		switch button {
		case ButtonLeft:
			return NameLeftClick, true
		case ButtonRight:
			return NameRightClick, true
		case ButtonMiddle:
			return NameMiddleClick, true
		}
	case PhaseDown:
		switch button {
		case ButtonLeft:
			return NameLeftMouseDown, true
		case ButtonRight:
			return NameRightMouseDown, true
		case ButtonMiddle:
			return NameMiddleMouseDown, true
		}
	case PhaseUp:
		switch button {
		case ButtonLeft:
			return NameLeftMouseUp, true
		case ButtonRight:
			return NameRightMouseUp, true
		case ButtonMiddle:
			return NameMiddleMouseUp, true
		}
	case PhaseToggle:
		switch button {
		case ButtonLeft:
			return NameLeftMouseToggle, true
		case ButtonRight:
			return NameRightMouseToggle, true
		case ButtonMiddle:
			return NameMiddleMouseToggle, true
		}
	}

	return "", false
}

// String returns the string representation of the action type.
func (t Type) String() string {
	switch t {
	case TypeLeftClick:
		return "left_click"
	case TypeRightClick:
		return "right_click"
	case TypeMiddleClick:
		return "middle_click"
	case TypeLeftMouseDown:
		return "left_mouse_down"
	case TypeLeftMouseUp:
		return "left_mouse_up"
	case TypeRightMouseDown:
		return "right_mouse_down"
	case TypeRightMouseUp:
		return "right_mouse_up"
	case TypeMiddleMouseDown:
		return "middle_mouse_down"
	case TypeMiddleMouseUp:
		return "middle_mouse_up"
	case TypeLeftMouseToggle:
		return "left_mouse_toggle"
	case TypeRightMouseToggle:
		return "right_mouse_toggle"
	case TypeMiddleMouseToggle:
		return "middle_mouse_toggle"
	case TypeMoveMouse:
		return "move_mouse"
	case TypeMoveMouseRelative:
		return "move_mouse_relative"
	case TypeScroll:
		return "scroll"
	default:
		return unknownLabel
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
	// mouse_down / mouse_up are the original left-button spellings, from
	// before the right and middle buttons could be pressed and released. They
	// stay accepted so existing configs keep working.
	case "left_mouse_down", "mouse_down":
		return TypeLeftMouseDown, nil
	case "left_mouse_up", "mouse_up":
		return TypeLeftMouseUp, nil
	case "right_mouse_down":
		return TypeRightMouseDown, nil
	case "right_mouse_up":
		return TypeRightMouseUp, nil
	case "middle_mouse_down":
		return TypeMiddleMouseDown, nil
	case "middle_mouse_up":
		return TypeMiddleMouseUp, nil
	case "left_mouse_toggle":
		return TypeLeftMouseToggle, nil
	case "right_mouse_toggle":
		return TypeRightMouseToggle, nil
	case "middle_mouse_toggle":
		return TypeMiddleMouseToggle, nil
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
	_, _, ok := t.MouseButtonPhase()

	return ok
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
	TypeLeftMouseDown,
	TypeLeftMouseUp,
	TypeRightMouseDown,
	TypeRightMouseUp,
	TypeMiddleMouseDown,
	TypeMiddleMouseUp,
	TypeLeftMouseToggle,
	TypeRightMouseToggle,
	TypeMiddleMouseToggle,
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

// Name is a named action that can be performed by the application.
// This is used for configuration and user input, while Type is used for execution.
type Name string

const (
	// NameLeftClick is the left click action.
	NameLeftClick Name = "left_click"
	// NameRightClick is the right click action.
	NameRightClick Name = "right_click"
	// NameMiddleClick is the middle click action.
	NameMiddleClick Name = "middle_click"
	// NameLeftMouseDown is the left mouse button down action.
	NameLeftMouseDown Name = "left_mouse_down"
	// NameLeftMouseUp is the left mouse button up action.
	NameLeftMouseUp Name = "left_mouse_up"
	// NameMouseDown is the original spelling of NameLeftMouseDown, from before
	// the right and middle buttons could be pressed and released.
	//
	// Deprecated: use NameLeftMouseDown; "mouse_down" is still accepted from
	// configs and IPC callers.
	NameMouseDown Name = "mouse_down"
	// NameMouseUp is the original spelling of NameLeftMouseUp, from before the
	// right and middle buttons could be pressed and released.
	//
	// Deprecated: use NameLeftMouseUp; "mouse_up" is still accepted from
	// configs and IPC callers.
	NameMouseUp Name = "mouse_up"
	// NameRightMouseDown is the right mouse button down action.
	NameRightMouseDown Name = "right_mouse_down"
	// NameRightMouseUp is the right mouse button up action.
	NameRightMouseUp Name = "right_mouse_up"
	// NameMiddleMouseDown is the middle mouse button down action.
	NameMiddleMouseDown Name = "middle_mouse_down"
	// NameMiddleMouseUp is the middle mouse button up action.
	NameMiddleMouseUp Name = "middle_mouse_up"
	// NameLeftMouseToggle is the left mouse button toggle action.
	NameLeftMouseToggle Name = "left_mouse_toggle"
	// NameRightMouseToggle is the right mouse button toggle action.
	NameRightMouseToggle Name = "right_mouse_toggle"
	// NameMiddleMouseToggle is the middle mouse button toggle action.
	NameMiddleMouseToggle Name = "middle_mouse_toggle"
	// NameMoveMouse is the mouse move action.
	NameMoveMouse Name = "move_mouse"
	// NameMoveMouseRelative is the relative mouse move action.
	NameMoveMouseRelative Name = "move_mouse_relative"
	// NameScroll is the scroll action.
	NameScroll Name = "scroll"
	// NameReset resets current mode state.
	NameReset Name = "reset"
	// NameBackspace performs a mode-aware backspace operation.
	NameBackspace Name = "backspace"
	// NameMoveCell slides the active mode's selection to a neighboring cell.
	NameMoveCell Name = "move_cell"
	// NameWaitForModeExit blocks until the current mode exits.
	NameWaitForModeExit Name = "wait_for_mode_exit"
	// NameSaveCursorPos saves the current cursor position for later restoration.
	NameSaveCursorPos Name = "save_cursor_pos"
	// NameRestoreCursorPos restores cursor position saved by save_cursor_pos.
	NameRestoreCursorPos Name = "restore_cursor_pos"
	// NameScrollUp is the scroll-up action.
	NameScrollUp Name = "scroll_up"
	// NameScrollDown is the scroll-down action.
	NameScrollDown Name = "scroll_down"
	// NameScrollLeft is the scroll-left action.
	NameScrollLeft Name = "scroll_left"
	// NameScrollRight is the scroll-right action.
	NameScrollRight Name = "scroll_right"
	// NameMoveMonitor moves the cursor (and any active overlay) to the next or previous connected monitor.
	NameMoveMonitor Name = "move_monitor"
	// NameFeed posts a key or key chord directly to the operating system.
	NameFeed Name = "feed"
	// NameGoTop is the go-to-top action.
	NameGoTop Name = "go_top"
	// NameGoBottom is the go-to-bottom action.
	NameGoBottom Name = "go_bottom"
	// NamePageUp is the page-up action.
	NamePageUp Name = "page_up"
	// NamePageDown is the page-down action.
	NamePageDown Name = "page_down"
	// NameHideCursor hides the system cursor.
	NameHideCursor Name = "hide_cursor"
	// NameShowCursor shows the system cursor.
	NameShowCursor Name = "show_cursor"
	// NameSleep pauses action execution for a specified duration.
	NameSleep Name = "sleep"
	// NameCycleHint cycles through visible hints in hints mode.
	NameCycleHint Name = "cycle_hint"
	// NameSearchHints starts text filtering in hints mode.
	NameSearchHints Name = "search_hints"

	// PrefixExec is the prefix for shell command actions.
	PrefixExec = "exec"
)

// knownNames lists the action names that can be used as pending mode actions
// (e.g. --action flag on hints/grid commands). Scroll sub-actions (scroll_up,
// page_down, etc.) are intentionally excluded — they are IPC/CLI-only and are
// recognized separately by IsScrollSubAction and IsKnownName.
var knownNames = []Name{
	NameLeftClick,
	NameRightClick,
	NameMiddleClick,
	NameLeftMouseDown,
	NameLeftMouseUp,
	NameRightMouseDown,
	NameRightMouseUp,
	NameMiddleMouseDown,
	NameMiddleMouseUp,
	NameLeftMouseToggle,
	NameRightMouseToggle,
	NameMiddleMouseToggle,
	NameMoveMouse,
	NameMoveMouseRelative,
	NameScroll,
}

// KnownNames returns the mode-compatible action names (excludes scroll sub-actions).
func KnownNames() []Name {
	result := make([]Name, len(knownNames))
	copy(result, knownNames)

	return result
}

// SupportedNamesString returns a comma-separated string of mode-compatible action names for user messages.
func SupportedNamesString() string {
	names := KnownNames()

	strs := make([]string, len(names))
	for i, name := range names {
		strs[i] = string(name)
	}

	return strings.Join(strs, ", ")
}

// ModeActionNamesString returns the comma-separated action names accepted by a
// mode's --action flag, for use in help text.
//
// A mode performs its pending action at the selected point once the selection
// is made, which only mouse button actions can do. The set is derived from the
// same IsMouseButton predicate the CLI validates against, so help text cannot
// drift from what is actually accepted.
func ModeActionNamesString() string {
	var names []string

	for _, name := range KnownNames() {
		actionType, err := name.ToType()
		if err != nil || !actionType.IsMouseButton() {
			continue
		}

		names = append(names, string(name))
	}

	return strings.Join(names, ", ")
}

// IsResetAction reports whether the given action is reset.
func IsResetAction(name string) bool {
	return Name(name) == NameReset
}

// IsBackspaceAction reports whether the given action is backspace.
func IsBackspaceAction(name string) bool {
	return Name(name) == NameBackspace
}

// IsMoveCellAction reports whether the given action is move_cell.
func IsMoveCellAction(name string) bool {
	return Name(name) == NameMoveCell
}

// IsWaitForModeExitAction reports whether the given action is wait_for_mode_exit.
func IsWaitForModeExitAction(name string) bool {
	return Name(name) == NameWaitForModeExit
}

// IsSaveCursorPosAction reports whether the given action is save_cursor_pos.
func IsSaveCursorPosAction(name string) bool {
	return Name(name) == NameSaveCursorPos
}

// IsRestoreCursorPosAction reports whether the given action is restore_cursor_pos.
func IsRestoreCursorPosAction(name string) bool {
	return Name(name) == NameRestoreCursorPos
}

// cursorSlotNamePattern is what a --slot value may be called. The rule matches
// the one for macro names, so the two kinds of name a config author writes look
// the same, and it keeps a mistyped flag (`--slot --center`) from quietly
// becoming a slot rather than an error.
var cursorSlotNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// IsValidCursorSlotName reports whether name may be used as a cursor slot.
func IsValidCursorSlotName(name string) bool {
	return cursorSlotNamePattern.MatchString(name)
}

// IsHideCursorAction reports whether the given action is hide_cursor.
func IsHideCursorAction(name string) bool {
	return Name(name) == NameHideCursor
}

// IsShowCursorAction reports whether the given action is show_cursor.
func IsShowCursorAction(name string) bool {
	return Name(name) == NameShowCursor
}

// IsMoveMonitorAction reports whether the given action is move_monitor.
func IsMoveMonitorAction(name string) bool {
	return Name(name) == NameMoveMonitor
}

// IsCycleHintAction reports whether the given action is cycle_hint.
func IsCycleHintAction(name string) bool {
	return Name(name) == NameCycleHint
}

// IsSearchHintsAction reports whether the given action is search_hints.
func IsSearchHintsAction(name string) bool {
	return Name(name) == NameSearchHints
}

// IsKnownName determines whether the specified action name is recognized by the
// application. This is a superset of the names in knownNames — it also includes
// scroll sub-actions (scroll_up, page_down, etc.) which are IPC/CLI-only.
// Use IsScrollSubAction to distinguish scroll sub-actions from mode-compatible names.
func IsKnownName(name Name) bool {
	switch name {
	case NameLeftClick,
		NameRightClick,
		NameMiddleClick,
		NameLeftMouseDown,
		NameLeftMouseUp,
		NameMouseDown,
		NameMouseUp,
		NameRightMouseDown,
		NameRightMouseUp,
		NameMiddleMouseDown,
		NameMiddleMouseUp,
		NameLeftMouseToggle,
		NameRightMouseToggle,
		NameMiddleMouseToggle,
		NameMoveMouse,
		NameMoveMouseRelative,
		NameScroll,
		NameReset, NameBackspace, NameMoveCell,
		NameWaitForModeExit, NameSaveCursorPos, NameRestoreCursorPos,
		NameScrollUp, NameScrollDown, NameScrollLeft, NameScrollRight,
		NameGoTop, NameGoBottom, NamePageUp, NamePageDown,
		NameMoveMonitor, NameFeed, NameSleep, NameCycleHint, NameSearchHints,
		NameHideCursor, NameShowCursor:
		return true
	default:
		return false
	}
}

// IsFeedAction reports whether the given action feeds a key to the operating system.
func IsFeedAction(name string) bool {
	return Name(name) == NameFeed
}

// IsScrollSubAction reports whether the given name is a scroll sub-action
// (scroll_up, scroll_down, etc.) that can be dispatched via the action CLI.
func IsScrollSubAction(name string) bool {
	switch Name(name) {
	case NameScrollUp, NameScrollDown, NameScrollLeft, NameScrollRight,
		NameGoTop, NameGoBottom, NamePageUp, NamePageDown:
		return true
	case NameLeftClick, NameRightClick, NameMiddleClick,
		NameLeftMouseDown, NameLeftMouseUp,
		NameMouseDown, NameMouseUp,
		NameRightMouseDown, NameRightMouseUp,
		NameMiddleMouseDown, NameMiddleMouseUp,
		NameLeftMouseToggle, NameRightMouseToggle, NameMiddleMouseToggle,
		NameMoveMouse, NameMoveMouseRelative, NameScroll,
		NameReset, NameBackspace, NameMoveCell,
		NameWaitForModeExit, NameSaveCursorPos, NameRestoreCursorPos,
		NameMoveMonitor, NameFeed, NameSleep, NameCycleHint, NameSearchHints,
		NameHideCursor, NameShowCursor:
		return false
	default:
		return false
	}
}

// IsHeldRepeatAction reports whether the action name supports held-key repeat
// (fires repeatedly while the key is held, with no initial delay).
// Currently applies to scroll, page, relative mouse move, and cell move actions.
func IsHeldRepeatAction(name Name) bool {
	switch name { //nolint:exhaustive
	case NameScrollUp, NameScrollDown, NameScrollLeft, NameScrollRight,
		NamePageUp, NamePageDown,
		NameMoveMouseRelative,
		NameMoveCell:
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
	case TypeLeftMouseDown:
		return NameLeftMouseDown
	case TypeLeftMouseUp:
		return NameLeftMouseUp
	case TypeRightMouseDown:
		return NameRightMouseDown
	case TypeRightMouseUp:
		return NameRightMouseUp
	case TypeMiddleMouseDown:
		return NameMiddleMouseDown
	case TypeMiddleMouseUp:
		return NameMiddleMouseUp
	case TypeLeftMouseToggle:
		return NameLeftMouseToggle
	case TypeRightMouseToggle:
		return NameRightMouseToggle
	case TypeMiddleMouseToggle:
		return NameMiddleMouseToggle
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
	case NameLeftMouseDown, NameMouseDown:
		return TypeLeftMouseDown, nil
	case NameLeftMouseUp, NameMouseUp:
		return TypeLeftMouseUp, nil
	case NameRightMouseDown:
		return TypeRightMouseDown, nil
	case NameRightMouseUp:
		return TypeRightMouseUp, nil
	case NameMiddleMouseDown:
		return TypeMiddleMouseDown, nil
	case NameMiddleMouseUp:
		return TypeMiddleMouseUp, nil
	case NameLeftMouseToggle:
		return TypeLeftMouseToggle, nil
	case NameRightMouseToggle:
		return TypeRightMouseToggle, nil
	case NameMiddleMouseToggle:
		return TypeMiddleMouseToggle, nil
	case NameMoveMouse:
		return TypeMoveMouse, nil
	case NameMoveMouseRelative:
		return TypeMoveMouseRelative, nil
	// NOTE: scroll sub-actions map to the generic TypeScroll, which loses
	// directional information. In practice these names are intercepted by
	// IsScrollSubAction in the IPC handler before ToType is called.
	case NameScroll,
		NameScrollUp, NameScrollDown, NameScrollLeft, NameScrollRight,
		NameGoTop, NameGoBottom, NamePageUp, NamePageDown:
		return TypeScroll, nil
	case NameReset,
		NameBackspace,
		NameMoveCell,
		NameWaitForModeExit,
		NameSaveCursorPos,
		NameRestoreCursorPos,
		NameMoveMonitor,
		NameFeed,
		NameSleep,
		NameCycleHint,
		NameSearchHints,
		NameHideCursor,
		NameShowCursor:
		return 0, derrors.Newf(derrors.CodeInvalidInput, "action name not executable: %s", n)
	default:
		return 0, derrors.Newf(derrors.CodeInvalidInput, "unknown action name: %s", n)
	}
}
