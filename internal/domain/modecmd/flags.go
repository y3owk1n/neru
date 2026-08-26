package modecmd

import (
	"slices"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// Flag is a mode flag's long name without the dashes, the form cobra registers
// and the form a user writes in a binding.
type Flag string

// The flags a mode command accepts.
const (
	// FlagAction is the mouse action to perform on the selection. It may also
	// be given positionally, as "hints left_click".
	FlagAction Flag = "action"

	// FlagModifier lists modifier keys to hold while the action fires.
	FlagModifier Flag = "modifier"

	// FlagOnExit is a step to run once the action is fulfilled. Repeatable.
	FlagOnExit Flag = "on-exit"

	// FlagRepeat re-activates the mode after the action, for repeated clicks.
	FlagRepeat Flag = "repeat"

	// FlagToggle exits to idle when the mode is already active.
	FlagToggle Flag = "toggle"

	// FlagSearch opens the search input as the mode activates.
	FlagSearch Flag = "search"

	// FlagHideOnEmptySearch hides every hint while the search query is empty.
	FlagHideOnEmptySearch Flag = "hide-on-empty-search"

	// FlagRole filters elements by accessibility role.
	FlagRole Flag = "role"

	// FlagText filters elements by their text content.
	FlagText Flag = "text"

	// FlagStrategy chooses how elements are detected.
	FlagStrategy Flag = "strategy"

	// FlagLabelDirection chooses how hint labels are enumerated.
	FlagLabelDirection Flag = "label-direction"

	// FlagSplitWord splits detected text into word-level regions.
	FlagSplitWord Flag = "split-word"

	// FlagZoomToDepth auto-zooms recursive grid to a depth.
	FlagZoomToDepth Flag = "zoom-to-depth"

	// FlagCursorSelectionMode chooses how the real cursor behaves during
	// selection.
	FlagCursorSelectionMode Flag = "cursor-selection-mode"
)

// The one message each flag gives when its value is missing or unusable.
//
// Missing and unusable share a message on purpose: they are the same mistake
// seen from two sides, and the answer to both is the same sentence. Each is
// written to be true wherever it is read — on a command line, in an IPC
// response, and as advice about a binding — so none of them names a way of
// invoking Neru that only one of those three has.
const (
	msgActionValue              = "--action requires a value"
	msgModifierValue            = "--modifier requires a value"
	msgOnExitValue              = "--on-exit requires a value"
	msgRoleValue                = "--role requires a value (use comma-separated: --role=button,link)"
	msgTextValue                = "--text requires a value (use comma-separated: --text=foo,bar)"
	msgStrategyValue            = "--strategy requires axtree, vision, or wl-kbptr"
	msgLabelDirectionValue      = "--label-direction requires normal or reverse"
	msgZoomToDepthValue         = "--zoom-to-depth requires a non-negative integer"
	msgCursorSelectionModeValue = "--cursor-selection-mode requires follow or hold"

	// msgTakesNoValue completes the message a presence-only flag gives when it
	// is written with one. Absent and false say the same thing about such a
	// flag, so there is no value for it to carry.
	msgTakesNoValue = " takes no value"
)

// What each flag is for, in one sentence.
//
// A flag is explained here rather than where it is registered, for the same
// reason its rules are: a mode command is written in three places, and the
// sentence that says what a flag does is true in all of them. It is worded to
// read as advice about a flag rather than about a command line, so a reader
// that is not one — the daemon's answer, a generated reference — can say it
// too.
const (
	usageToggle              = "Toggle mode on/off (exit to idle if already active)"
	usageRepeat              = "Re-activate mode after performing the action (requires --action)"
	usageModifier            = "Comma-separated modifier keys to hold during action (cmd, super, meta, shift, alt, option, ctrl) (requires --action)"
	usageOnExit              = "Step to run after the action is fulfilled and the mode exits (same syntax as hotkeys, e.g. 'action left_click' or 'exec notify-send done'). Repeat the flag to run several steps in order. Requires --action; not run on manual escape/idle"
	usageCursorSelectionMode = "How the real cursor should behave during selection: follow or hold"
	usageSearch              = "Show search input when the mode is activated"
	usageHideOnEmptySearch   = "Hide all hints when search query is empty (requires --search)"
	usageRole                = "Filter by element role (comma-separated: button,link — the hints.clickable_roles vocabulary, see 'neru roles'). Repeat the flag to add more"
	usageText                = "Filter elements by text content (comma-separated, case-insensitive substring match). Repeat the flag to add more"
	usageStrategy            = "Element detection strategy: axtree (the platform accessibility tree), vision (screen recognition: the Vision framework on macOS, tesseract OCR on Linux), or wl-kbptr (contour detection via embedded C)"
	usageLabelDirection      = "Hint label enumeration: normal (default, prefix-avoidance, prefers shorter labels) or reverse (spreads labels across the alphabet)"
	usageSplitWord           = "Split detected text into word-level regions (requires vision strategy)"
	usageZoomToDepth         = "Auto-zoom to the given depth (a non-negative integer) in recursive-grid at the current cursor position"
)

// usageAction names the actions a mode can perform, so the vocabulary a user
// is offered is the one the rules accept rather than a list kept alongside it.
var usageAction = "Mouse button action to perform on the selection (" +
	action.ModeActionNamesString() +
	"). Commas chain multiple actions (e.g. left_click,left_click for double-click). " +
	"Other actions, such as scroll or move_mouse, are actions in their own right and need no mode"

// String returns the bare name.
func (f Flag) String() string { return string(f) }

// Long returns the flag as written on a command line: "--action".
func (f Flag) Long() string { return "--" + string(f) }

// Assign returns the flag with a value attached, the form a rendering uses:
// "--action=left_click".
func (f Flag) Assign(value string) string { return "--" + string(f) + "=" + value }

// Descriptor is everything the grammar knows about one flag: how it is
// written, which modes accept it, how a written value becomes part of an
// activation, and how it is written back out.
//
// Its fields are unexported and it is built only by the two constructors
// below, so a flag cannot be declared without saying how it parses and how it
// renders — the compiler asks for both.
type Descriptor struct {
	name  Flag
	short string
	usage string
	kind  Kind
	// valueMessage is the whole message this flag gives when its value is
	// missing or unusable.
	valueMessage string
	modes        []domain.Mode
	set          func(*Activation, string) error
	render       func(Activation) []string
}

// Kind is the shape a flag is written in, and the whole of what a reader needs
// to know to offer it and read it back.
//
// It is one value rather than a pair of booleans so that a reader answers it
// once, exhaustively. A reader deciding "does it take a value, and if so may it
// be repeated?" for itself has somewhere for a shape it has not heard of to
// fall through to, which is the shape of mistake this package exists to stop.
type Kind int

const (
	// KindPresence is a flag that carries no value: writing it is the whole
	// statement.
	KindPresence Kind = iota

	// KindValue is a flag written with a value, where a second occurrence
	// replaces the first.
	KindValue

	// KindList is a flag written with a value, where a second occurrence adds
	// to the first.
	KindList
)

// valueFlag declares a flag written with a value, either attached with an
// equals sign or as the following argument.
func valueFlag(
	name Flag,
	short string,
	usage string,
	valueMessage string,
	modes []domain.Mode,
	set func(*Activation, string) error,
	render func(Activation) []string,
) Descriptor {
	return Descriptor{
		name:         name,
		short:        short,
		usage:        usage,
		kind:         KindValue,
		valueMessage: valueMessage,
		modes:        modes,
		set:          set,
		render:       render,
	}
}

// listFlag declares a value flag that may be written more than once, each
// occurrence adding to the list the earlier ones started.
//
// It is a value flag in every other respect. What separates the two is what a
// second occurrence means: on a plain value flag the last one wins, and on one
// of these none of them is lost. A reader that offers the flag has to know
// which, or the steps of an --on-exit sequence would silently come down to the
// last one written.
func listFlag(
	name Flag,
	short string,
	usage string,
	valueMessage string,
	modes []domain.Mode,
	set func(*Activation, string) error,
	render func(Activation) []string,
) Descriptor {
	descriptor := valueFlag(name, short, usage, valueMessage, modes, set, render)
	descriptor.kind = KindList

	return descriptor
}

// presenceFlag declares a flag that carries no value: writing it is the whole
// statement.
//
// A value written onto one anyway is refused rather than dropped. Taking
// "--toggle=false" as a request to toggle is the shape of mistake this module
// exists to stop: the flag was read, the value was not, and nothing said so.
func presenceFlag(
	name Flag,
	short string,
	usage string,
	modes []domain.Mode,
	set func(*Activation),
	render func(Activation) []string,
) Descriptor {
	return Descriptor{
		name:         name,
		short:        short,
		usage:        usage,
		valueMessage: name.Long() + msgTakesNoValue,
		modes:        modes,
		set: func(activation *Activation, value string) error {
			if value != "" {
				return invalid(name.Long() + msgTakesNoValue)
			}

			set(activation)

			return nil
		},
		render: render,
	}
}

// Name returns the flag this describes.
func (d Descriptor) Name() Flag { return d.name }

// Short returns the single-letter alias, empty when there is none. Cobra reads
// an empty shorthand the same way.
func (d Descriptor) Short() string { return d.short }

// Usage returns the one sentence saying what this flag does, as a command
// line's help and the flag reference both read it.
func (d Descriptor) Usage() string { return d.usage }

// Kind returns the shape this flag is written in.
func (d Descriptor) Kind() Kind { return d.kind }

// TakesValue separates "--repeat" from "--modifier=cmd".
func (d Descriptor) TakesValue() bool { return d.kind != KindPresence }

// Apply reads one written value into an activation, as this flag's own rule
// says. A presence-only flag is applied with an empty value: writing it is the
// whole statement.
//
// It is exported for the reader that holds a flag's value already typed rather
// than as an argument to be found — the CLI reads cobra's own flags this way.
// Going through the same function a parse does is what keeps the two from
// disagreeing about what a written value means.
func (d Descriptor) Apply(activation *Activation, value string) error {
	return d.set(activation, value)
}

// Render writes just this flag back out, empty when the activation was not
// given it. [Render] is the whole table's version of the same thing.
//
// It is exported for a reader that carries some of the vocabulary but not all
// of it: the hints probe takes the flags that decide which elements are
// collected and refuses the rest, and answering "was this one given?" is what
// it needs to say so.
func (d Descriptor) Render(activation Activation) []string { return d.render(activation) }

// ValueMessage returns the whole message this flag gives when its value is
// missing or unusable. The two are one mistake seen from two sides, so they
// share a sentence.
func (d Descriptor) ValueMessage() string { return d.valueMessage }

// AcceptedBy reports whether the mode accepts this flag. Which modes accept a
// given flag is part of the flag's definition, not of the mode's.
func (d Descriptor) AcceptedBy(mode domain.Mode) bool {
	return slices.Contains(d.modes, mode)
}

// AcceptedModes returns the modes that accept this flag, in the order the
// vocabulary declares them.
//
// [Descriptor.AcceptedBy] answers the question a reader with a mode in hand
// asks. This answers the one a reader without a mode asks — the published flag
// reference lists which modes take a flag, and reading that from the same
// declaration is what stops the document from claiming a mode the binary does
// not offer it on.
func (d Descriptor) AcceptedModes() []domain.Mode {
	return slices.Clone(d.modes)
}

// Match reports whether arg is this flag in any spelling: long, short, or
// either with a value attached.
func (d Descriptor) Match(arg string) bool {
	if matchesForm(arg, d.name.Long()) {
		return true
	}

	return d.short != "" && matchesForm(arg, "-"+d.short)
}

// matchesForm reports whether arg is form, or form with "=value".
func matchesForm(arg, form string) bool {
	return arg == form || strings.HasPrefix(arg, form+"=")
}

// The modes each group of flags belongs to.
var (
	// selectionModes make a selection the user then acts on, so they take the
	// flags that describe what happens to it.
	selectionModes = []domain.Mode{domain.ModeHints, domain.ModeGrid, domain.ModeRecursiveGrid}

	// enterableModes are the modes a user enters. Every mode except idle,
	// which is the departure rather than an arrival.
	enterableModes = []domain.Mode{
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeRecursiveGrid,
		domain.ModeScroll,
		domain.ModeMonitorSelect,
	}

	// hintsOnly are the flags that describe element detection, which only
	// hints does.
	hintsOnly = []domain.Mode{domain.ModeHints}

	// recursiveGridOnly is the one flag about zooming.
	recursiveGridOnly = []domain.Mode{domain.ModeRecursiveGrid}

	// modes is every mode a mode command can name: the ones a user enters, and
	// idle, which is how they leave.
	modes = append(slices.Clone(enterableModes), domain.ModeIdle)
)

// Modes returns every mode a mode command can name, idle included.
//
// It is exported for the reader that has to answer "is every mode accounted
// for?" rather than "what does this one accept?" — the guardrail that pins a
// command to the vocabulary needs the list to walk, and reading it from here is
// what makes a mode added to the grammar and forgotten on the command line a
// build failure.
func Modes() []domain.Mode {
	return slices.Clone(modes)
}

// LookupMode returns the mode a command word names.
//
// It is exported for the reader that has to decide whether something is a mode
// command before it can be parsed: a configuration holds its bindings as text,
// and the word it recognizes has to be the word the daemon dispatches on, or
// the two would disagree about which steps are even mode commands.
func LookupMode(word string) (domain.Mode, bool) {
	for _, mode := range modes {
		if domain.ModeString(mode) == word {
			return mode, true
		}
	}

	return domain.ModeIdle, false
}

// descriptors is the vocabulary, in the order a rendering writes it and the
// documentation lists it.
var descriptors = []Descriptor{
	valueFlag(FlagAction, "a", usageAction, msgActionValue, selectionModes,
		func(activation *Activation, value string) error {
			activation.Action = &value

			return nil
		},
		func(activation Activation) []string {
			return renderValue(FlagAction, activation.Action)
		},
	),
	valueFlag(FlagModifier, "", usageModifier, msgModifierValue, selectionModes,
		func(activation *Activation, value string) error {
			activation.Modifier = &value

			return nil
		},
		func(activation Activation) []string {
			return renderValue(FlagModifier, activation.Modifier)
		},
	),
	listFlag(FlagOnExit, "", usageOnExit, msgOnExitValue, selectionModes, setOnExit, renderOnExit),
	presenceFlag(FlagRepeat, "r", usageRepeat, selectionModes,
		func(activation *Activation) { activation.Repeat = enabled() },
		func(activation Activation) []string {
			return renderPresence(FlagRepeat, activation.Repeat)
		},
	),
	presenceFlag(FlagToggle, "t", usageToggle, enterableModes,
		func(activation *Activation) { activation.Toggle = enabled() },
		func(activation Activation) []string {
			return renderPresence(FlagToggle, activation.Toggle)
		},
	),
	presenceFlag(FlagSearch, "s", usageSearch, hintsOnly,
		func(activation *Activation) { activation.Search = enabled() },
		func(activation Activation) []string {
			return renderPresence(FlagSearch, activation.Search)
		},
	),
	presenceFlag(FlagHideOnEmptySearch, "", usageHideOnEmptySearch, hintsOnly,
		func(activation *Activation) { activation.HideOnEmptySearch = enabled() },
		func(activation Activation) []string {
			return renderPresence(FlagHideOnEmptySearch, activation.HideOnEmptySearch)
		},
	),
	listFlag(FlagRole, "", usageRole, msgRoleValue, hintsOnly,
		func(activation *Activation, value string) error {
			entries, err := splitCSV(value, msgRoleValue)
			if err != nil {
				return err
			}

			activation.FilterRoles = append(activation.FilterRoles, entries...)

			return nil
		},
		func(activation Activation) []string {
			return renderList(FlagRole, activation.FilterRoles)
		},
	),
	listFlag(FlagText, "", usageText, msgTextValue, hintsOnly,
		func(activation *Activation, value string) error {
			entries, err := splitCSV(value, msgTextValue)
			if err != nil {
				return err
			}

			activation.FilterTextContains = append(activation.FilterTextContains, entries...)

			return nil
		},
		func(activation Activation) []string {
			return renderList(FlagText, activation.FilterTextContains)
		},
	),
	valueFlag(FlagStrategy, "", usageStrategy, msgStrategyValue, hintsOnly,
		func(activation *Activation, value string) error {
			strategy, err := ParseStrategy(value)
			if err != nil {
				return err
			}

			activation.Strategy = &strategy

			return nil
		},
		func(activation Activation) []string {
			return renderValue(FlagStrategy, activation.Strategy)
		},
	),
	valueFlag(FlagLabelDirection, "", usageLabelDirection, msgLabelDirectionValue, hintsOnly,
		func(activation *Activation, value string) error {
			if value != domain.LabelDirectionNormal && value != domain.LabelDirectionReverse {
				return invalid(msgLabelDirectionValue)
			}

			activation.LabelDirection = &value

			return nil
		},
		func(activation Activation) []string {
			return renderValue(FlagLabelDirection, activation.LabelDirection)
		},
	),
	presenceFlag(FlagSplitWord, "", usageSplitWord, hintsOnly,
		func(activation *Activation) { activation.SplitWord = enabled() },
		func(activation Activation) []string {
			return renderPresence(FlagSplitWord, activation.SplitWord)
		},
	),
	valueFlag(FlagZoomToDepth, "", usageZoomToDepth, msgZoomToDepthValue, recursiveGridOnly,
		func(activation *Activation, value string) error {
			depth, err := strconv.Atoi(value)
			if err != nil || depth < 0 {
				return invalid(msgZoomToDepthValue)
			}

			activation.ZoomToDepth = &depth

			return nil
		},
		func(activation Activation) []string {
			if activation.ZoomToDepth == nil {
				return nil
			}

			return []string{FlagZoomToDepth.Assign(strconv.Itoa(*activation.ZoomToDepth))}
		},
	),
	valueFlag(FlagCursorSelectionMode, "", usageCursorSelectionMode, msgCursorSelectionModeValue,
		selectionModes,
		func(activation *Activation, value string) error {
			switch value {
			case domain.CursorSelectionModeFollow:
				activation.CursorFollowSelection = enabled()
			case domain.CursorSelectionModeHold:
				held := false
				activation.CursorFollowSelection = &held
			default:
				return invalid(msgCursorSelectionModeValue)
			}

			return nil
		},
		func(activation Activation) []string {
			if activation.CursorFollowSelection == nil {
				return nil
			}

			mode := domain.CursorSelectionModeHold
			if *activation.CursorFollowSelection {
				mode = domain.CursorSelectionModeFollow
			}

			return []string{FlagCursorSelectionMode.Assign(mode)}
		},
	),
}

// All returns every mode flag, in the order a rendering writes them.
func All() []Descriptor {
	return slices.Clone(descriptors)
}

// Lookup returns the descriptor for a flag.
func Lookup(name Flag) (Descriptor, bool) {
	for _, descriptor := range descriptors {
		if descriptor.name == name {
			return descriptor, true
		}
	}

	return Descriptor{}, false
}

// ParseStrategy reads an element-detection strategy, the value --strategy
// accepts.
//
// It is exported because the hints probe takes the same flag without being a
// mode command: a probe reports what hints mode would target and enters
// nothing, so it has an argument list of its own. Reading the vocabulary from
// here is what stops the two from disagreeing about what "axtree" means.
func ParseStrategy(value string) (string, error) {
	if value != domain.StrategyAXTree && value != domain.StrategyVision && value != domain.StrategyWLKBPTR {
		return "", invalid(msgStrategyValue)
	}

	return value, nil
}

// setOnExit appends one step to the sequence that runs once the action is
// fulfilled.
//
// A value that is empty after trimming adds no step but still marks the
// sequence as given, which is how a command says "run nothing on exit" as
// distinct from saying nothing at all. See [Activation.OnExit].
func setOnExit(activation *Activation, value string) error {
	if activation.OnExit == nil {
		activation.OnExit = []string{}
	}

	step := strings.TrimSpace(value)
	if step == "" {
		return nil
	}

	activation.OnExit = append(activation.OnExit, step)

	return nil
}

// renderOnExit writes one argument per step, and the empty flag when the
// sequence was given with no steps in it.
func renderOnExit(activation Activation) []string {
	if activation.OnExit == nil {
		return nil
	}

	if len(activation.OnExit) == 0 {
		return []string{FlagOnExit.Assign("")}
	}

	args := make([]string, 0, len(activation.OnExit))
	for _, step := range activation.OnExit {
		args = append(args, FlagOnExit.Assign(step))
	}

	return args
}

// renderValue writes a flag that was given a value.
func renderValue(name Flag, value *string) []string {
	if value == nil {
		return nil
	}

	return []string{name.Assign(*value)}
}

// renderPresence writes a flag whose presence is the whole statement.
func renderPresence(name Flag, value *bool) []string {
	if value == nil || !*value {
		return nil
	}

	return []string{name.Long()}
}

// renderList writes a repeatable list flag as the one comma-separated
// argument that parses back into the same entries.
func renderList(name Flag, values []string) []string {
	if len(values) == 0 {
		return nil
	}

	return []string{name.Assign(strings.Join(values, ","))}
}

// enabled returns a pointer to true, which is what a presence-only flag sets.
func enabled() *bool {
	on := true

	return &on
}

// splitCSV reads the comma-separated form the list flags accept. An empty
// value filters nothing, so it is refused rather than quietly doing nothing.
func splitCSV(value, missing string) ([]string, error) {
	if value == "" {
		return nil, invalid(missing)
	}

	return strings.Split(value, ","), nil
}

// invalid builds the refusal a grammar rule returns. Every one of them is
// invalid input, which is the response code a script branches on.
func invalid(message string) error {
	return derrors.New(derrors.CodeInvalidInput, message)
}
