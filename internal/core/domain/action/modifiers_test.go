package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestParseModifiers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    action.Modifiers
		wantErr bool
	}{
		{name: "empty string", input: "", want: 0},
		{name: "single cmd", input: "cmd", want: action.ModCmd},
		{name: "single shift", input: "shift", want: action.ModShift},
		{name: "cmd and shift", input: "cmd,shift", want: action.ModCmd | action.ModShift},
		{
			name:  "all modifiers",
			input: "cmd,shift,alt,ctrl",
			want:  action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl,
		},
		{
			name:  "aliases command and option",
			input: "command,option",
			want:  action.ModCmd | action.ModAlt,
		},
		{name: "alias control", input: "control", want: action.ModCtrl},
		{name: "primary alias", input: "primary", want: action.PrimaryModifier()},
		{name: "case insensitive", input: "CMD,Shift", want: action.ModCmd | action.ModShift},
		{name: "whitespace trimmed", input: " cmd , shift ", want: action.ModCmd | action.ModShift},
		{name: "duplicate modifiers", input: "cmd,cmd", want: action.ModCmd},
		{name: "unknown modifier", input: "cmd,whatever", wantErr: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := action.ParseModifiers(testCase.input)
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"ParseModifiers(%q) error = %v, wantErr %v",
					testCase.input,
					err,
					testCase.wantErr,
				)

				return
			}

			if got != testCase.want {
				t.Errorf("ParseModifiers(%q) = %d, want %d", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestModifiers_Has(t *testing.T) {
	mods := action.ModCmd | action.ModShift
	if !mods.Has(action.ModCmd) {
		t.Error("Expected Has(ModCmd) to be true")
	}

	if !mods.Has(action.ModShift) {
		t.Error("Expected Has(ModShift) to be true")
	}

	if mods.Has(action.ModAlt) {
		t.Error("Expected Has(ModAlt) to be false")
	}
}

func TestModifiers_String(t *testing.T) {
	tests := []struct {
		name string
		mods action.Modifiers
		want string
	}{
		{"zero", 0, ""},
		{"cmd only", action.ModCmd, "Cmd"},
		{"cmd+shift", action.ModCmd | action.ModShift, "Cmd+Shift"},
		{
			"all",
			action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl,
			"Cmd+Shift+Alt+Ctrl",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.mods.String()
			if got != testCase.want {
				t.Errorf("Modifiers.String() = %q, want %q", got, testCase.want)
			}
		})
	}
}
