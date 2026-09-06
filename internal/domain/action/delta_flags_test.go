package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

func TestParseDeltaFlags(t *testing.T) {
	tests := []struct {
		name   string
		in     []string
		dx, dy int
		ok     bool
	}{
		{
			"joined flags",
			[]string{"action", "move_mouse_relative", "--dx=10", "--dy=-5"},
			10,
			-5,
			true,
		},
		{
			"spaced flags",
			[]string{"action", "move_mouse_relative", "--dx", "10", "--dy", "0"},
			10,
			0,
			true,
		},
		{"missing dy", []string{"action", "move_mouse_relative", "--dx=10"}, 0, 0, false},
		{"non-integer", []string{"action", "move_mouse_relative", "--dx=a", "--dy=1"}, 0, 0, false},
		{
			"trailing flag without value",
			[]string{"action", "move_mouse_relative", "--dy=1", "--dx"},
			0,
			0,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dx, dy, ok := action.ParseDeltaFlags(tt.in)
			if dx != tt.dx || dy != tt.dy || ok != tt.ok {
				t.Errorf("ParseDeltaFlags(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tt.in, dx, dy, ok, tt.dx, tt.dy, tt.ok)
			}
		})
	}
}
