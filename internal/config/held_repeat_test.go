package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	testAccelMoveBinding = "action move_mouse_relative --dx=20 --dy=0"
	testAccelScrollDown  = "action scroll_down"
	testAccelScrollUp    = "action scroll_up"
)

func TestHeldRepeatActionName(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    string
	}{
		{
			name:    "single action binding",
			actions: []string{testAccelMoveBinding},
			want:    "move_mouse_relative",
		},
		{
			name:    "action without arguments",
			actions: []string{testAccelScrollDown},
			want:    "scroll_down",
		},
		{
			name:    "leading whitespace is tolerated",
			actions: []string{"  " + testAccelScrollUp + "  "},
			want:    "scroll_up",
		},
		{
			name:    "quoted name is unquoted",
			actions: []string{`action "scroll_up"`},
			want:    "scroll_up",
		},
		{
			name:    "multi-action binding has no single name",
			actions: []string{testAccelScrollDown, testAccelScrollUp},
			want:    "",
		},
		{
			name:    "mode switch is not an action binding",
			actions: []string{"hints --action left_click"},
			want:    "",
		},
		{
			name:    "exec binding is not an action binding",
			actions: []string{"exec echo test"},
			want:    "",
		},
		{
			name:    "empty list",
			actions: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.HeldRepeatActionName(tt.actions)
			if got != tt.want {
				t.Errorf("HeldRepeatActionName(%v) = %q, want %q", tt.actions, got, tt.want)
			}
		})
	}
}
