package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	testMoveBinding = "action move_mouse_relative --dx=20 --dy=0"
	testScrollDown  = "action scroll_down"
)

func TestScaleDeltaFlags(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		multiplier float64
		want       string
	}{
		{
			name:       "joined form scales both axes",
			input:      "action move_mouse_relative --dx=20 --dy=-20",
			multiplier: 2,
			want:       "action move_mouse_relative --dx=40 --dy=-40",
		},
		{
			name:       "separated form scales both axes",
			input:      "action move_mouse_relative --dx 20 --dy -20",
			multiplier: 3,
			want:       "action move_mouse_relative --dx 60 --dy -60",
		},
		{
			name:       "fractional multiplier rounds to nearest",
			input:      testMoveBinding,
			multiplier: 1.55,
			want:       "action move_mouse_relative --dx=31 --dy=0",
		},
		{
			name:       "multiplier of one is a no-op",
			input:      testMoveBinding,
			multiplier: 1,
			want:       testMoveBinding,
		},
		{
			name:       "action without delta flags is untouched",
			input:      testScrollDown,
			multiplier: 4,
			want:       testScrollDown,
		},
		{
			name:       "unrelated flags are preserved",
			input:      "action move_mouse_relative --dx=10 --bail --dy=10",
			multiplier: 2,
			want:       "action move_mouse_relative --dx=20 --bail --dy=20",
		},
		{
			name:       "non-numeric value is left alone",
			input:      "action move_mouse_relative --dx=abc",
			multiplier: 2,
			want:       "action move_mouse_relative --dx=abc",
		},
		{
			name:       "overflowing delta saturates instead of wrapping",
			input:      "action move_mouse_relative --dx=9223372036854775807",
			multiplier: 100,
			want:       "action move_mouse_relative --dx=9223372036854775807",
		},
		{
			name:       "overflowing negative delta saturates instead of wrapping",
			input:      "action move_mouse_relative --dx=-9223372036854775808",
			multiplier: 100,
			want:       "action move_mouse_relative --dx=-9223372036854775808",
		},
		{
			name:       "trailing bare flag without value is left alone",
			input:      "action move_mouse_relative --dx",
			multiplier: 2,
			want:       "action move_mouse_relative --dx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := action.ScaleDeltaFlags(tt.input, tt.multiplier)
			if got != tt.want {
				t.Errorf("ScaleDeltaFlags(%q, %v) = %q, want %q",
					tt.input, tt.multiplier, got, tt.want)
			}
		})
	}
}

func TestScaleAllDeltaFlags(t *testing.T) {
	tests := []struct {
		name       string
		actions    []string
		multiplier float64
		want       []string
	}{
		{
			name:       "scales every entry that carries delta flags",
			actions:    []string{testMoveBinding, "action move_mouse_relative --dx=0 --dy=-5"},
			multiplier: 2,
			want: []string{
				"action move_mouse_relative --dx=40 --dy=0",
				"action move_mouse_relative --dx=0 --dy=-10",
			},
		},
		{
			name:       "leaves entries without delta flags alone",
			actions:    []string{testMoveBinding, testScrollDown},
			multiplier: 2,
			want:       []string{"action move_mouse_relative --dx=40 --dy=0", testScrollDown},
		},
		{
			name:       "multiplier of one returns the input untouched",
			actions:    []string{testMoveBinding},
			multiplier: 1,
			want:       []string{testMoveBinding},
		},
		{
			name:       "empty list stays empty",
			actions:    []string{},
			multiplier: 2,
			want:       []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := action.ScaleAllDeltaFlags(testCase.actions, testCase.multiplier)
			if len(got) != len(testCase.want) {
				t.Fatalf("ScaleAllDeltaFlags(%v, %v) = %v, want %v",
					testCase.actions, testCase.multiplier, got, testCase.want)
			}

			for idx := range got {
				if got[idx] != testCase.want[idx] {
					t.Errorf("ScaleAllDeltaFlags(%v, %v)[%d] = %q, want %q",
						testCase.actions, testCase.multiplier, idx, got[idx], testCase.want[idx])
				}
			}
		})
	}
}

func TestScaleAllDeltaFlagsDoesNotMutateInput(t *testing.T) {
	actions := []string{testMoveBinding}

	first := action.ScaleAllDeltaFlags(actions, 2)
	second := action.ScaleAllDeltaFlags(actions, 2)

	if actions[0] != testMoveBinding {
		t.Fatalf("input was mutated: %q", actions[0])
	}

	if first[0] != second[0] {
		t.Errorf("repeated scaling diverged: %q vs %q", first[0], second[0])
	}
}
