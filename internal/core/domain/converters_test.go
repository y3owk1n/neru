//go:build !integration

package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/domain"
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
		action domain.Action
		want   string
	}{
		{
			name:   "left click",
			action: domain.ActionLeftClick,
			want:   "left_click",
		},
		{
			name:   "right click",
			action: domain.ActionRightClick,
			want:   "right_click",
		},
		{
			name:   "mouse up",
			action: domain.ActionMouseUp,
			want:   "mouse_up",
		},
		{
			name:   "mouse down",
			action: domain.ActionMouseDown,
			want:   "mouse_down",
		},
		{
			name:   "middle click",
			action: domain.ActionMiddleClick,
			want:   "middle_click",
		},
		{
			name:   "move mouse",
			action: domain.ActionMoveMouse,
			want:   "move_mouse",
		},
		{
			name:   "scroll",
			action: domain.ActionScroll,
			want:   "scroll",
		},
		{
			name:   "unknown action",
			action: domain.Action(999),
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
		wantAction domain.Action
		wantOk     bool
	}{
		{
			name:       "left_click",
			actionStr:  "left_click",
			wantAction: domain.ActionLeftClick,
			wantOk:     true,
		},
		{
			name:       "right_click",
			actionStr:  "right_click",
			wantAction: domain.ActionRightClick,
			wantOk:     true,
		},
		{
			name:       "mouse_up",
			actionStr:  "mouse_up",
			wantAction: domain.ActionMouseUp,
			wantOk:     true,
		},
		{
			name:       "mouse_down",
			actionStr:  "mouse_down",
			wantAction: domain.ActionMouseDown,
			wantOk:     true,
		},
		{
			name:       "middle_click",
			actionStr:  "middle_click",
			wantAction: domain.ActionMiddleClick,
			wantOk:     true,
		},
		{
			name:       "move_mouse",
			actionStr:  "move_mouse",
			wantAction: domain.ActionMoveMouse,
			wantOk:     true,
		},
		{
			name:       "scroll",
			actionStr:  "scroll",
			wantAction: domain.ActionScroll,
			wantOk:     true,
		},
		{
			name:       "unknown action",
			actionStr:  "unknown_action",
			wantAction: domain.ActionMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "empty string",
			actionStr:  "",
			wantAction: domain.ActionMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "invalid action",
			actionStr:  "invalid",
			wantAction: domain.ActionMoveMouse, // Default fallback
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
	actions := []domain.Action{
		domain.ActionLeftClick,
		domain.ActionRightClick,
		domain.ActionMouseUp,
		domain.ActionMouseDown,
		domain.ActionMiddleClick,
		domain.ActionMoveMouse,
		domain.ActionScroll,
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
