package modecmd_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// The literals these cases repeat, spelled out rather than built from the
// vocabulary so that a case still pins the exact text a user would write.
const (
	leftClick        = "left_click"
	modeNameHints    = "hints"
	stepIdle         = "idle"
	flagAction       = "--action"
	flagOnExit       = "--on-exit"
	flagRole         = "--role"
	flagRepeat       = "--repeat"
	flagStrategy     = "--strategy"
	flagHideOnEmpty  = "--hide-on-empty-search"
	argAction        = "--action=left_click"
	argSearch        = "--search"
	argToggle        = "--toggle"
	argOnExitStep    = "--on-exit=action left_click"
	argModifierCmd   = "--modifier=cmd"
	argBadStrategy   = "--strategy=nonsense"
	argZoomToDepth2  = "--zoom-to-depth=2"
	flagMistyped     = "--serach"
	stepLeftClick    = "action left_click"
	directionReverse = "reverse"

	// msgZoomToDepth is the one message that flag gives, whichever way its
	// value is unusable.
	msgZoomToDepth = "--zoom-to-depth requires a non-negative integer"

	// The messages both this file and the diagnosis cases pin, so that the two
	// readings of the same command are held to the same sentence.
	msgRepeatNeedsAction = "--repeat requires --action"
	msgStrategyValue     = "--strategy requires axtree, vision, or wl-kbptr"
	msgGridRejectsSearch = "grid does not accept --search"
)

// flagCase is one flag together with proof that parsing it took effect.
//
// Every flag in the table needs one: a flag the grammar declares but does not
// act on is a flag that fails silently, which is the failure this module
// exists to remove.
type flagCase struct {
	// mode is a mode that accepts the flag.
	mode domain.Mode

	// args is the flag as a user writes it, value included where it needs one.
	args []string

	// applied reports whether the parsed activation shows the flag took effect.
	applied func(modecmd.Activation) bool

	// build sets the same flag on an activation directly, the way a caller
	// holding typed flags does instead of writing a command out.
	build func(*modecmd.Activation)
}

// flagCases covers the whole vocabulary. TestFlags_EveryFlagHasACase is what
// keeps it that way.
func flagCases() map[modecmd.Flag]flagCase {
	return map[modecmd.Flag]flagCase{
		modecmd.FlagAction: {
			mode:    domain.ModeHints,
			args:    []string{argAction},
			applied: func(a modecmd.Activation) bool { return a.Action != nil },
			build:   func(a *modecmd.Activation) { a.Action = new(leftClick) },
		},
		// A modifier is held during the action, so it needs one to hold it for.
		modecmd.FlagModifier: {
			mode:    domain.ModeHints,
			args:    []string{argAction, argModifierCmd},
			applied: func(a modecmd.Activation) bool { return a.Modifier != nil },
			build: func(a *modecmd.Activation) {
				a.Action = new(leftClick)
				a.Modifier = new("cmd")
			},
		},
		// --on-exit runs once the action is fulfilled, so it needs one too.
		modecmd.FlagOnExit: {
			mode:    domain.ModeHints,
			args:    []string{argAction, argOnExitStep},
			applied: func(a modecmd.Activation) bool { return len(a.OnExit) == 1 },
			build: func(a *modecmd.Activation) {
				a.Action = new(leftClick)
				a.OnExit = []string{stepLeftClick}
			},
		},
		modecmd.FlagRepeat: {
			mode:    domain.ModeHints,
			args:    []string{argAction, flagRepeat},
			applied: func(a modecmd.Activation) bool { return a.Repeat != nil && *a.Repeat },
			build: func(a *modecmd.Activation) {
				a.Action = new(leftClick)
				a.Repeat = new(true)
			},
		},
		modecmd.FlagToggle: {
			mode:    domain.ModeScroll,
			args:    []string{argToggle},
			applied: func(a modecmd.Activation) bool { return a.Toggle != nil && *a.Toggle },
			build:   func(a *modecmd.Activation) { a.Toggle = new(true) },
		},
		modecmd.FlagSearch: {
			mode:    domain.ModeHints,
			args:    []string{argSearch},
			applied: func(a modecmd.Activation) bool { return a.Search != nil && *a.Search },
			build:   func(a *modecmd.Activation) { a.Search = new(true) },
		},
		modecmd.FlagHideOnEmptySearch: {
			mode: domain.ModeHints,
			args: []string{argSearch, flagHideOnEmpty},
			applied: func(a modecmd.Activation) bool {
				return a.HideOnEmptySearch != nil && *a.HideOnEmptySearch
			},
			build: func(a *modecmd.Activation) {
				a.Search = new(true)
				a.HideOnEmptySearch = new(true)
			},
		},
		modecmd.FlagRole: {
			mode:    domain.ModeHints,
			args:    []string{"--role=AXButton"},
			applied: func(a modecmd.Activation) bool { return len(a.FilterRoles) == 1 },
			build:   func(a *modecmd.Activation) { a.FilterRoles = []string{"AXButton"} },
		},
		modecmd.FlagText: {
			mode:    domain.ModeHints,
			args:    []string{"--text=OK"},
			applied: func(a modecmd.Activation) bool { return len(a.FilterTextContains) == 1 },
			build:   func(a *modecmd.Activation) { a.FilterTextContains = []string{"OK"} },
		},
		modecmd.FlagStrategy: {
			mode:    domain.ModeHints,
			args:    []string{"--strategy=vision"},
			applied: func(a modecmd.Activation) bool { return a.Strategy != nil },
			build:   func(a *modecmd.Activation) { a.Strategy = new("vision") },
		},
		modecmd.FlagLabelDirection: {
			mode:    domain.ModeHints,
			args:    []string{"--label-direction=" + directionReverse},
			applied: func(a modecmd.Activation) bool { return a.LabelDirection != nil },
			build:   func(a *modecmd.Activation) { a.LabelDirection = new(directionReverse) },
		},
		modecmd.FlagSplitWord: {
			mode:    domain.ModeHints,
			args:    []string{"--split-word"},
			applied: func(a modecmd.Activation) bool { return a.SplitWord != nil && *a.SplitWord },
			build:   func(a *modecmd.Activation) { a.SplitWord = new(true) },
		},
		modecmd.FlagZoomToDepth: {
			mode:    domain.ModeRecursiveGrid,
			args:    []string{"--zoom-to-depth=3"},
			applied: func(a modecmd.Activation) bool { return a.ZoomToDepth != nil },
			build:   func(a *modecmd.Activation) { a.ZoomToDepth = new(3) },
		},
		modecmd.FlagCursorSelectionMode: {
			mode:    domain.ModeGrid,
			args:    []string{"--cursor-selection-mode=hold"},
			applied: func(a modecmd.Activation) bool { return a.CursorFollowSelection != nil },
			build:   func(a *modecmd.Activation) { a.CursorFollowSelection = new(false) },
		},
	}
}

// TestFlags_EveryFlagHasACase stops the coverage below from silently shrinking
// as flags are added.
func TestFlags_EveryFlagHasACase(t *testing.T) {
	t.Parallel()

	covered := flagCases()

	for _, descriptor := range modecmd.All() {
		if _, exists := covered[descriptor.Name()]; !exists {
			t.Errorf("--%s has no case; add one so the flag stays pinned", descriptor.Name())
		}
	}

	if len(covered) != len(modecmd.All()) {
		t.Errorf("cases = %d, vocabulary = %d; a case names a flag that no longer exists",
			len(covered), len(modecmd.All()))
	}
}

// TestParse_ActsOnEveryFlag pins that each flag in the vocabulary reaches the
// activation. A flag the parser declares but drops stops working with no error
// anywhere.
func TestParse_ActsOnEveryFlag(t *testing.T) {
	t.Parallel()

	for name, testCase := range flagCases() {
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()

			activation, err := modecmd.Parse(testCase.mode, testCase.args)
			if err != nil {
				t.Fatalf("Parse(%v) error = %v", testCase.args, err)
			}

			if !testCase.applied(activation) {
				t.Errorf("%v parsed without taking effect; --%s is not acted on",
					testCase.args, name)
			}
		})
	}
}

// TestParse_ShortFormsReachTheSameFlag pins the short spellings, since a
// binding is written by hand and may use either form.
func TestParse_ShortFormsReachTheSameFlag(t *testing.T) {
	t.Parallel()

	covered := flagCases()

	for _, descriptor := range modecmd.All() {
		if descriptor.Short() == "" {
			continue
		}

		t.Run(string(descriptor.Name()), func(t *testing.T) {
			t.Parallel()

			testCase := covered[descriptor.Name()]

			short := slices.Clone(testCase.args)
			// Rewrite only this flag's own argument into its short form; any
			// others the case needed stay as they are.
			for index, arg := range short {
				if descriptor.Match(arg) {
					short[index] = "-" + descriptor.Short() + valueOf(arg)
				}
			}

			activation, err := modecmd.Parse(testCase.mode, short)
			if err != nil {
				t.Fatalf("Parse(%v) error = %v", short, err)
			}

			if !testCase.applied(activation) {
				t.Errorf("%v did not take effect; -%s does not reach --%s",
					short, descriptor.Short(), descriptor.Name())
			}
		})
	}
}

// valueOf returns the "=value" part of an argument, or an empty string.
func valueOf(arg string) string {
	_, value, found := strings.Cut(arg, "=")
	if !found {
		return ""
	}

	return "=" + value
}

// TestParse_ValueFlagsRefuseAMissingValue pins the other half of what the
// vocabulary declares: a flag that takes a value says so when it arrives
// without one, rather than swallowing whatever follows.
func TestParse_ValueFlagsRefuseAMissingValue(t *testing.T) {
	t.Parallel()

	covered := flagCases()

	for _, descriptor := range modecmd.All() {
		if !descriptor.TakesValue() {
			continue
		}

		t.Run(string(descriptor.Name()), func(t *testing.T) {
			t.Parallel()

			mode := covered[descriptor.Name()].mode

			_, err := modecmd.Parse(mode, []string{descriptor.Name().Long()})
			if err == nil {
				t.Errorf("--%s with no value was accepted; it is declared as taking one",
					descriptor.Name())
			}
		})
	}
}

// TestParse_PresenceFlagsRefuseAValue pins that a value written onto a flag
// that carries none is reported.
//
// Reading "--toggle=false" as a request to toggle is the shape of failure this
// module exists to remove: the flag was read, the value was not, and nothing
// said so.
func TestParse_PresenceFlagsRefuseAValue(t *testing.T) {
	t.Parallel()

	covered := flagCases()

	for _, descriptor := range modecmd.All() {
		if descriptor.TakesValue() {
			continue
		}

		t.Run(string(descriptor.Name()), func(t *testing.T) {
			t.Parallel()

			mode := covered[descriptor.Name()].mode
			arg := descriptor.Name().Assign("false")

			_, err := modecmd.Parse(mode, []string{arg})
			if err == nil {
				t.Fatalf("%s was accepted; it carries no value", arg)
			}

			want := descriptor.Name().Long() + " takes no value"
			if got := message(err); got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestParse_ListFlagsRefuseAnEmptyValue pins that a filter given nothing to
// filter by is reported. Accepting it would leave a user watching every element
// come back from a command that named a filter.
func TestParse_ListFlagsRefuseAnEmptyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want string
	}{
		{
			[]string{"--role="},
			"--role requires a value (use comma-separated: --role=button,link)",
		},
		{[]string{"--text="}, "--text requires a value (use comma-separated: --text=foo,bar)"},
	}

	for _, testCase := range tests {
		t.Run(testCase.args[0], func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(domain.ModeHints, testCase.args)
			if err == nil {
				t.Fatalf("%v was accepted; want a refusal", testCase.args)
			}

			if got := message(err); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestParse_AcceptsBothArgumentShapes pins the normalization every other case
// depends on: a caller modeled on the CLI's own traffic repeats the mode name
// as the first argument, and a binding does not.
func TestParse_AcceptsBothArgumentShapes(t *testing.T) {
	t.Parallel()

	withName, err := modecmd.Parse(domain.ModeHints, []string{modeNameHints, flagAction, leftClick})
	if err != nil {
		t.Fatalf("leading mode name refused: %v", err)
	}

	withoutName, err := modecmd.Parse(domain.ModeHints, []string{flagAction, leftClick})
	if err != nil {
		t.Fatalf("bare flags refused: %v", err)
	}

	if !reflect.DeepEqual(withName, withoutName) {
		t.Errorf("shapes disagree: %+v vs %+v", withName, withoutName)
	}
}

// TestParse_ReadsBothValueSpellings pins that a value may be attached with an
// equals sign or written as the following argument.
func TestParse_ReadsBothValueSpellings(t *testing.T) {
	t.Parallel()

	split, err := modecmd.Parse(domain.ModeHints, []string{"--label-direction", directionReverse})
	if err != nil {
		t.Fatalf("space-separated value refused: %v", err)
	}

	equals, err := modecmd.Parse(
		domain.ModeHints,
		[]string{"--label-direction=" + directionReverse},
	)
	if err != nil {
		t.Fatalf("attached value refused: %v", err)
	}

	if !reflect.DeepEqual(split, equals) {
		t.Errorf("spellings disagree: %+v vs %+v", split, equals)
	}
}

// TestParse_TakesAPositionalAction pins that an argument matching no flag
// becomes the action, which is how `hints left_click` is written.
func TestParse_TakesAPositionalAction(t *testing.T) {
	t.Parallel()

	activation, err := modecmd.Parse(domain.ModeHints, []string{leftClick})
	if err != nil {
		t.Fatalf("positional action refused: %v", err)
	}

	if activation.Action == nil || *activation.Action != leftClick {
		t.Errorf("Action = %v, want %q", activation.Action, leftClick)
	}
}

// TestParse_AccumulatesListFlags pins that the list flags collect across
// repeats rather than overwriting.
func TestParse_AccumulatesListFlags(t *testing.T) {
	t.Parallel()

	activation, err := modecmd.Parse(
		domain.ModeHints,
		[]string{flagRole, "button", flagRole, "link", "--text", "OK"},
	)
	if err != nil {
		t.Fatalf("list flags refused: %v", err)
	}

	if len(activation.FilterRoles) != 2 {
		t.Errorf("FilterRoles = %v, want two entries", activation.FilterRoles)
	}

	if len(activation.FilterTextContains) != 1 {
		t.Errorf("FilterTextContains = %v, want one entry", activation.FilterTextContains)
	}
}

// TestParse_OnExitAccumulatesSteps pins that --on-exit is repeatable, so a mode
// can finish with a sequence rather than a single step, in the order written.
func TestParse_OnExitAccumulatesSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "space separated",
			args: []string{argAction, flagOnExit, stepLeftClick, flagOnExit, stepIdle},
			want: []string{stepLeftClick, stepIdle},
		},
		{
			name: "equals form",
			args: []string{argAction, argOnExitStep, "--on-exit=" + stepIdle},
			want: []string{stepLeftClick, stepIdle},
		},
		{
			name: "mixed forms keep order",
			args: []string{argAction, "--on-exit=first", flagOnExit, "second"},
			want: []string{"first", "second"},
		},
		{
			// Absent is not the same as empty: it leaves whatever a previous
			// activation stored alone.
			name: "omitted stays nil",
			args: []string{argAction},
			want: nil,
		},
		{
			// Given but empty clears the stored steps without adding any.
			name: "given but empty clears",
			args: []string{argAction, "--on-exit="},
			want: []string{},
		},
		{
			// A step's padding is not part of the step, wherever the step was
			// written.
			name: "padding is trimmed",
			args: []string{argAction, "--on-exit=  " + stepLeftClick + "  "},
			want: []string{stepLeftClick},
		},
		{
			// A step that is nothing but padding is no step, and says what an
			// empty one says: clear what was stored and run nothing.
			name: "padding only clears",
			args: []string{argAction, "--on-exit=   "},
			want: []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			activation, err := modecmd.Parse(domain.ModeHints, testCase.args)
			if err != nil {
				t.Fatalf("Parse(%v) error = %v", testCase.args, err)
			}

			if !slices.Equal(activation.OnExit, testCase.want) {
				t.Fatalf("OnExit = %#v, want %#v", activation.OnExit, testCase.want)
			}

			if (activation.OnExit == nil) != (testCase.want == nil) {
				t.Fatalf("OnExit nil-ness = %v, want %v",
					activation.OnExit == nil, testCase.want == nil)
			}
		})
	}
}

// TestParse_RefusesFlagsTheModeDoesNotAccept pins the failure the whole module
// exists for: a flag a mode has no use for was accepted, stored and dropped,
// so the mode activated and the flag did nothing.
func TestParse_RefusesFlagsTheModeDoesNotAccept(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
		want string
	}{
		{
			name: "search on grid",
			mode: domain.ModeGrid,
			args: []string{argSearch},
			want: msgGridRejectsSearch,
		},
		{
			name: "action on scroll",
			mode: domain.ModeScroll,
			args: []string{argAction},
			want: "scroll does not accept --action",
		},
		{
			name: "action on monitor_select",
			mode: domain.ModeMonitorSelect,
			args: []string{argAction},
			want: "monitor_select does not accept --action",
		},
		{
			name: "toggle on idle",
			mode: domain.ModeIdle,
			args: []string{argToggle},
			want: "idle does not accept --toggle",
		},
		{
			name: "zoom-to-depth on hints",
			mode: domain.ModeHints,
			args: []string{argZoomToDepth2},
			want: "hints does not accept --zoom-to-depth",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(testCase.mode, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) was accepted; want a refusal", testCase.args)
			}

			if got := message(err); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestParse_RefusesUnknownArguments pins that a mistyped flag is reported
// rather than mistaken for the positional action, and that a stray argument
// after the action is refused rather than dropped.
func TestParse_RefusesUnknownArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
		want string
	}{
		{
			name: "mistyped flag",
			mode: domain.ModeHints,
			args: []string{flagMistyped},
			want: "unknown flag: --serach",
		},
		{
			name: "mistyped short flag",
			mode: domain.ModeHints,
			args: []string{"-z"},
			want: "unknown flag: -z",
		},
		{
			name: "stray argument after the action",
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
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(testCase.mode, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) was accepted; want a refusal", testCase.args)
			}

			if got := message(err); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestParse_RefusalMessages pins what a user is told when a flag arrives
// without a usable value. The wording is the whole feedback they get, and
// several flags name their accepted vocabulary in it.
func TestParse_RefusalMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode domain.Mode
		args []string
		want string
	}{
		{domain.ModeHints, []string{flagAction}, "--action requires a value"},
		{
			domain.ModeHints,
			[]string{argAction, "--modifier"},
			"--modifier requires a value",
		},
		{domain.ModeHints, []string{argAction, flagOnExit}, "--on-exit requires a value"},
		{
			domain.ModeRecursiveGrid,
			[]string{"--zoom-to-depth"},
			msgZoomToDepth,
		},
		{
			domain.ModeRecursiveGrid,
			[]string{"--zoom-to-depth=-1"},
			msgZoomToDepth,
		},
		{
			domain.ModeRecursiveGrid,
			[]string{"--zoom-to-depth=deep"},
			msgZoomToDepth,
		},
		{domain.ModeHints, []string{flagStrategy}, msgStrategyValue},
		{
			domain.ModeHints,
			[]string{argBadStrategy},
			msgStrategyValue,
		},
		{
			domain.ModeHints,
			[]string{"--label-direction"},
			"--label-direction requires normal or reverse",
		},
		{
			domain.ModeHints,
			[]string{"--label-direction=sideways"},
			"--label-direction requires normal or reverse",
		},
		{
			domain.ModeGrid,
			[]string{"--cursor-selection-mode"},
			"--cursor-selection-mode requires follow or hold",
		},
		{
			domain.ModeGrid,
			[]string{"--cursor-selection-mode=sometimes"},
			"--cursor-selection-mode requires follow or hold",
		},
		{
			domain.ModeHints,
			[]string{flagRole},
			"--role requires a value (use comma-separated: --role=button,link)",
		},
		{
			domain.ModeHints,
			[]string{"--text"},
			"--text requires a value (use comma-separated: --text=foo,bar)",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(testCase.mode, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) was accepted; want a refusal", testCase.args)
			}

			if got := message(err); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestValidate_EnforcesFlagDependencies pins the rules that make one flag
// meaningless without another, and the one message each of them gives.
//
// --on-exit is the rule that used to exist on the command line only: over the
// wire and from a binding the steps were stored and never run.
func TestValidate_EnforcesFlagDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode domain.Mode
		args []string
		want string
	}{
		{
			name: "repeat without action",
			mode: domain.ModeHints,
			args: []string{flagRepeat},
			want: msgRepeatNeedsAction,
		},
		{
			name: "on-exit without action",
			mode: domain.ModeHints,
			args: []string{"--on-exit=" + stepLeftClick},
			want: "--on-exit requires --action (it runs only when the action is fulfilled)",
		},
		{
			name: "modifier without action",
			mode: domain.ModeHints,
			args: []string{argModifierCmd},
			want: "--modifier requires --action",
		},
		{
			name: "hide-on-empty-search without search",
			mode: domain.ModeHints,
			args: []string{flagHideOnEmpty},
			want: "--hide-on-empty-search requires --search",
		},
		{
			name: "empty modifier list",
			mode: domain.ModeHints,
			args: []string{argAction, "--modifier=,"},
			want: "modifier values cannot be empty",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(testCase.mode, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) was accepted; the dependency is not enforced", testCase.args)
			}

			if got := message(err); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestValidate_PinsTheActionVocabulary pins which actions a mode may be given.
// A mode action is fulfilled by a selection, so only the mouse buttons make
// sense; everything else has its own command.
func TestValidate_PinsTheActionVocabulary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown action",
			args: []string{"--action=nonsense"},
			want: "invalid action: nonsense. ",
		},
		{
			name: "empty entry in a chain",
			args: []string{"--action=left_click,"},
			want: "invalid --action at position 1: empty action in comma-separated list",
		},
		{
			name: "scroll sub-action",
			args: []string{"--action=scroll_up"},
			want: `scroll sub-action "scroll_up" cannot be used as a mode action; only mouse button actions can`,
		},
		{
			name: "action that is not a mouse button",
			args: []string{"--action=move_mouse"},
			want: `"move_mouse" cannot be used as a mode action; only mouse button actions can`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(domain.ModeHints, testCase.args)
			if err == nil {
				t.Fatalf("Parse(%v) was accepted; want a refusal", testCase.args)
			}

			if got := message(err); !strings.HasPrefix(got, testCase.want) {
				t.Errorf("message = %q, want it to start with %q", got, testCase.want)
			}
		})
	}
}

// TestValidate_AcceptsAChainedAction pins that commas chain several clicks,
// which is how a double-click is written.
func TestValidate_AcceptsAChainedAction(t *testing.T) {
	t.Parallel()

	_, err := modecmd.Parse(domain.ModeHints, []string{"--action=left_click,left_click"})
	if err != nil {
		t.Errorf("chained action refused: %v", err)
	}
}

// TestValidate_RefusesAFlagTheModeDoesNotAccept pins that the rule holds for an
// activation built field by field, not only for one parsed from arguments. The
// CLI builds one from its own typed flags and never parses a string.
func TestValidate_RefusesAFlagTheModeDoesNotAccept(t *testing.T) {
	t.Parallel()

	search := true

	err := modecmd.Validate(modecmd.Activation{Mode: domain.ModeGrid, Search: &search})
	if err == nil {
		t.Fatal("Validate() accepted --search on grid; want a refusal")
	}

	if want := msgGridRejectsSearch; message(err) != want {
		t.Errorf("message = %q, want %q", message(err), want)
	}
}

// TestValidate_AcceptsAnEmptyActivation pins that a mode command with no flags
// at all is valid, which is what every default binding is.
func TestValidate_AcceptsAnEmptyActivation(t *testing.T) {
	t.Parallel()

	for _, mode := range []domain.Mode{
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeRecursiveGrid,
		domain.ModeScroll,
		domain.ModeMonitorSelect,
		domain.ModeIdle,
	} {
		t.Run(domain.ModeString(mode), func(t *testing.T) {
			t.Parallel()

			_, err := modecmd.Parse(mode, nil)
			if err != nil {
				t.Errorf("bare mode command refused: %v", err)
			}
		})
	}
}

// message returns the text a user is shown, without the error code the domain
// error carries for callers.
func message(err error) string {
	_, after, found := strings.Cut(err.Error(), "] ")
	if !found {
		return err.Error()
	}

	return after
}
