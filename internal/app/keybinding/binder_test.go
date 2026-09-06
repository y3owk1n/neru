package keybinding

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	actionScrollDown     = "action scroll_down"
	leftClickStep        = "action left_click"
	hintsStep            = "hints --action left_click"
	builtInRetinaDisplay = "Built-in Retina Display"
	actionCommand        = "action"
	moveMonitorAction    = "move_monitor"
	flagPreviousArg      = "--previous"
	flagNameArg          = "--name"
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
			got := ModifiersFromKey(testCase.key)
			if got != testCase.want {
				t.Fatalf(
					"ModifiersFromKey(%q) = %v, want %v",
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
			want:  []string{actionCommand, moveMonitorAction, flagPreviousArg},
		},
		{
			name:  "double-quoted monitor name with space",
			input: `action move_monitor --name "DELL U2720Q"`,
			want:  []string{actionCommand, moveMonitorAction, flagNameArg, "DELL U2720Q"},
		},
		{
			name:  "single-quoted monitor name",
			input: `action move_monitor --name 'Built-in Retina Display'`,
			want:  []string{actionCommand, moveMonitorAction, flagNameArg, builtInRetinaDisplay},
		},
		{
			name:  "equals form with double quotes",
			input: `action move_monitor --name="DELL U2720Q"`,
			want:  []string{actionCommand, moveMonitorAction, "--name=DELL U2720Q"},
		},
		{
			name:  "single quote literal inside double quotes",
			input: `action move_monitor --name "It's a Monitor"`,
			want:  []string{actionCommand, moveMonitorAction, flagNameArg, "It's a Monitor"},
		},
		{
			name:  "unclosed single quote is treated as closed token",
			input: `action move_monitor --name 'DELL`,
			want:  []string{actionCommand, moveMonitorAction, flagNameArg, "DELL"},
		},
		{
			name:  "unclosed double quote is treated as closed token",
			input: `action move_monitor --name "DELL`,
			want:  []string{actionCommand, moveMonitorAction, flagNameArg, "DELL"},
		},
		{
			name:  "empty string returns empty slice",
			input: ``,
			want:  []string{},
		},
		{
			name:  "multiple spaces are collapsed",
			input: `action   move_monitor   --previous`,
			want:  []string{actionCommand, moveMonitorAction, flagPreviousArg},
		},
		{
			name:  "trailing space produces trailing empty token ignored",
			input: `action move_monitor --previous `,
			want:  []string{actionCommand, moveMonitorAction, flagPreviousArg},
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
	app := &Binder{}

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
			actions: []string{leftClickStep},
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
	app := &Binder{}

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

	cfg := config.DefaultConfig()
	cfg.Macros = map[string]config.StringOrStringArray{
		"open_hints": {"action save_cursor_pos", hintsStep},
		"just_click": {leftClickStep},
	}

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
			// A declared mode is entered by the mode word, so the chord that
			// enters it has its modifiers suppressed like any other.
			name:           "declared mode by the mode word",
			actions:        []string{"mode window"},
			wantModeSwitch: true,
		},
		{
			// Idle is a mode command that leaves a mode, and nothing about the
			// chord that left needs suppressing.
			name:           "idle leaves rather than enters",
			actions:        []string{"idle"},
			wantModeSwitch: false,
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
			// A macro body is as much part of what the binding does as an
			// inline step is.
			name:           "mode inside a macro",
			actions:        []string{"macro open_hints"},
			wantModeSwitch: true,
		},
		{
			name:           "macro without a mode",
			actions:        []string{"macro just_click"},
			wantModeSwitch: false,
		},
		{
			name:           "unknown macro names no mode",
			actions:        []string{"macro not_defined"},
			wantModeSwitch: false,
		},
		{
			name:           "run without a mode",
			actions:        []string{"run 'action left_click' 'action sleep 0.2'"},
			wantModeSwitch: false,
		},
		{
			name:           "nested run without a mode",
			actions:        []string{`run 'run leftClickStep' 'action sleep 0.2'`},
			wantModeSwitch: false,
		},
		{
			name:           "no mode at all",
			actions:        []string{leftClickStep, "exec true"},
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

			if got := actionsContainModeSwitch(
				testCase.actions,
				cfg,
			); got != testCase.wantModeSwitch {
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
	if !ActionsReferenceDisabledMode(actions, cfg) {
		t.Fatal("a disabled mode inside a run step should be detected")
	}

	cfg.Hints.Enabled = true

	if ActionsReferenceDisabledMode(actions, cfg) {
		t.Fatal("an enabled mode inside a run step should not be reported as disabled")
	}
}

// The walk must stop where the executor does: past the nesting limit a
// sequence is refused, so its steps never run and must not be reported as
// things the binding does.
func TestAnyBindingStep_StopsAtTheExecutorsNestingLimit(t *testing.T) {
	t.Parallel()

	mode := domain.ModeString(domain.ModeHints)
	names := func(step string) bool { return step == mode }

	budget := maxInspectedSteps
	if !anyBindingStepAtDepth([]string{"run '" + mode + "'"}, nil, 0, &budget, names) {
		t.Fatal("below the limit: expected the nested step to be visited")
	}

	budget = maxInspectedSteps
	if anyBindingStepAtDepth(
		[]string{"run '" + mode + "'"},
		nil,
		sequence.MaxDepth,
		&budget,
		names,
	) {
		t.Fatal("at the limit: expected the run to be left unexpanded")
	}
}

// A binding whose macros fan out into each other multiplies at every level,
// and this runs on the key-press path, so the walk is bounded rather than
// trusted.
func TestAnyBindingStep_IsBounded(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()

	// Each level expands into eight calls to the next, which without a budget
	// would visit thousands of steps before answering.
	fanOut := func(next string) config.StringOrStringArray {
		steps := make([]string, 0, 8)
		for range 8 {
			steps = append(steps, "macro "+next)
		}

		return steps
	}

	cfg.Macros = map[string]config.StringOrStringArray{
		"a": fanOut("b"),
		"b": fanOut("c"),
		"c": fanOut("d"),
		"d": {leftClickStep},
	}

	visited := 0
	counted := func(string) bool {
		visited++

		return false
	}

	if anyBindingStep([]string{"macro a"}, cfg, counted) {
		t.Fatal("no step names a mode, want false")
	}

	if visited > maxInspectedSteps {
		t.Fatalf("visited %d steps, want no more than the %d budget", visited, maxInspectedSteps)
	}
}
