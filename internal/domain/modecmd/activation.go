package modecmd

import "github.com/y3owk1n/neru/internal/domain"

// Activation is the typed intent to enter a mode: which mode, plus the value
// of every mode flag that was given.
//
// A mode command parses into one, the CLI builds one directly from its typed
// flags, and both reach the mode handler as the same value. Every flag is a
// pointer or a slice so that "not given" stays distinguishable from "given the
// zero value": a mode reads an absent flag as "keep what the configuration
// says" and a present one as an override for this activation.
//
// A presence-only flag is the exception to that, and is nil or true: writing
// "--toggle" is the whole statement, so there is nothing a false could ask for
// that leaving it out does not. Parsing never produces a false one, and
// building one by hand says the same as leaving it nil.
type Activation struct {
	// Mode is the mode being entered. It decides which flags are accepted.
	Mode domain.Mode

	// Name is the declared mode a custom activation enters, the "window" in
	// "mode window". It is empty for every built-in mode, and a custom
	// activation without one is refused: the word alone names nothing.
	Name string

	// Action is the mouse button action to perform on the selection. Commas
	// chain several, which is how a double-click is written.
	Action *string

	// Modifier lists the modifier keys held while the action fires.
	Modifier *string

	// OnExit is the sequence to run once the action is fulfilled.
	//
	// Nil and empty mean different things and the difference is load-bearing.
	// Nil is "not given": the steps a previous activation stored survive, which
	// is what a repeat re-activation needs. A non-nil slice replaces them, so a
	// given-but-empty one clears them and leaves nothing to run.
	OnExit []string

	// Repeat re-activates the mode after the action, for repeated clicks.
	Repeat *bool

	// Toggle exits to idle when the mode is already active.
	Toggle *bool

	// Search opens the search input as the mode activates.
	Search *bool

	// HideOnEmptySearch hides every hint while the search query is empty.
	HideOnEmptySearch *bool

	// SplitWord splits detected text into word-level regions.
	SplitWord *bool

	// CursorFollowSelection is how the real cursor behaves during selection:
	// true follows the selection, false holds it still.
	CursorFollowSelection *bool

	// ZoomToDepth auto-zooms recursive grid to a depth.
	ZoomToDepth *int

	// FilterRoles keeps only the elements with these accessibility roles.
	FilterRoles []string

	// FilterTextContains keeps only the elements whose text contains one of
	// these.
	FilterTextContains []string

	// Strategy chooses how elements are detected.
	Strategy *string

	// CaptureScope chooses the region the capture strategies scan.
	CaptureScope *string

	// LabelDirection chooses how hint labels are enumerated.
	LabelDirection *string
}
