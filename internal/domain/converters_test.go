package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/domain"
)

func TestGetModeString(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.GetModeString(tt.mode)
			if got != tt.want {
				t.Errorf("GetModeString(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGetActionString(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.GetActionString(tt.action)
			if got != tt.want {
				t.Errorf("GetActionString(%v) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestGetActionFromString(t *testing.T) {
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotAction, gotOk := domain.GetActionFromString(test.actionStr)
			if gotAction != test.wantAction {
				t.Errorf(
					"GetActionFromString(%q) action = %v, want %v",
					test.actionStr,
					gotAction,
					test.wantAction,
				)
			}

			if gotOk != test.wantOk {
				t.Errorf(
					"GetActionFromString(%q) ok = %v, want %v",
					test.actionStr,
					gotOk,
					test.wantOk,
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
		t.Run(domain.GetActionString(action), func(t *testing.T) {
			actionString := domain.GetActionString(action)

			gotAction, ok := domain.GetActionFromString(actionString)
			if !ok {
				t.Errorf(
					"Round trip failed: GetActionFromString(%q) returned ok=false",
					actionString,
				)
			}

			if gotAction != action {
				t.Errorf("Round trip failed: got %v, want %v", gotAction, action)
			}
		})
	}
}
