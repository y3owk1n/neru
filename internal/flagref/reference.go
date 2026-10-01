package flagref

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// The comments delimiting the generated region of a document.
//
// They name the command that writes the region, because the first thing a
// reader who wants to change a row needs to know is that editing it here would
// be overwritten.
const (
	BeginMarker = "<!-- BEGIN GENERATED MODE FLAGS: edit internal/domain/modecmd, then run `just genflagref` -->"
	EndMarker   = "<!-- END GENERATED MODE FLAGS -->"
)

// markers is the region this reference is published into.
var markers = docsregion.Markers{
	Begin: BeginMarker,
	End:   EndMarker,
	What:  "mode-flag",
}

// The sentences that say how each shape of flag is written.
const (
	valueNone       = "Takes no value."
	valueOne        = "Takes a value."
	valueRepeatable = "Takes a value, and can be repeated."
	valueUnknown    = "Takes an unknown shape of value."
)

// separator joins the modes that accept a flag.
const separator = " · "

// Table renders every mode flag as one entry, in the order the vocabulary
// declares them.
//
// Each entry is a heading a page can link to, one line of facts (shorthand,
// value, modes), and the sentence that says what the flag does, which is the
// same sentence a command line's help prints, so the document and the binary
// cannot describe a flag differently. Entries rather than a table, because the
// descriptions are long enough to make a table unreadable on a narrow screen.
func Table() string {
	var out strings.Builder

	for index, descriptor := range modecmd.All() {
		if index > 0 {
			out.WriteString("\n")
		}

		fmt.Fprintf(&out, "#### `%s`\n\n", descriptor.Name().Long())
		fmt.Fprintf(
			&out,
			"%s%s Modes: %s.\n\n",
			shorthand(descriptor),
			value(descriptor),
			modes(descriptor),
		)
		fmt.Fprintf(&out, "%s\n", sentence(descriptor.Usage()))
	}

	return out.String()
}

// Rewrite returns the document with its generated region rewritten, and
// reports a document that has no region to write into.
//
// Everything outside the markers is left exactly as it was: the reference is
// one table inside a hand-written page, not the page.
func Rewrite(document string) (string, error) {
	return markers.Rewrite(document, Table())
}

// Region returns what a document currently holds between the markers.
//
// [Rewrite] answers "what should this page say?"; this answers "what does it
// say?", which is what a reader reporting a page's contents back — a guardrail
// naming the flag it could not find — needs.
func Region(document string) (string, error) {
	return markers.Region(document)
}

// shorthand names the single-letter alias, or nothing when the flag has none.
func shorthand(descriptor modecmd.Descriptor) string {
	if descriptor.Short() == "" {
		return ""
	}

	return "Shorthand `-" + descriptor.Short() + "`. "
}

// value says whether the flag is written with a value, and whether writing it
// twice adds or replaces. The two are one question to a reader deciding how to
// write the flag, so they share a sentence.
func value(descriptor modecmd.Descriptor) string {
	switch descriptor.Kind() {
	case modecmd.KindPresence:
		return valueNone
	case modecmd.KindValue:
		return valueOne
	case modecmd.KindList:
		return valueRepeatable
	}

	// Unreachable while every shape is answered above. Saying so in the
	// document rather than silently rendering an empty cell is what makes a
	// shape nobody taught this renderer about visible.
	return valueUnknown
}

// modes lists the modes that accept the flag, spelled as a user writes them.
func modes(descriptor modecmd.Descriptor) string {
	accepted := descriptor.AcceptedModes()

	names := make([]string, 0, len(accepted))
	for _, mode := range accepted {
		names = append(names, "`"+domain.ModeString(mode)+"`")
	}

	return strings.Join(names, separator)
}

// sentence ends a description with a full stop, since help text is written
// without one.
func sentence(text string) string {
	return strings.TrimSuffix(text, ".") + "."
}
