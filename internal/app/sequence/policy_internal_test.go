package sequence

import "testing"

const leftClickStep = "action left_click"

func TestSplitBailOnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		step      string
		wantStep  string
		wantFatal bool
		wantErr   bool
	}{
		{
			name:      "trailing directive is consumed",
			step:      "action left_click " + BailOnErrorFlag,
			wantStep:  leftClickStep,
			wantFatal: true,
		},
		{
			name:     "no directive",
			step:     leftClickStep,
			wantStep: leftClickStep,
		},
		{
			// The text appears inside a quoted argument, so it belongs to the
			// step rather than to the sequence.
			name:     "quoted text is not a directive",
			step:     `exec sh -c "echo ` + BailOnErrorFlag + `"`,
			wantStep: `exec sh -c "echo ` + BailOnErrorFlag + `"`,
		},
		{
			// Quoted as the final argument, so the author wants the text, not the
			// directive, and the step must reach the action unchanged.
			name:     "quoted final token is an argument",
			step:     `exec printf '` + BailOnErrorFlag + `'`,
			wantStep: `exec printf '` + BailOnErrorFlag + `'`,
		},
		{
			name:    "directive in the middle is rejected",
			step:    "action left_click " + BailOnErrorFlag + " --bare",
			wantErr: true,
		},
		{
			name:     "directive alone is not a step",
			step:     BailOnErrorFlag,
			wantStep: BailOnErrorFlag,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotStep, gotFatal, gotErr := splitBailOnError(testCase.step)

			if (gotErr != nil) != testCase.wantErr {
				t.Fatalf("error = %v, wantErr = %v", gotErr, testCase.wantErr)
			}

			if testCase.wantErr {
				return
			}

			if gotStep != testCase.wantStep || gotFatal != testCase.wantFatal {
				t.Fatalf("splitBailOnError(%q) = (%q, %v), want (%q, %v)",
					testCase.step, gotStep, gotFatal, testCase.wantStep, testCase.wantFatal)
			}
		})
	}
}
