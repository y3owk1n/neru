package architecture_test

import (
	"os"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/hookref"
)

// TestHooksReference_IsGeneratedFromTheEventCatalog fails while a page's hooks
// tables differ from what the event catalog renders. The catalog is what sets
// a hook's variables and what `neru describe` lists, so a hand edit here would
// describe a hook nobody runs.
func TestHooksReference_IsGeneratedFromTheEventCatalog(t *testing.T) {
	published := 0

	for _, doc := range markdownFiles(t, findRepoRoot(t)) {
		contents, err := os.ReadFile(doc.absPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", doc.relPath, err)
		}

		document := string(contents)
		if !strings.Contains(document, hookref.HooksBeginMarker) {
			continue
		}

		published++

		regenerated, err := hookref.Rewrite(document)
		if err != nil {
			t.Errorf("%s: %v", doc.relPath, err)

			continue
		}

		if regenerated != document {
			t.Errorf(
				"the hooks tables in %s are out of date with the event catalog; "+
					"run `just genhookref` (the regions are generated, do not edit them by hand)",
				doc.relPath,
			)
		}
	}

	if published == 0 {
		t.Errorf("no page carries the hooks reference; add the %q marker to the page "+
			"that documents [hooks]", hookref.HooksBeginMarker)
	}
}
