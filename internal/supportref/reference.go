package supportref

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// The comments delimiting the generated region of a document.
//
// They name the command that writes the region, because the first thing a
// reader who wants to change a row needs to know is that editing it here would
// be overwritten.
const (
	BeginMarker = "<!-- BEGIN GENERATED PLATFORM SUPPORT: edit the platform_support.go declarations, then run `just gensupportref` -->"
	EndMarker   = "<!-- END GENERATED PLATFORM SUPPORT -->"
)

// markers is the region this reference is published into.
var markers = docsregion.Markers{
	Begin: BeginMarker,
	End:   EndMarker,
	What:  "platform-support",
}

// The marks a platform column is rendered with. They are the same two the
// capability matrix on the same page uses, so a reader is not learning a second
// notation halfway down.
const (
	supported = "✅"
	inert     = "❌"
)

// Declaration is every word Neru declares a platform column for, in the order
// the three vocabularies declare them.
//
// This is the one place they are read together. Each table stays beside the
// declaration that owns its words; a projection over all of them — this page,
// and the load-time warning through internal/config — needs them joined, and
// joining them in one function is what stops two projections from disagreeing
// about which vocabularies are in scope.
func Declaration() parity.Declaration {
	return parity.Join(
		config.PlatformSupport(),
		modecmd.PlatformSupport(),
		action.PlatformSupport(),
	)
}

// Table renders every word whose platform column is narrower than every
// platform, one markdown row per reason.
//
// Words that share a platform column and the sentence that says why share a
// row, so a reason is read once rather than once per word. The sentence is the
// one the load-time warning and `neru doctor` print, so the page and the binary
// cannot explain a gap differently.
func Table() string {
	var out strings.Builder

	out.WriteString("| Words |")

	for _, platform := range parity.AllPlatforms {
		fmt.Fprintf(&out, " %s |", heading(platform))
	}

	out.WriteString(" Why |\n| ---- |")

	for range parity.AllPlatforms {
		out.WriteString(" --- |")
	}

	out.WriteString(" --- |\n")

	for _, group := range Groups() {
		fmt.Fprintf(&out, "| %s |", cell(strings.Join(writtenNames(group), ", ")))

		for _, platform := range parity.AllPlatforms {
			fmt.Fprintf(&out, " %s |", mark(group[0].Platforms.Supports(platform)))
		}

		fmt.Fprintf(&out, " %s |\n", cell(group[0].Note))
	}

	return out.String()
}

// Groups partitions the narrow words by platform column and note, in the order
// each pair first appears in the declaration.
func Groups() []parity.Declaration {
	var groups []parity.Declaration

	index := map[string]int{}

	for _, word := range Declaration().Limited() {
		key := fmt.Sprint(word.Platforms) + "\x00" + word.Note

		position, seen := index[key]
		if !seen {
			position = len(groups)
			index[key] = position

			groups = append(groups, nil)
		}

		groups[position] = append(groups[position], word)
	}

	return groups
}

// writtenNames renders a group's words the way a person writes them, which is
// also how the load-time warning names them, so searching the page for a word
// from a warning finds its row. A word that is not an option says which kind
// it is.
func writtenNames(group parity.Declaration) []string {
	names := make([]string, 0, len(group))

	for _, word := range group {
		name := "`" + word.Written() + "`"
		if word.Kind != parity.KindOption {
			name += " (" + string(word.Kind) + ")"
		}

		names = append(names, name)
	}

	return names
}

// heading names a platform the way the rest of the page does.
func heading(platform parity.Platform) string {
	switch platform {
	case parity.Darwin:
		return "macOS"
	case parity.Linux:
		return "Linux"
	case parity.Windows:
		return "Windows"
	}

	// Unreachable while every platform is answered above. Rendering the raw
	// name rather than an empty cell is what makes a platform nobody taught
	// this renderer about visible.
	return string(platform)
}

// mark renders one cell of the platform column.
func mark(supports bool) string {
	if supports {
		return supported
	}

	return inert
}

// Rewrite returns the document with its generated region rewritten, and
// reports a document that has no region to write into.
func Rewrite(document string) (string, error) {
	return markers.Rewrite(document, Table())
}

// Region returns what a document currently holds between the markers, for a
// reader reporting a page's contents back.
func Region(document string) (string, error) {
	return markers.Region(document)
}

// cell escapes what a markdown table cell cannot hold literally. A pipe in a
// note would otherwise end the cell early and shift every column after it.
func cell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
