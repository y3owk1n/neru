package domain

import (
	"testing"
)

func TestGetModeString(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{
			name: "idle mode",
			mode: ModeIdle,
			want: "idle",
		},
		{
			name: "hints mode",
			mode: ModeHints,
			want: "hints",
		},
		{
			name: "grid mode",
			mode: ModeGrid,
			want: "grid",
		},
		{
			name: "unknown mode",
			mode: Mode(999),
			want: UnknownMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModeString(tt.mode)
			if got != tt.want {
				t.Errorf("GetModeString(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGetActionString(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   string
	}{
		{
			name:   "left click",
			action: ActionLeftClick,
			want:   "left_click",
		},
		{
			name:   "right click",
			action: ActionRightClick,
			want:   "right_click",
		},
		{
			name:   "mouse up",
			action: ActionMouseUp,
			want:   "mouse_up",
		},
		{
			name:   "mouse down",
			action: ActionMouseDown,
			want:   "mouse_down",
		},
		{
			name:   "middle click",
			action: ActionMiddleClick,
			want:   "middle_click",
		},
		{
			name:   "move mouse",
			action: ActionMoveMouse,
			want:   "move_mouse",
		},
		{
			name:   "scroll",
			action: ActionScroll,
			want:   "scroll",
		},
		{
			name:   "unknown action",
			action: Action(999),
			want:   UnknownAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetActionString(tt.action)
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
		wantAction Action
		wantOk     bool
	}{
		{
			name:       "left_click",
			actionStr:  "left_click",
			wantAction: ActionLeftClick,
			wantOk:     true,
		},
		{
			name:       "right_click",
			actionStr:  "right_click",
			wantAction: ActionRightClick,
			wantOk:     true,
		},
		{
			name:       "mouse_up",
			actionStr:  "mouse_up",
			wantAction: ActionMouseUp,
			wantOk:     true,
		},
		{
			name:       "mouse_down",
			actionStr:  "mouse_down",
			wantAction: ActionMouseDown,
			wantOk:     true,
		},
		{
			name:       "middle_click",
			actionStr:  "middle_click",
			wantAction: ActionMiddleClick,
			wantOk:     true,
		},
		{
			name:       "move_mouse",
			actionStr:  "move_mouse",
			wantAction: ActionMoveMouse,
			wantOk:     true,
		},
		{
			name:       "scroll",
			actionStr:  "scroll",
			wantAction: ActionScroll,
			wantOk:     true,
		},
		{
			name:       "unknown action",
			actionStr:  "unknown_action",
			wantAction: ActionMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "empty string",
			actionStr:  "",
			wantAction: ActionMoveMouse, // Default fallback
			wantOk:     false,
		},
		{
			name:       "invalid action",
			actionStr:  "invalid",
			wantAction: ActionMoveMouse, // Default fallback
			wantOk:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotAction, gotOk := GetActionFromString(test.actionStr)
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
	actions := []Action{
		ActionLeftClick,
		ActionRightClick,
		ActionMouseUp,
		ActionMouseDown,
		ActionMiddleClick,
		ActionMoveMouse,
		ActionScroll,
	}

	for _, action := range actions {
		t.Run(GetActionString(action), func(t *testing.T) {
			actionString := GetActionString(action)

			gotAction, ok := GetActionFromString(actionString)
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

// Benchmark tests.
func BenchmarkGetModeString(b *testing.B) {
	for b.Loop() {
		_ = GetModeString(ModeHints)
	}
}

func BenchmarkGetActionString(b *testing.B) {
	for b.Loop() {
		_ = GetActionString(ActionLeftClick)
	}
}

func BenchmarkGetActionFromString(b *testing.B) {
	for b.Loop() {
		_, _ = GetActionFromString("left_click")
	}
}

func BenchmarkActionStringRoundTrip(b *testing.B) {
	for b.Loop() {
		str := GetActionString(ActionLeftClick)
		_, _ = GetActionFromString(str)
	}
}
