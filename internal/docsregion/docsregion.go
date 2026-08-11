// Package docsregion is the one implementation of a generated region inside a
// hand-written documentation page.
//
// Two references are published this way — the mode-flag table in docs/CLI.md
// and the platform-support table in docs/CROSS_PLATFORM.md — and both need the
// same three things: locate the region, replace what is between the markers,
// and hand back what a page currently holds so a guardrail can report it. A
// shared derivation has one implementation
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md); the second
// copy had already drifted from the first in how it reported markers written
// out of order.
//
// What each renderer keeps is the part that is actually its own: its markers,
// and the table it renders.
package docsregion

import (
	"os"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Markers delimit one generated region. They name the command that writes the
// region, because the first thing a reader who wants to change a row needs to
// know is that editing it here would be overwritten.
type Markers struct {
	Begin string
	End   string
	// What names the reference, for the message a document with no region
	// gives. It reads as "document has no generated <what> region".
	What string
}

// Rewrite returns the document with its generated region replaced by the table,
// and reports a document that has no region to write into.
//
// Everything outside the markers is left exactly as it was: a reference is one
// table inside a hand-written page, not the page.
func (m Markers) Rewrite(document, table string) (string, error) {
	begin, end, err := m.bounds(document)
	if err != nil {
		return "", err
	}

	return document[:begin] + "\n\n" + table + "\n" + document[end:], nil
}

// Region returns what a document currently holds between the markers.
//
// [Markers.Rewrite] answers "what should this page say?"; this answers "what
// does it say?", which is what a reader reporting a page's contents back — a
// guardrail naming the row it could not find — needs. Both read the markers
// here, so there is one answer to where a region starts.
func (m Markers) Region(document string) (string, error) {
	begin, end, err := m.bounds(document)
	if err != nil {
		return "", err
	}

	return document[begin:end], nil
}

// bounds locates the generated region: the offset just past the opening marker
// and the offset of the closing one.
func (m Markers) bounds(document string) (int, int, error) {
	begin := strings.Index(document, m.Begin)
	if begin < 0 {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"document has no generated %s region; add the %q marker",
			m.What, m.Begin,
		)
	}

	end := strings.Index(document, m.End)
	if end < 0 {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"document has no closing %s marker; add %q",
			m.What, m.End,
		)
	}

	begin += len(m.Begin)

	if end < begin {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"the %s markers are in the wrong order",
			m.What,
		)
	}

	return begin, end, nil
}

// Generate rewrites one page in place and reports what it did, which is the
// whole body of every generator command that writes a region.
//
// The page's own mode is kept: a generated region does not make the file the
// generator's to re-permission.
func Generate(path string, rewrite func(string) (string, error)) (string, error) {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is a build argument
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeInvalidInput, "read "+path)
	}

	updated, err := rewrite(string(contents))
	if err != nil {
		return "", err
	}

	if updated == string(contents) {
		return path + " is already up to date", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeInvalidInput, "read the mode of "+path)
	}

	err = os.WriteFile(path, []byte(updated), info.Mode().Perm())
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeInvalidInput, "write "+path)
	}

	return "written to " + path, nil
}
