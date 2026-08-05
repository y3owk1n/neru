package flagref_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/flagref"
)

// page is a document shaped like the one this writes into: prose, a region to
// fill, and prose after it.
const page = "# CLI Reference\n\nBefore.\n\n" +
	flagref.BeginMarker + "\n\nstale\n\n" + flagref.EndMarker + "\n\nAfter.\n"

// TestRegion_ReadsBackWhatWasWritten pins the two halves against each other: a
// reader asking what a page says has to be reading the same span the writer
// wrote, or a guardrail built on the pair would report the wrong page as wrong.
//
// That every declared flag has a row is asserted against the published page in
// internal/architecture, where a missing row is a fact about the repository
// rather than about this function.
func TestRegion_ReadsBackWhatWasWritten(t *testing.T) {
	t.Parallel()

	updated, err := flagref.Rewrite(page)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}

	region, err := flagref.Region(updated)
	if err != nil {
		t.Fatalf("Region() error = %v", err)
	}

	if !strings.Contains(region, flagref.Table()) {
		t.Error("Region() did not read back the table Rewrite() wrote")
	}

	if strings.Contains(region, "Before.") || strings.Contains(region, "After.") {
		t.Errorf("Region() reached outside the markers: %q", region)
	}
}

// TestRewrite_ReplacesOnlyTheRegion pins what a generator may touch. The page
// around the markers is somebody's prose, and rewriting it would make running
// the generator a change nobody asked for.
func TestRewrite_ReplacesOnlyTheRegion(t *testing.T) {
	t.Parallel()

	updated, err := flagref.Rewrite(page)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}

	if strings.Contains(updated, "stale") {
		t.Error("the previous contents of the region survived")
	}

	for _, kept := range []string{"# CLI Reference", "Before.", "After."} {
		if !strings.Contains(updated, kept) {
			t.Errorf("Rewrite() dropped %q from outside the region", kept)
		}
	}

	if !strings.Contains(updated, flagref.Table()) {
		t.Error("Rewrite() did not write the table into the region")
	}
}

// TestRewrite_IsIdempotent is what lets the guardrail compare rather than
// regenerate: applying to an up-to-date page has to be a no-op, or every page
// would read as out of date.
func TestRewrite_IsIdempotent(t *testing.T) {
	t.Parallel()

	once, err := flagref.Rewrite(page)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}

	twice, err := flagref.Rewrite(once)
	if err != nil {
		t.Fatalf("Rewrite() error on the second pass = %v", err)
	}

	if twice != once {
		t.Error("Rewrite() changed an already-current page")
	}
}

// TestRewrite_RefusesAPageWithNoRegion covers the mistake a contributor makes
// once: pointing the generator at a page that has no region to write into.
// Silently appending a table, or silently doing nothing, both leave the page
// wrong.
func TestRewrite_RefusesAPageWithNoRegion(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"no markers at all": "# CLI Reference\n\nNothing to fill.\n",
		"never closed":      "# CLI Reference\n\n" + flagref.BeginMarker + "\n",
		"closed first":      flagref.EndMarker + "\n" + flagref.BeginMarker + "\n",
	}

	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := flagref.Rewrite(document)
			if err == nil {
				t.Fatal("Rewrite() accepted a page with no region to write into")
			}

			if !derrors.IsCode(err, derrors.CodeInvalidInput) {
				t.Errorf("Rewrite() error = %v, want invalid input", err)
			}
		})
	}
}
