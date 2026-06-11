package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const (
	testConvLeftClick   = "left_click"
	testConvRightClick  = "right_click"
	testConvMiddleClick = "middle_click"
	testConvMouseDown   = "mouse_down"
	testConvMouseUp     = "mouse_up"
	testConvMoveMouse   = "move_mouse"
	testConvScroll      = "scroll"
)

func TestModeString(t *testing.T) {
	tests := []struct {
		name string
		mode app.Mode
		want string
	}{
		{
			name: "idle mode",
			mode: app.ModeIdle,
			want: "idle",
		},
		{
			name: "hints mode",
			mode: app.ModeHints,
			want: "hints",
		},
		{
			name: "grid mode",
			mode: app.ModeGrid,
			want: "grid",
		},
		{
			name: "scroll mode",
			mode: app.ModeScroll,
			want: domain.ModeNameScroll,
		},
		{
			name: "unknown mode",
			mode: app.Mode(999),
			want: domain.UnknownMode,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := domain.ModeString(testCase.mode)
			if got != testCase.want {
				t.Errorf("ModeString(%v) = %q, want %q", testCase.mode, got, testCase.want)
			}
		})
	}
}

func TestActionString(t *testing.T) {
	tests := []struct {
		name   string
		action action.Type
		want   string
	}{
		{
			name:   "left click",
			action: action.TypeLeftClick,
			want:   testConvLeftClick,
		},
		{
			name:   "right click",
			action: action.TypeRightClick,
			want:   testConvRightClick,
		},
		{
			name:   "mouse up",
			action: action.TypeMouseUp,
			want:   testConvMouseUp,
		},
		{
			name:   "mouse down",
			action: action.TypeMouseDown,
			want:   testConvMouseDown,
		},
		{
			name:   "middle click",
			action: action.TypeMiddleClick,
			want:   testConvMiddleClick,
		},
		{
			name:   "move mouse",
			action: action.TypeMoveMouse,
			want:   testConvMoveMouse,
		},
		{
			name:   "scroll",
			action: action.TypeScroll,
			want:   testConvScroll,
		},
		{
			name:   "unknown action",
			action: action.Type(999),
			want:   domain.UnknownAction,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := domain.ActionString(testCase.action)
			if got != testCase.want {
				t.Errorf("ActionString(%v) = %q, want %q", testCase.action, got, testCase.want)
			}
		})
	}
}

func TestActionFromString(t *testing.T) {
	tests := []struct {
		name       string
		actionStr  string
		wantAction action.Type
		wantOk     bool
	}{
		{
			name:       testConvLeftClick,
			actionStr:  testConvLeftClick,
			wantAction: action.TypeLeftClick,
			wantOk:     true,
		},
		{
			name:       testConvRightClick,
			actionStr:  testConvRightClick,
			wantAction: action.TypeRightClick,
			wantOk:     true,
		},
		{
			name:       testConvMouseUp,
			actionStr:  testConvMouseUp,
			wantAction: action.TypeMouseUp,
			wantOk:     true,
		},
		{
			name:       testConvMouseDown,
			actionStr:  testConvMouseDown,
			wantAction: action.TypeMouseDown,
			wantOk:     true,
		},
		{
			name:       testConvMiddleClick,
			actionStr:  testConvMiddleClick,
			wantAction: action.TypeMiddleClick,
			wantOk:     true,
		},
		{
			name:       testConvMoveMouse,
			actionStr:  testConvMoveMouse,
			wantAction: action.TypeMoveMouse,
			wantOk:     true,
		},
		{
			name:       testConvScroll,
			actionStr:  testConvScroll,
			wantAction: action.TypeScroll,
			wantOk:     true,
		},
		{
			name:       "unknown action",
			actionStr:  "unknown_action",
			wantAction: action.TypeMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "empty string",
			actionStr:  "",
			wantAction: action.TypeMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "invalid action",
			actionStr:  "invalid",
			wantAction: action.TypeMoveMouse, // Default fallback
			wantOk:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			gotAction, gotOk := domain.ActionFromString(testCase.actionStr)
			if gotAction != testCase.wantAction {
				t.Errorf(
					"ActionFromString(%q) action = %v, want %v",
					testCase.actionStr,
					gotAction,
					testCase.wantAction,
				)
			}

			if gotOk != testCase.wantOk {
				t.Errorf(
					"ActionFromString(%q) ok = %v, want %v",
					testCase.actionStr,
					gotOk,
					testCase.wantOk,
				)
			}
		})
	}
}

// TestActionStringRoundTrip verifies that converting an Action to string and back
// returns the same Action.
func TestActionStringRoundTrip(t *testing.T) {
	actions := []action.Type{
		action.TypeLeftClick,
		action.TypeRightClick,
		action.TypeMouseUp,
		action.TypeMouseDown,
		action.TypeMiddleClick,
		action.TypeMoveMouse,
		action.TypeScroll,
	}

	for _, action := range actions {
		t.Run(domain.ActionString(action), func(t *testing.T) {
			actionString := domain.ActionString(action)

			gotAction, ok := domain.ActionFromString(actionString)
			if !ok {
				t.Errorf(
					"Round trip failed: ActionFromString(%q) returned ok=false",
					actionString,
				)
			}

			if gotAction != action {
				t.Errorf("Round trip failed: got %v, want %v", gotAction, action)
			}
		})
	}
}
