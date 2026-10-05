package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	testConvLeftClick   = "left_click"
	testConvRightClick  = "right_click"
	testConvMiddleClick = "middle_click"
	testConvMouseDown   = "left_mouse_down"
	testConvMouseUp     = "left_mouse_up"
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
			action: action.TypeLeftMouseUp,
			want:   testConvMouseUp,
		},
		{
			name:   "mouse down",
			action: action.TypeLeftMouseDown,
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
