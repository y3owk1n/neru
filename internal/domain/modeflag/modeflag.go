package modeflag

import "strings"

// Name is a flag's long name, written without the leading dashes.
//
// Cobra registers a flag under this bare form; the wire form adds the dashes.
type Name string

// The flags a mode command accepts. Every one of them is written twice over the
// course of a single command — once by the user, once by the CLI onto the wire —
// and read twice, so both halves name them from here.
const (
	// Action is the mouse action to perform on the selection. It is the one
	// flag that may also be given positionally, as `neru hints left_click`.
	Action Name = "action"

	// Modifier lists modifier keys to hold while the action fires.
	Modifier Name = "modifier"

	// OnExit is a step to run once the action is fulfilled. It is repeatable:
	// each occurrence appends one step.
	OnExit Name = "on-exit"

	// Repeat re-activates the mode after the action, for repeated clicks.
	Repeat Name = "repeat"

	// Toggle exits to idle when the mode is already active.
	Toggle Name = "toggle"

	// Search opens the search input as the mode activates.
	Search Name = "search"

	// HideOnEmptySearch hides every hint while the search query is empty.
	HideOnEmptySearch Name = "hide-on-empty-search"

	// Role filters elements by accessibility role.
	Role Name = "role"

	// Text filters elements by their text content.
	Text Name = "text"

	// Strategy chooses how elements are detected.
	Strategy Name = "strategy"

	// Debug prints the detected elements instead of showing the overlay. The
	// daemon accepts it and does nothing with it: the CLI is what acts on it.
	Debug Name = "debug"

	// LabelDirection chooses how hint labels are enumerated.
	LabelDirection Name = "label-direction"

	// SplitWord splits detected text into word-level regions.
	SplitWord Name = "split-word"

	// ZoomToDepth auto-zooms recursive grid to a depth.
	ZoomToDepth Name = "zoom-to-depth"

	// CursorSelectionMode chooses how the real cursor behaves during selection.
	CursorSelectionMode Name = "cursor-selection-mode"
)

// String returns the bare name, which is the form cobra registers.
func (n Name) String() string {
	return string(n)
}

// Short returns the flag's single-letter alias, or an empty string when it has
// none. Cobra reads an empty shorthand as "no shorthand", which is the same
// thing this means.
func (n Name) Short() string {
	spec, known := Get(n)
	if !known {
		return ""
	}

	return spec.Short
}

// Flag returns the flag as written on a command line: "--action".
func (n Name) Flag() string {
	return "--" + string(n)
}

// Assign returns the flag with its value attached: "--action=left_click".
// This is the form the CLI puts on the wire.
func (n Name) Assign(value string) string {
	return "--" + string(n) + "=" + value
}

// Spec is everything both halves of a mode command need to agree on about one
// flag. What each side then does with the value is its own business: the CLI
// validates it against cobra's types, the daemon turns it into an activation
// option.
type Spec struct {
	// Name is the long name.
	Name Name

	// Short is the single-letter alias, empty when the flag has none. The
	// daemon accepts the short form too, because a hotkey binding is written by
	// hand and may use either.
	Short string

	// TakesValue separates "--repeat" from "--modifier=cmd". A flag that takes
	// a value may also be written "--modifier cmd", so a parser has to know
	// whether to consume the argument that follows.
	TakesValue bool
}

// specs is the vocabulary itself, in the order the flags are documented.
var specs = []Spec{
	{Name: Action, Short: "a", TakesValue: true},
	{Name: Modifier, TakesValue: true},
	{Name: OnExit, TakesValue: true},
	{Name: Repeat, Short: "r"},
	{Name: Toggle, Short: "t"},
	{Name: Search, Short: "s"},
	{Name: HideOnEmptySearch},
	{Name: Role, TakesValue: true},
	{Name: Text, TakesValue: true},
	{Name: Strategy, TakesValue: true},
	{Name: Debug, Short: "d"},
	{Name: LabelDirection, TakesValue: true},
	{Name: SplitWord},
	{Name: ZoomToDepth, TakesValue: true},
	{Name: CursorSelectionMode, TakesValue: true},
}

// All returns every mode flag.
func All() []Spec {
	out := make([]Spec, len(specs))
	copy(out, specs)

	return out
}

// Get returns the spec for a name.
func Get(name Name) (Spec, bool) {
	for _, spec := range specs {
		if spec.Name == name {
			return spec, true
		}
	}

	return Spec{}, false
}

// Match reports whether arg is this flag, in any spelling a caller may have
// written: the long form, the short form, or either with a value attached.
func (s Spec) Match(arg string) bool {
	if matchesForm(arg, s.Name.Flag()) {
		return true
	}

	return s.Short != "" && matchesForm(arg, "-"+s.Short)
}

// matchesForm reports whether arg is exactly form, or form with "=value".
func matchesForm(arg, form string) bool {
	return arg == form || strings.HasPrefix(arg, form+"=")
}
