package action_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	testLeftClick         = "left_click"
	testRightClick        = "right_click"
	testMiddleClick       = "middle_click"
	testMouseDown         = "mouse_down"
	testMouseUp           = "mouse_up"
	testLeftMouseDown     = "left_mouse_down"
	testLeftMouseUp       = "left_mouse_up"
	testMoveMouse         = "move_mouse"
	testMoveMouseRelative = "move_mouse_relative"
	testScroll            = "scroll"
	testUnknown           = "unknown"
)

func TestParseType(t *testing.T) {
	tests := []struct {
		input   string
		want    action.Type
		wantErr bool
	}{
		{testLeftClick, action.TypeLeftClick, false},
		{testRightClick, action.TypeRightClick, false},
		{testMiddleClick, action.TypeMiddleClick, false},
		{testMouseDown, action.TypeLeftMouseDown, false},
		{testMouseUp, action.TypeLeftMouseUp, false},
		{testLeftMouseDown, action.TypeLeftMouseDown, false},
		{testLeftMouseUp, action.TypeLeftMouseUp, false},
		{testMoveMouse, action.TypeMoveMouse, false},
		{testMoveMouseRelative, action.TypeMoveMouseRelative, false},
		{testScroll, action.TypeScroll, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			parsedAction, parsedActionErr := action.ParseType(testCase.input)

			if testCase.wantErr {
				if parsedActionErr == nil {
					t.Error("ParseType() expected error, got nil")
				}

				return
			}

			if parsedActionErr != nil {
				t.Errorf("ParseType() unexpected error: %v", parsedActionErr)

				return
			}

			if parsedAction != testCase.want {
				t.Errorf("ParseType(%q) = %v, want %v", testCase.input, parsedAction, testCase.want)
			}
		})
	}
}

func TestType_String(t *testing.T) {
	tests := []struct {
		actionType action.Type
		want       string
	}{
		{action.TypeLeftClick, testLeftClick},
		{action.TypeRightClick, testRightClick},
		{action.TypeMiddleClick, testMiddleClick},
		{action.TypeLeftMouseDown, testLeftMouseDown},
		{action.TypeLeftMouseUp, testLeftMouseUp},
		{action.TypeMoveMouse, testMoveMouse},
		{action.TypeMoveMouseRelative, testMoveMouseRelative},
		{action.TypeScroll, testScroll},
		{action.Type(999), testUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := testCase.actionType.String()
			if got != testCase.want {
				t.Errorf("String() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestIsKnownName(t *testing.T) {
	tests := []struct {
		name action.Name
		want bool
	}{
		{action.NameLeftClick, true},
		{action.NameRightClick, true},
		{action.NameMiddleClick, true},
		{action.NameMouseDown, true},
		{action.NameMouseUp, true},
		{action.NameMoveMouse, true},
		{action.NameMoveMouseRelative, true},
		{action.NameScroll, true},
		{action.NameReset, true},
		{action.NameBackspace, true},
		{action.NameWaitForModeExit, true},
		{action.NameSaveCursorPos, true},
		{action.NameRestoreCursorPos, true},
		{action.NameMoveMonitor, true},
		{action.NameFeed, true},
		{action.NameHideCursor, true},
		{action.NameShowCursor, true},
		// Scroll sub-actions are recognized by IsKnownName (superset of knownNames).
		{action.NameScrollUp, true},
		{action.NameScrollDown, true},
		{action.NameScrollLeft, true},
		{action.NameScrollRight, true},
		{action.NameGoTop, true},
		{action.NameGoBottom, true},
		{action.NamePageUp, true},
		{action.NamePageDown, true},
		{action.Name("unknown"), false},
		{action.Name(""), false},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.name), func(t *testing.T) {
			got := action.IsKnownName(testCase.name)
			if got != testCase.want {
				t.Errorf("IsKnownName(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestIsScrollSubAction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Scroll sub-actions return true.
		{"scroll_up", true},
		{"scroll_down", true},
		{"scroll_left", true},
		{"scroll_right", true},
		{"go_top", true},
		{"go_bottom", true},
		{"page_up", true},
		{"page_down", true},
		// Non-scroll actions return false.
		{"scroll_diagonal", false},
		{testLeftClick, false},
		{testRightClick, false},
		{testMiddleClick, false},
		{testMouseDown, false},
		{testMouseUp, false},
		{testMoveMouse, false},
		{testMoveMouseRelative, false},
		{testScroll, false},
		{"reset", false},
		{"backspace", false},
		{"wait_for_mode_exit", false},
		{"save_cursor_pos", false},
		{"restore_cursor_pos", false},
		{"move_monitor", false},
		{"hide_cursor", false},
		{"show_cursor", false},
		// Unknown / empty return false.
		{"unknown", false},
		{"", false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := action.IsScrollSubAction(testCase.name)
			if got != testCase.want {
				t.Errorf("IsScrollSubAction(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

// TestSupportedNamesString pins the list the invalid-action error shows. It
// must name every known action, each once, in KnownNames order.
func TestSupportedNamesString(t *testing.T) {
	listed := strings.Split(action.SupportedNamesString(), ", ")
	known := action.KnownNames()

	if len(listed) != len(known) {
		t.Fatalf(
			"SupportedNamesString() lists %d names %q, want %d",
			len(listed),
			listed,
			len(known),
		)
	}

	for i, name := range known {
		if listed[i] != string(name) {
			t.Errorf("SupportedNamesString()[%d] = %q, want %q", i, listed[i], name)
		}
	}
}

func TestKnownNames(t *testing.T) {
	names := action.KnownNames()

	if len(names) != 15 {
		t.Errorf("KnownNames() returned %d names, want 15", len(names))
	}

	seen := make(map[action.Name]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("KnownNames() contains duplicate: %v", name)
		}

		seen[name] = true
	}

	expected := []action.Name{
		action.NameLeftClick,
		action.NameRightClick,
		action.NameMiddleClick,
		action.NameLeftMouseDown,
		action.NameLeftMouseUp,
		action.NameRightMouseDown,
		action.NameRightMouseUp,
		action.NameMiddleMouseDown,
		action.NameMiddleMouseUp,
		action.NameLeftMouseToggle,
		action.NameRightMouseToggle,
		action.NameMiddleMouseToggle,
		action.NameMoveMouse,
		action.NameMoveMouseRelative,
		action.NameScroll,
	}

	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("KnownNames() missing name: %v", exp)
		}
	}
}

func TestName_ToType(t *testing.T) {
	tests := []struct {
		name    action.Name
		want    action.Type
		wantErr bool
	}{
		{action.NameLeftClick, action.TypeLeftClick, false},
		{action.NameRightClick, action.TypeRightClick, false},
		{action.NameMiddleClick, action.TypeMiddleClick, false},
		{action.NameMouseDown, action.TypeLeftMouseDown, false},
		{action.NameMouseUp, action.TypeLeftMouseUp, false},
		{action.NameLeftMouseDown, action.TypeLeftMouseDown, false},
		{action.NameLeftMouseUp, action.TypeLeftMouseUp, false},
		{action.NameMoveMouse, action.TypeMoveMouse, false},
		{action.NameMoveMouseRelative, action.TypeMoveMouseRelative, false},
		{action.NameScroll, action.TypeScroll, false},
		// Scroll sub-actions all map to the generic TypeScroll.
		{action.NameScrollUp, action.TypeScroll, false},
		{action.NameScrollDown, action.TypeScroll, false},
		{action.NameScrollLeft, action.TypeScroll, false},
		{action.NameScrollRight, action.TypeScroll, false},
		{action.NameGoTop, action.TypeScroll, false},
		{action.NameGoBottom, action.TypeScroll, false},
		{action.NamePageUp, action.TypeScroll, false},
		{action.NamePageDown, action.TypeScroll, false},
		{action.NameReset, 0, true},
		{action.NameBackspace, 0, true},
		{action.NameWaitForModeExit, 0, true},
		{action.NameSaveCursorPos, 0, true},
		{action.NameRestoreCursorPos, 0, true},
		{action.Name("unknown"), 0, true},
		{action.Name(""), 0, true},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.name), func(t *testing.T) {
			got, err := testCase.name.ToType()

			if testCase.wantErr {
				if err == nil {
					t.Errorf("ToType() expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("ToType() unexpected error: %v", err)

				return
			}

			if got != testCase.want {
				t.Errorf("ToType() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestName_Type_ParseType_RoundTrip(t *testing.T) {
	for _, name := range action.KnownNames() {
		typ, err := name.ToType()
		if err != nil {
			t.Errorf("ToType(%v) error: %v", name, err)

			continue
		}

		str := typ.String()

		parsedType, err := action.ParseType(str)
		if err != nil {
			t.Errorf("ParseType(%q) error: %v", str, err)

			continue
		}

		if parsedType != typ {
			t.Errorf("Round trip failed: %v -> %v -> %q -> %v", name, typ, str, parsedType)
		}
	}
}
