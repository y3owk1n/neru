//nolint:testpackage // Tests private hotkey helper behavior.
package app

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const (
	actionScrollDown     = "action scroll_down"
	builtInRetinaDisplay = "Built-in Retina Display"
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
			want:  []string{actionCmd, moveMonitor, flagPrevious},
		},
		{
			name:  "double-quoted monitor name with space",
			input: `action move_monitor --name "DELL U2720Q"`,
			want:  []string{actionCmd, moveMonitor, flagName, "DELL U2720Q"},
		},
		{
			name:  "single-quoted monitor name",
			input: `action move_monitor --name 'Built-in Retina Display'`,
			want:  []string{actionCmd, moveMonitor, flagName, builtInRetinaDisplay},
		},
		{
			name:  "equals form with double quotes",
			input: `action move_monitor --name="DELL U2720Q"`,
			want:  []string{actionCmd, moveMonitor, "--name=DELL U2720Q"},
		},
		{
			name:  "single quote literal inside double quotes",
			input: `action move_monitor --name "It's a Monitor"`,
			want:  []string{actionCmd, moveMonitor, flagName, "It's a Monitor"},
		},
		{
			name:  "unclosed single quote is treated as closed token",
			input: `action move_monitor --name 'DELL`,
			want:  []string{actionCmd, moveMonitor, flagName, "DELL"},
		},
		{
			name:  "unclosed double quote is treated as closed token",
			input: `action move_monitor --name "DELL`,
			want:  []string{actionCmd, moveMonitor, flagName, "DELL"},
		},
		{
			name:  "empty string returns empty slice",
			input: ``,
			want:  []string{},
		},
		{
			name:  "multiple spaces are collapsed",
			input: `action   move_monitor   --previous`,
			want:  []string{actionCmd, moveMonitor, flagPrevious},
		},
		{
			name:  "trailing space produces trailing empty token ignored",
			input: `action move_monitor --previous `,
			want:  []string{actionCmd, moveMonitor, flagPrevious},
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

	cfg := config.DefaultConfig()
	cfg.HeldRepeat.Enabled = true

	tests := []struct {
		name    string
		actions []string
		want    bool
	}{
		{
			name:    "scroll down repeats",
			actions: []string{actionScrollDown},
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
			actions: []string{actionScrollDown, actionScrollDown},
			want:    false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := app.hotkeyActionsRepeatWhileHeld(testCase.actions, cfg)
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

func TestHotkeyActionsRepeatWhileHeldDisabled(t *testing.T) {
	app := &App{}

	cfg := config.DefaultConfig()

	tests := []struct {
		name    string
		actions []string
	}{
		{
			name:    "scroll down does not repeat when disabled",
			actions: []string{actionScrollDown},
		},
		{
			name:    "relative mouse movement does not repeat when disabled",
			actions: []string{"action move_mouse_relative --dx=0 --dy=10"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := app.hotkeyActionsRepeatWhileHeld(testCase.actions, cfg)
			if got {
				t.Fatalf(
					"hotkeyActionsRepeatWhileHeld(%v) with Enabled=false = %v, want false",
					testCase.actions,
					got,
				)
			}
		})
	}
}

// A binding may name a mode directly or carry it inside a "run" sequence.
// Both spellings have to be visible to the callers that inspect a binding
// before dispatching it, or a mode switch written as a sequence would skip
// modifier suppression and the disabled-mode check.
func TestBindingInspectionSeesModesInsideRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		actions        []string
		wantModeSwitch bool
	}{
		{
			name:           "direct mode",
			actions:        []string{hintsStep},
			wantModeSwitch: true,
		},
		{
			name:           "mode inside run",
			actions:        []string{"run 'action save_cursor_pos' '" + hintsStep + "'"},
			wantModeSwitch: true,
		},
		{
			// A run step may itself be a run, and the executor follows it, so a
			// mode below the first level must still be visible here.
			name:           "mode inside a nested run",
			actions:        []string{`run 'run "hints"' 'action left_click'`},
			wantModeSwitch: true,
		},
		{
			name:           "run without a mode",
			actions:        []string{"run 'action left_click' 'action sleep 0.2'"},
			wantModeSwitch: false,
		},
		{
			name:           "nested run without a mode",
			actions:        []string{`run 'run "action left_click"' 'action sleep 0.2'`},
			wantModeSwitch: false,
		},
		{
			name:           "no mode at all",
			actions:        []string{"action left_click", "exec true"},
			wantModeSwitch: false,
		},
		{
			name:           "blank actions",
			actions:        []string{"", "   "},
			wantModeSwitch: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := actionsContainModeSwitch(testCase.actions); got != testCase.wantModeSwitch {
				t.Fatalf("actionsContainModeSwitch(%v) = %v, want %v",
					testCase.actions, got, testCase.wantModeSwitch)
			}
		})
	}
}

func TestActionsReferenceDisabledModeSeesModesInsideRun(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = false

	actions := []string{"run 'action save_cursor_pos' '" + hintsStep + "'"}
	if !actionsReferenceDisabledMode(actions, cfg) {
		t.Fatal("a disabled mode inside a run step should be detected")
	}

	cfg.Hints.Enabled = true

	if actionsReferenceDisabledMode(actions, cfg) {
		t.Fatal("an enabled mode inside a run step should not be reported as disabled")
	}
}

// The expansion must stop where the executor does: past the nesting limit a
// sequence is refused, so its steps never run and must not be reported as
// things the binding does.
func TestFlattenRunSteps_StopsAtTheExecutorsNestingLimit(t *testing.T) {
	t.Parallel()

	mode := domain.ModeString(domain.ModeHints)
	nested := []string{"run '" + mode + "'"}

	got := flattenRunStepsAtDepth(nested, 0)
	if !slices.Equal(got, []string{mode}) {
		t.Fatalf("below the limit: got %v, want the nested step expanded", got)
	}

	got = flattenRunStepsAtDepth(nested, maxSequenceDepth)
	if !slices.Equal(got, nested) {
		t.Fatalf("at the limit: got %v, want the run left unexpanded", got)
	}
}
