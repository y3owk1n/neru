package cli

import (
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
