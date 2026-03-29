//nolint:testpackage // Tests private hotkey helper behavior.
package app

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestHotkeyModifiersFromKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want action.Modifiers
	}{
		{
			name: "cmd shift key",
			key:  "Cmd+Shift+C",
			want: action.ModCmd | action.ModShift,
		},
		{
			name: "left right aliases",
			key:  "LeftCmd+RightShift+Space",
			want: action.ModCmd | action.ModShift,
		},
		{
			name: "all modifiers with option alias",
			key:  "Command+Option+Ctrl+Shift+K",
			want: action.ModCmd | action.ModAlt | action.ModCtrl | action.ModShift,
		},
		{
			name: "plain key has no modifiers",
			key:  "Escape",
			want: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := hotkeyModifiersFromKey(testCase.key)
			if got != testCase.want {
				t.Fatalf(
					"hotkeyModifiersFromKey(%q) = %v, want %v",
					testCase.key,
					got,
					testCase.want,
				)
			}
		})
	}
}
