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

// The words the value column uses for each shape a flag is written in.
const (
	valueNone       = "none"
	valueOne        = "value"
	valueRepeatable = "value, repeatable"
	valueUnknown    = "unknown"
)

// separator joins the modes that accept a flag. A middle dot rather than a
// comma, because several of the descriptions contain commas of their own.
const separator = " · "

// Table renders every mode flag as one markdown row, in the order the
// vocabulary declares them.
//
// The columns are what the declaration knows: how the flag is written, whether
// it carries a value, which modes accept it, and the one sentence that says
// what it does — the same sentence a command line's help prints, so the
// document and the binary cannot describe a flag differently.
func Table() string {
	var out strings.Builder

	out.WriteString("| Flag | Shorthand | Value | Modes | Description |\n")
	out.WriteString("| ---- | --------- | ----- | ----- | ----------- |\n")

	for _, descriptor := range modecmd.All() {
		fmt.Fprintf(
			&out,
			"| `%s` | %s | %s | %s | %s |\n",
			descriptor.Name().Long(),
			shorthand(descriptor),
			value(descriptor),
			modes(descriptor),
			cell(descriptor.Usage()),
		)
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

// shorthand renders the single-letter alias, or nothing when the flag has none.
func shorthand(descriptor modecmd.Descriptor) string {
	if descriptor.Short() == "" {
		return ""
	}

	return "`-" + descriptor.Short() + "`"
}

// value says whether the flag is written with a value, and whether writing it
// twice adds or replaces. The two are one question to a reader deciding how to
// write the flag, so they share a column.
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

// cell escapes what a markdown table cell cannot hold literally. A pipe in a
// flag's description would otherwise end the cell early and shift every column
// after it.
func cell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
