//nolint:testpackage // exercises the unexported flag parser directly.
package ipcctrl

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// The flags and values these cases repeat. The parser keeps its own copies in
// another file in this package; these exist so the table entries below read as
// data rather than as a wall of quoted strings.
const (
	flagAction         = "--action"
	flagStrategy       = "--strategy"
	flagLabelDirection = "--label-direction"
	flagZoomToDepth    = "--zoom-to-depth"
	modifierCmd        = "cmd"
	strategyAXTree     = "axtree"
	directionReverse   = "reverse"
	flagRepeat         = "--repeat"
	flagHideOnEmpty    = "--hide-on-empty-search"
)

// parse runs the parser the way the mode handlers do.
func parse(t *testing.T, args ...string) (ModeActivationOptions, *ipc.Response) {
	t.Helper()

	handler := &ModesHandler{}

	return handler.extractModeOptions(ipc.Command{Action: "hints", Args: args})
}

// TestExtractModeOptions_AcceptsBothArgumentShapes pins the normalization every
// other case depends on: the CLI passes the mode name as the first argument and
// the hotkey path does not.
//
// extractModeOptions turns a mode command's arguments into the options the mode
// handler acts on. Every flag a user can pass to `neru hints`, `neru grid` and
// friends arrives here, so a change in what it accepts is a change in the CLI.
//
// These cases pin the accepted spelling of each flag, the two argument shapes
// the parser has to handle, and what an unusable value produces.
func TestExtractModeOptions_AcceptsBothArgumentShapes(t *testing.T) {
	withName, resp := parse(t, "hints", flagAction, leftClick)
	if resp != nil {
		t.Fatalf("leading mode name rejected: %v", resp.Message)
	}

	withoutName, resp := parse(t, flagAction, leftClick)
	if resp != nil {
		t.Fatalf("bare flags rejected: %v", resp.Message)
	}

	if withName.Action == nil || withoutName.Action == nil {
		t.Fatal("Action unset for one of the two argument shapes")
	}

	if *withName.Action != *withoutName.Action {
		t.Errorf("shapes disagree: %q vs %q", *withName.Action, *withoutName.Action)
	}
}

func TestExtractModeOptions_ReadsNoArguments(t *testing.T) {
	opts, resp := parse(t)
	if resp != nil {
		t.Fatalf("empty arguments rejected: %v", resp.Message)
	}

	if opts.Action != nil || opts.Repeat != nil || len(opts.FilterRoles) != 0 {
		t.Error("empty arguments produced a non-zero option set")
	}
}

// TestExtractModeOptions_ReadsStringFlags pins the value flags. Each is given in
// both spellings the parser accepts, since a user may write either.
func TestExtractModeOptions_ReadsStringFlags(t *testing.T) {
	tests := []struct {
		name  string
		split []string
		equal []string
		want  string
		get   func(ModeActivationOptions) *string
	}{
		{
			name:  "action",
			split: []string{flagAction, leftClick},
			equal: []string{argActionLeftClick},
			want:  leftClick,
			get:   func(o ModeActivationOptions) *string { return o.Action },
		},
		{
			// --modifier only means something alongside an action, so it is
			// given with one here.
			name:  "modifier",
			split: []string{flagAction, leftClick, flagModifier, modifierCmd},
			equal: []string{argActionLeftClick, argModifierCmd},
			want:  modifierCmd,
			get:   func(o ModeActivationOptions) *string { return o.Modifier },
		},
		{
			name:  "strategy",
			split: []string{flagStrategy, strategyAXTree},
			equal: []string{"--strategy=axtree"},
			want:  strategyAXTree,
			get:   func(o ModeActivationOptions) *string { return o.Strategy },
		},
		{
			name:  "label direction",
			split: []string{flagLabelDirection, directionReverse},
			equal: []string{"--label-direction=reverse"},
			want:  directionReverse,
			get:   func(o ModeActivationOptions) *string { return o.LabelDirection },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for _, args := range [][]string{testCase.split, testCase.equal} {
				opts, resp := parse(t, args...)
				if resp != nil {
					t.Fatalf("%v rejected: %v", args, resp.Message)
				}

				got := testCase.get(opts)
				if got == nil {
					t.Fatalf("%v left the option unset", args)
				}

				if *got != testCase.want {
					t.Errorf("%v gave %q, want %q", args, *got, testCase.want)
				}
			}
		})
	}
}

// TestExtractModeOptions_ReadsBoolFlags pins the flags that are presence-only or
// take an explicit value.
func TestExtractModeOptions_ReadsBoolFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		get  func(ModeActivationOptions) *bool
		want bool
	}{
		{
			// --repeat modifies an action, so it is given with one.
			"repeat",
			[]string{flagAction, leftClick, flagRepeat},
			func(o ModeActivationOptions) *bool { return o.Repeat },
			true,
		},
		{
			"search",
			[]string{argSearch},
			func(o ModeActivationOptions) *bool { return o.Search },
			true,
		},
		{
			toggleStateToggle,
			[]string{"--toggle"},
			func(o ModeActivationOptions) *bool { return o.Toggle },
			true,
		},
		{
			"split word",
			[]string{"--split-word"},
			func(o ModeActivationOptions) *bool { return o.SplitWord },
			true,
		},
		{
			// --hide-on-empty-search qualifies --search, so it is given with it.
			"hide on empty search",
			[]string{argSearch, flagHideOnEmpty},
			func(o ModeActivationOptions) *bool { return o.HideOnEmptySearch },
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			opts, resp := parse(t, testCase.args...)
			if resp != nil {
				t.Fatalf("%v rejected: %v", testCase.args, resp.Message)
			}

			got := testCase.get(opts)
			if got == nil {
				t.Fatalf("%v left the option unset", testCase.args)
			}

			if *got != testCase.want {
				t.Errorf("%v gave %v, want %v", testCase.args, *got, testCase.want)
			}
		})
	}
}

// TestExtractModeOptions_AccumulatesListFlags pins that the list flags collect
// across repeats rather than overwriting.
func TestExtractModeOptions_AccumulatesListFlags(t *testing.T) {
	opts, resp := parse(t, "--role", "button", "--role", "link", "--text", "OK")
	if resp != nil {
		t.Fatalf("list flags rejected: %v", resp.Message)
	}

	if len(opts.FilterRoles) != 2 {
		t.Errorf("FilterRoles = %v, want two entries", opts.FilterRoles)
	}

	if len(opts.FilterTextContains) != 1 {
		t.Errorf("FilterTextContains = %v, want one entry", opts.FilterTextContains)
	}
}

// TestExtractModeOptions_ReadsZoomToDepth pins the one numeric flag.
func TestExtractModeOptions_ReadsZoomToDepth(t *testing.T) {
	opts, resp := parse(t, flagZoomToDepth, "3")
	if resp != nil {
		t.Fatalf("--zoom-to-depth rejected: %v", resp.Message)
	}

	if opts.ZoomToDepth == nil || *opts.ZoomToDepth != 3 {
		t.Errorf("ZoomToDepth = %v, want 3", opts.ZoomToDepth)
	}
}

// TestExtractModeOptions_RejectsBadValues pins that an unusable value produces a
// response rather than a silently wrong option. The message is not pinned, only
// that the command is refused.
func TestExtractModeOptions_RejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown strategy", []string{flagStrategy, "nonsense"}},
		{"unknown strategy, equals form", []string{"--strategy=nonsense"}},
		{"unknown label direction", []string{flagLabelDirection, "sideways"}},
		{"unknown cursor selection mode", []string{"--cursor-selection-mode", "sometimes"}},
		{"non-numeric depth", []string{flagZoomToDepth, "deep"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, resp := parse(t, testCase.args...)
			if resp == nil {
				t.Errorf("%v was accepted; want a refusal", testCase.args)
			}
		})
	}
}

// TestExtractModeOptions_TakesAPositionalAction pins that an argument matching
// no flag becomes the action, which is how `neru hints left_click` is written.
func TestExtractModeOptions_TakesAPositionalAction(t *testing.T) {
	opts, resp := parse(t, leftClick)
	if resp != nil {
		t.Fatalf("positional action rejected: %v", resp.Message)
	}

	if opts.Action == nil || *opts.Action != leftClick {
		t.Errorf("Action = %v, want left_click", opts.Action)
	}
}

// TestExtractModeOptions_RefusesAStrayArgument pins that once an action is set,
// a further unmatched argument is refused rather than silently dropped. Its
// only plausible cause is a typo, and swallowing it would apply a command the
// user did not write.
func TestExtractModeOptions_RefusesAStrayArgument(t *testing.T) {
	_, resp := parse(t, leftClick, "stray")
	if resp == nil {
		t.Fatal("a stray argument after the action was accepted")
	}

	if want := "unexpected argument: stray"; resp.Message != want {
		t.Errorf("message = %q, want %q", resp.Message, want)
	}
}

// TestExtractModeOptions_EnforcesFlagDependencies pins the rules that make one
// flag meaningless without another. These are easy to lose in a rewrite,
// because each is a check sitting far from the flag it constrains.
func TestExtractModeOptions_EnforcesFlagDependencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"modifier without action", []string{flagModifier, modifierCmd}},
		{"repeat without action", []string{flagRepeat}},
		{"hide-on-empty-search without search", []string{flagHideOnEmpty}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, resp := parse(t, testCase.args...)
			if resp == nil {
				t.Errorf("%v was accepted; the dependency is not enforced", testCase.args)
			}
		})
	}
}

// TestExtractModeOptions_PinsAcceptedValues pins the vocabularies, which are
// what a user sees when they mistype a flag value.
func TestExtractModeOptions_PinsAcceptedValues(t *testing.T) {
	for _, value := range []string{strategyAXTree, "vision"} {
		if _, resp := parse(t, flagStrategy, value); resp != nil {
			t.Errorf("--strategy %s rejected: %v", value, resp.Message)
		}
	}

	for _, value := range []string{directionReverse, "normal"} {
		if _, resp := parse(t, flagLabelDirection, value); resp != nil {
			t.Errorf("--label-direction %s rejected: %v", value, resp.Message)
		}
	}
}

// TestExtractModeOptions_RefusalMessages pins what a user is told when a flag is
// given without its value. The wording is the whole feedback they get from the
// CLI, and several flags name their accepted vocabulary in it.
func TestExtractModeOptions_RefusalMessages(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{flagAction}, "--action requires a value"},
		{[]string{flagAction, leftClick, flagModifier}, "--modifier requires a value"},
		{[]string{flagZoomToDepth}, "--zoom-to-depth requires a value"},
		{[]string{flagZoomToDepth, "-1"}, "--zoom-to-depth requires a non-negative integer"},
		{[]string{flagStrategy}, "--strategy requires a value: axtree or vision"},
		{[]string{flagLabelDirection}, "--label-direction requires a value: reverse or normal"},
		{
			[]string{"--role"},
			"--role requires a value (use comma-separated: --role=AXButton,AXLink)",
		},
		{[]string{"--text"}, "--text requires a value (use comma-separated: --text=foo,bar)"},
		{[]string{"--on-exit"}, "--on-exit requires a value"},
		{[]string{flagHideOnEmpty}, "--hide-on-empty-search requires --search"},
		{[]string{flagRepeat}, "--repeat requires an action"},
		{[]string{flagModifier, modifierCmd}, "--modifier requires an action"},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			_, resp := parse(t, testCase.args...)
			if resp == nil {
				t.Fatalf("%v was accepted; want a refusal", testCase.args)
			}

			if resp.Message != testCase.want {
				t.Errorf("message = %q, want %q", resp.Message, testCase.want)
			}
		})
	}
}
