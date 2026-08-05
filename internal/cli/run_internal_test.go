package cli

import (
	"slices"
	"testing"
)

// runTestStep is the step these tests reuse across cases.
const runTestStep = "hints"

func TestValidateRunArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "single step", args: []string{"action left_click"}},
		{name: "several steps", args: []string{"action save_cursor_pos", runTestStep}},
		{name: "no steps", args: nil, wantErr: true},
		{name: "blank step", args: []string{runTestStep, "   "}, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validateRunArgs(nil, testCase.args)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("validateRunArgs(%v) error = %v, wantErr = %v",
					testCase.args, err, testCase.wantErr)
			}
		})
	}
}

// A step's padding is not part of the step, so it is trimmed before the step
// travels. A step that is nothing but padding is kept: an empty --on-exit says
// to clear whatever a previous activation stored and run nothing in its place,
// which the grammar reads the same way wherever it is written.
func TestTrimOnExitSteps(t *testing.T) {
	t.Parallel()

	steps := trimOnExitSteps([]string{" action sleep 0.2 ", runTestStep, "  "})

	want := []string{"action sleep 0.2", runTestStep, ""}
	if !slices.Equal(steps, want) {
		t.Fatalf("trimOnExitSteps() = %v, want %v", steps, want)
	}
}
