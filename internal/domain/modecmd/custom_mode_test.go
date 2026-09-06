package modecmd_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// The spellings a user-declared mode is entered with.
const (
	modeNameWindow = "window"
	stepModeWindow = "mode window"
)

// TestParse_ReadsTheCustomModeName pins where the declared name is read from:
// the first bare argument, in both shapes the wire carries. A binding writes
// "mode window --toggle" and the CLI repeats the command word, and both have to
// reach the same activation or a script and a binding would enter different
// modes.
func TestParse_ReadsTheCustomModeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want modecmd.Activation
	}{
		{
			name: "name alone",
			args: []string{modeNameWindow},
			want: modecmd.Activation{Mode: domain.ModeCustom, Name: modeNameWindow},
		},
		{
			name: "name then a flag",
			args: []string{modeNameWindow, argToggle},
			want: modecmd.Activation{
				Mode:   domain.ModeCustom,
				Name:   modeNameWindow,
				Toggle: new(true),
			},
		},
		{
			name: "flag then the name",
			args: []string{argToggle, modeNameWindow},
			want: modecmd.Activation{
				Mode:   domain.ModeCustom,
				Name:   modeNameWindow,
				Toggle: new(true),
			},
		},
		{
			name: "the command word repeated, as the CLI sends it",
			args: []string{domain.ModeNameCustom, modeNameWindow},
			want: modecmd.Activation{Mode: domain.ModeCustom, Name: modeNameWindow},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := modecmd.Parse(domain.ModeCustom, testCase.args)
			if err != nil {
				t.Fatalf("Parse(%v) error = %v", testCase.args, err)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Parse(%v) = %+v, want %+v", testCase.args, got, testCase.want)
			}
		})
	}
}

// TestParse_RefusesACustomActivationWithoutAName pins that the word alone
// names nothing, and that a second bare word is a stray argument rather than a
// positional action: a declared mode makes no selection to act on.
func TestParse_RefusesACustomActivationWithoutAName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no arguments",
			args: nil,
			want: "mode requires the name of a declared mode",
		},
		{
			name: "only the command word",
			args: []string{domain.ModeNameCustom},
			want: "mode requires the name of a declared mode",
		},
		{
			name: "a name no mode may have",
			args: []string{"9lives"},
			want: "a mode name starts with a letter",
		},
		{
			name: "a second bare word",
			args: []string{modeNameWindow, leftClick},
			want: "unexpected argument: " + leftClick,
		},
		{
			name: "a selection flag",
			args: []string{modeNameWindow, argAction},
			want: "mode does not accept --action",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(domain.ModeCustom, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) accepted the command; want a refusal", testCase.args)
			}

			if got := err.Error(); !strings.Contains(got, testCase.want) {
				t.Errorf(
					"Parse(%v) error = %q, want it to contain %q",
					testCase.args,
					got,
					testCase.want,
				)
			}
		})
	}
}

// TestRoundTrip_CarriesTheCustomModeName pins the name through both trips:
// rendered first, so that parsing the rendering finds it where a binding
// would have written it.
func TestRoundTrip_CarriesTheCustomModeName(t *testing.T) {
	t.Parallel()

	built := modecmd.Activation{Mode: domain.ModeCustom, Name: modeNameWindow, Toggle: new(true)}

	rendered := modecmd.Render(built)
	if len(rendered) == 0 || rendered[0] != modeNameWindow {
		t.Fatalf("Render(%+v) = %v, want the name first", built, rendered)
	}

	parsed, err := modecmd.Parse(domain.ModeCustom, rendered)
	if err != nil {
		t.Fatalf("Parse(Render(%+v)) error = %v", built, err)
	}

	if !reflect.DeepEqual(built, parsed) {
		t.Errorf("round trip changed the activation:\n want %+v\n  got %+v", built, parsed)
	}
}

// TestValidate_RefusesANameOnABuiltInMode pins the other side of the rule: a
// name on a built-in activation is dropped by the rendering, so accepting it
// would let a caller build an activation that says one thing and sends another.
func TestValidate_RefusesANameOnABuiltInMode(t *testing.T) {
	t.Parallel()

	err := modecmd.Validate(modecmd.Activation{Mode: domain.ModeHints, Name: modeNameWindow})
	if err == nil {
		t.Fatal("Validate() accepted a name on hints; want a refusal")
	}

	if got, want := err.Error(), "hints does not take a mode name"; !strings.Contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

// TestLookupMode_RecognizesTheCustomCommandWord pins the word a binding step
// is recognized by. The configuration decides whether "mode window" is a mode
// command with this lookup, so the word has to be the one the daemon
// dispatches on.
func TestLookupMode_RecognizesTheCustomCommandWord(t *testing.T) {
	t.Parallel()

	mode, isMode := modecmd.LookupMode(strings.Fields(stepModeWindow)[0])
	if !isMode || mode != domain.ModeCustom {
		t.Fatalf("LookupMode(%q) = (%v, %v), want (ModeCustom, true)", stepModeWindow, mode, isMode)
	}

	if _, isMode := modecmd.LookupMode(modeNameWindow); isMode {
		t.Errorf("LookupMode(%q) recognized a declared name as a command word", modeNameWindow)
	}
}
