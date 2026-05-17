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
		{
			name: "primary alias follows current platform",
			key:  "Primary+Space",
			want: action.PrimaryModifier(),
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

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "plain split, no quotes",
			input: `action move_monitor --previous`,
			want:  []string{"action", "move_monitor", "--previous"},
		},
		{
			name:  "double-quoted monitor name with space",
			input: `action move_monitor --name "DELL U2720Q"`,
			want:  []string{"action", "move_monitor", "--name", "DELL U2720Q"},
		},
		{
			name:  "single-quoted monitor name",
			input: `action move_monitor --name 'Built-in Retina Display'`,
			want:  []string{"action", "move_monitor", "--name", "Built-in Retina Display"},
		},
		{
			name:  "equals form with double quotes",
			input: `action move_monitor --name="DELL U2720Q"`,
			want:  []string{"action", "move_monitor", "--name=DELL U2720Q"},
		},
		{
			name:  "single quote literal inside double quotes",
			input: `action move_monitor --name "It's a Monitor"`,
			want:  []string{"action", "move_monitor", "--name", "It's a Monitor"},
		},
		{
			name:  "unclosed single quote is treated as closed token",
			input: `action move_monitor --name 'DELL`,
			want:  []string{"action", "move_monitor", "--name", "DELL"},
		},
		{
			name:  "unclosed double quote is treated as closed token",
			input: `action move_monitor --name "DELL`,
			want:  []string{"action", "move_monitor", "--name", "DELL"},
		},
		{
			name:  "empty string returns empty slice",
			input: ``,
			want:  []string{},
		},
		{
			name:  "multiple spaces are collapsed",
			input: `action   move_monitor   --previous`,
			want:  []string{"action", "move_monitor", "--previous"},
		},
		{
			name:  "trailing space produces trailing empty token ignored",
			input: `action move_monitor --previous `,
			want:  []string{"action", "move_monitor", "--previous"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := splitArgs(testCase.input)
			if len(got) != len(testCase.want) {
				t.Fatalf(
					"splitArgs(%q) returned %d args, want %d: %v",
					testCase.input,
					len(got),
					len(testCase.want),
					got,
				)
			}

			for idx := range got {
				if got[idx] != testCase.want[idx] {
					t.Fatalf(
						"splitArgs(%q)[%d] = %q, want %q",
						testCase.input,
						idx,
						got[idx],
						testCase.want[idx],
					)
				}
			}
		})
	}
}

func TestHotkeyActionsRepeatWhileHeld(t *testing.T) {
	app := &App{}

	tests := []struct {
		name    string
		actions []string
		want    bool
	}{
		{
			name:    "scroll down repeats",
			actions: []string{"action scroll_down"},
			want:    true,
		},
		{
			name:    "page down repeats",
			actions: []string{"action page_down"},
			want:    true,
		},
		{
			name:    "relative mouse movement repeats",
			actions: []string{"action move_mouse_relative --dx=0 --dy=10"},
			want:    true,
		},
		{
			name:    "mode launcher does not repeat",
			actions: []string{"scroll"},
			want:    false,
		},
		{
			name:    "absolute terminal scroll does not repeat",
			actions: []string{"action go_bottom"},
			want:    false,
		},
		{
			name:    "click does not repeat",
			actions: []string{"action left_click"},
			want:    false,
		},
		{
			name:    "exec does not repeat",
			actions: []string{"exec echo hello"},
			want:    false,
		},
		{
			name:    "chains do not repeat",
			actions: []string{"action scroll_down", "action scroll_down"},
			want:    false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := app.hotkeyActionsRepeatWhileHeld(testCase.actions)
			if got != testCase.want {
				t.Fatalf(
					"hotkeyActionsRepeatWhileHeld(%v) = %v, want %v",
					testCase.actions,
					got,
					testCase.want,
				)
			}
		})
	}
}
