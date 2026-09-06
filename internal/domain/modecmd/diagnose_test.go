package modecmd_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// modeNameGrid is the mode's own name, which a caller modeled on the CLI's
// traffic repeats as the first argument.
const modeNameGrid = "grid"

// TestDiagnose_WarnsAboutACommandThatStillRuns pins the faults a configuration
// is told about rather than refused for: a flag the mode has no use for, and a
// flag written without the one it depends on. Both describe a binding that
// works minus one flag, and refusing the load would cost the user every other
// binding they wrote.
func TestDiagnose_WarnsAboutACommandThatStillRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
		want []string
	}{
		{
			name: "flag the mode does not accept",
			mode: domain.ModeGrid,
			args: []string{argSearch},
			want: []string{msgGridRejectsSearch},
		},
		{
			// The value belongs to the flag that was dropped, so it is stepped
			// over. Left where it was, "vision" would be read as the positional
			// action and the command would be refused for naming one no mode
			// can perform.
			name: "value of a flag the mode does not accept",
			mode: domain.ModeGrid,
			args: []string{flagStrategy, "vision"},
			want: []string{"grid does not accept --strategy"},
		},
		{
			// The flag after it is not its value: stepping over "--search"
			// here would drop a flag in silence, which is the whole failure
			// this package exists to remove.
			name: "a flag written where the dropped flag's value would be",
			mode: domain.ModeGrid,
			args: []string{flagStrategy, argSearch},
			want: []string{
				"grid does not accept --strategy",
				msgGridRejectsSearch,
			},
		},
		{
			name: "action on a mode that makes no selection",
			mode: domain.ModeScroll,
			args: []string{flagAction, leftClick},
			want: []string{"scroll does not accept --action"},
		},
		{
			name: "repeat without action",
			mode: domain.ModeHints,
			args: []string{flagRepeat},
			want: []string{msgRepeatNeedsAction},
		},
		{
			name: "on-exit without action",
			mode: domain.ModeHints,
			args: []string{"--on-exit=" + stepLeftClick},
			want: []string{
				"--on-exit requires --action (it runs only when the action is fulfilled)",
			},
		},
		{
			name: "modifier without action",
			mode: domain.ModeHints,
			args: []string{argModifierCmd},
			want: []string{"--modifier requires --action"},
		},
		{
			name: "hide-on-empty-search without search",
			mode: domain.ModeHints,
			args: []string{flagHideOnEmpty},
			want: []string{"--hide-on-empty-search requires --search"},
		},
		{
			// Every fault is reported, not just the first: a user fixing their
			// configuration should see the whole of what is wrong with a
			// binding rather than one fault per edit.
			name: "several at once",
			mode: domain.ModeGrid,
			args: []string{argSearch, argZoomToDepth2, flagRepeat},
			want: []string{
				msgGridRejectsSearch,
				"grid does not accept --zoom-to-depth",
				msgRepeatNeedsAction,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, warnings, err := modecmd.Diagnose(testCase.mode, testCase.args)
			if err != nil {
				t.Fatalf("Diagnose(%v) refused the command: %v", testCase.args, err)
			}

			if got := messages(warnings); !slices.Equal(got, testCase.want) {
				t.Errorf("warnings = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestDiagnose_RefusesACommandThatCouldNotRun pins the other side of the split:
// a command naming a flag nothing knows, or a value no flag can carry, never
// activated anything, so refusing the configuration takes nothing away that
// worked.
func TestDiagnose_RefusesACommandThatCouldNotRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
		want string
	}{
		{
			name: "a flag nothing knows",
			mode: domain.ModeHints,
			args: []string{flagMistyped, "--action=bogus"},
			want: "unknown flag: --serach",
		},
		{
			name: "stray argument",
			mode: domain.ModeHints,
			args: []string{leftClick, "stray"},
			want: "unexpected argument: stray",
		},
		{
			name: "argument to a mode that takes none",
			mode: domain.ModeIdle,
			args: []string{leftClick},
			want: "unexpected argument: left_click",
		},
		{
			name: "missing value",
			mode: domain.ModeHints,
			args: []string{flagAction},
			want: "--action requires a value",
		},
		{
			name: "unusable value",
			mode: domain.ModeHints,
			args: []string{argBadStrategy},
			want: msgStrategyValue,
		},
		{
			name: "empty modifier list",
			mode: domain.ModeHints,
			args: []string{argAction, "--modifier=,"},
			want: "modifier values cannot be empty",
		},
		{
			// The command cannot run, so there is nothing left to weigh: the
			// inert --repeat is not reported alongside it.
			name: "unusable action outweighs a warning",
			mode: domain.ModeHints,
			args: []string{flagRepeat, "--action=nonsense"},
			want: "invalid action: nonsense",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, warnings, err := modecmd.Diagnose(testCase.mode, testCase.args)
			if err == nil {
				t.Fatalf("Diagnose(%v) was accepted; want a refusal", testCase.args)
			}

			// A prefix, so the action vocabulary can be pinned without
			// repeating the whole list of action names it ends with.
			if got := message(err); !strings.HasPrefix(got, testCase.want) {
				t.Errorf("message = %q, want it to start with %q", got, testCase.want)
			}

			if len(warnings) > 0 {
				t.Errorf("warnings = %q, want none alongside a refusal", messages(warnings))
			}
		})
	}
}

// TestDiagnose_AcceptsACleanCommand pins that a command with nothing wrong with
// it produces neither, for every mode. A configuration made only of these is
// what "valid" has to keep meaning.
func TestDiagnose_AcceptsACleanCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode domain.Mode
		args []string
	}{
		{domain.ModeHints, nil},
		{domain.ModeHints, []string{argAction, argSearch, flagHideOnEmpty, argModifierCmd}},
		{domain.ModeGrid, []string{modeNameGrid, argAction, argOnExitStep}},
		{domain.ModeRecursiveGrid, []string{argZoomToDepth2, argToggle}},
		{domain.ModeScroll, []string{argToggle}},
		{domain.ModeMonitorSelect, []string{argToggle}},
		{domain.ModeIdle, nil},
	}

	for _, testCase := range tests {
		t.Run(domain.ModeString(testCase.mode), func(t *testing.T) {
			t.Parallel()

			_, warnings, err := modecmd.Diagnose(testCase.mode, testCase.args)
			if err != nil {
				t.Fatalf("Diagnose(%v) refused a clean command: %v", testCase.args, err)
			}

			if len(warnings) > 0 {
				t.Errorf("warnings = %q, want none", messages(warnings))
			}
		})
	}
}

// TestDiagnose_SaysWhatParseSays pins that weighing a fault does not reword it:
// the sentence a configuration prints is the one the daemon and the command
// line give for the same mistake.
func TestDiagnose_SaysWhatParseSays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
	}{
		{name: "unsupported flag", mode: domain.ModeGrid, args: []string{argSearch}},
		{name: "unmet dependency", mode: domain.ModeHints, args: []string{flagRepeat}},
		{name: "a flag nothing knows", mode: domain.ModeHints, args: []string{flagMistyped}},
		{name: "unusable value", mode: domain.ModeHints, args: []string{argBadStrategy}},
		{
			name: "unusable action",
			mode: domain.ModeHints,
			args: []string{"--action=move_mouse"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, warnings, diagnoseErr := modecmd.Diagnose(testCase.mode, testCase.args)

			_, parseErr := modecmd.Parse(testCase.mode, testCase.args)
			if parseErr == nil {
				t.Fatalf("Parse(%v) was accepted; want a refusal", testCase.args)
			}

			got := ""

			switch {
			case diagnoseErr != nil:
				got = message(diagnoseErr)
			case len(warnings) > 0:
				got = message(warnings[0])
			default:
				t.Fatalf(
					"Diagnose(%v) reported nothing; want the fault Parse refuses",
					testCase.args,
				)
			}

			if want := message(parseErr); got != want {
				t.Errorf("Diagnose says %q, Parse says %q", got, want)
			}
		})
	}
}

// messages returns what each reported fault says, without the error code.
func messages(reported []error) []string {
	said := make([]string, 0, len(reported))
	for _, one := range reported {
		said = append(said, message(one))
	}

	return said
}
