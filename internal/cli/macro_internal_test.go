package cli

import "testing"

func TestValidateMacroArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "name only", args: []string{"window_click"}},
		{name: "name with args", args: []string{"zoom_click", "3"}},
		{name: "dashes and digits", args: []string{"zoom-click2"}},
		// The arguments are the macro's, not the command's, so they are passed
		// through whatever they contain.
		{name: "argument with spaces", args: []string{"say_it", "hello there"}},
		{name: "argument that looks like a flag", args: []string{"say_it", "--bail-on-error"}},
		{name: "no name", args: nil, wantErr: true},
		{name: "blank name", args: []string{"   "}, wantErr: true},
		{name: "name with a space", args: []string{"window click"}, wantErr: true},
		{name: "name starting with a digit", args: []string{"9lives"}, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validateMacroArgs(nil, testCase.args)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("validateMacroArgs(%v) error = %v, wantErr %t",
					testCase.args, err, testCase.wantErr)
			}
		})
	}
}
