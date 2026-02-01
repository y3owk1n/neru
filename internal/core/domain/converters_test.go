package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
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
			want: "scroll",
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
			want:   "left_click",
		},
		{
			name:   "right click",
			action: action.TypeRightClick,
			want:   "right_click",
		},
		{
			name:   "mouse up",
			action: action.TypeMouseUp,
			want:   "mouse_up",
		},
		{
			name:   "mouse down",
			action: action.TypeMouseDown,
			want:   "mouse_down",
		},
		{
			name:   "middle click",
			action: action.TypeMiddleClick,
			want:   "middle_click",
		},
		{
			name:   "move mouse",
			action: action.TypeMoveMouse,
			want:   "move_mouse",
		},
		{
			name:   "scroll",
			action: action.TypeScroll,
			want:   "scroll",
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
			name:       "left_click",
			actionStr:  "left_click",
			wantAction: action.TypeLeftClick,
			wantOk:     true,
		},
		{
			name:       "right_click",
			actionStr:  "right_click",
			wantAction: action.TypeRightClick,
			wantOk:     true,
		},
		{
			name:       "mouse_up",
			actionStr:  "mouse_up",
			wantAction: action.TypeMouseUp,
			wantOk:     true,
		},
		{
			name:       "mouse_down",
			actionStr:  "mouse_down",
			wantAction: action.TypeMouseDown,
			wantOk:     true,
		},
		{
			name:       "middle_click",
			actionStr:  "middle_click",
			wantAction: action.TypeMiddleClick,
			wantOk:     true,
		},
		{
			name:       "move_mouse",
			actionStr:  "move_mouse",
			wantAction: action.TypeMoveMouse,
			wantOk:     true,
		},
		{
			name:       "scroll",
			actionStr:  "scroll",
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
