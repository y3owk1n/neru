package supportref_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/parity"
	"github.com/y3owk1n/neru/internal/supportref"
)

func TestTable_RendersOneRowPerNarrowWord(t *testing.T) {
	t.Parallel()

	table := supportref.Table()

	limited := supportref.Declaration().Limited()
	if len(limited) == 0 {
		t.Fatal("nothing is declared as narrower than every platform; the table would be empty")
	}

	for _, word := range limited {
		if !strings.Contains(table, "`"+word.Written()+"`") {
			t.Errorf("the table has no row for %q", word.Written())
		}
	}

	// One row per word, plus the header and the separator.
	rows := strings.Count(strings.TrimSpace(table), "\n") + 1
	if rows != len(limited)+2 {
		t.Errorf("the table has %d lines for %d narrow words, want %d",
			rows, len(limited), len(limited)+2)
	}
}

// TestTable_LeavesOutTheWordsThatWorkEverywhere is the whole reason the table is
// readable. The declarations name several hundred words; a table of all of them
// would bury the dozen a reader is actually looking for.
func TestTable_LeavesOutTheWordsThatWorkEverywhere(t *testing.T) {
	t.Parallel()

	table := supportref.Table()

	for _, word := range supportref.Declaration() {
		if !word.Platforms.Everywhere() {
			continue
		}

		if strings.Contains(table, "`"+word.Written()+"`") {
			t.Errorf("the table has a row for %q, which works everywhere", word.Written())
		}
	}
}

// TestTable_MarksEveryPlatformColumn pins that the column is a column. A table
// that rendered one mark would be the darwin-only boolean ADR 0013 rejected,
// wearing a header row.
func TestTable_MarksEveryPlatformColumn(t *testing.T) {
	t.Parallel()

	table := supportref.Table()

	for _, heading := range []string{"macOS", "Linux", "Windows"} {
		if !strings.Contains(table, heading) {
			t.Errorf("the table has no %s column", heading)
		}
	}

	for _, mark := range []string{"✅", "❌"} {
		if !strings.Contains(table, mark) {
			t.Errorf("the table uses no %s mark, so no column says anything", mark)
		}
	}
}

// TestDeclaration_CoversEveryVocabulary keeps the page a projection of all
// three declarations. A kind that stopped being joined here would go silently
// undocumented, which is the failure the whole declaration exists to end.
func TestDeclaration_CoversEveryVocabulary(t *testing.T) {
	t.Parallel()

	declaration := supportref.Declaration()

	for _, want := range []parity.Kind{
		parity.KindOption,
		parity.KindModeFlag,
		parity.KindAction,
	} {
		found := slices.ContainsFunc(declaration, func(word parity.Word) bool {
			return word.Kind == want
		})

		if !found {
			t.Errorf("the joined declaration holds no %s at all", want)
		}
	}
}

func TestRewrite_ReplacesOnlyTheGeneratedRegion(t *testing.T) {
	t.Parallel()

	document := "before\n\n" + supportref.BeginMarker + "\nstale\n" + supportref.EndMarker + "\n\nafter\n"

	rewritten, err := supportref.Rewrite(document)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	if strings.Contains(rewritten, "stale") {
		t.Error("the stale region survived the rewrite")
	}

	for _, kept := range []string{"before", "after"} {
		if !strings.Contains(rewritten, kept) {
			t.Errorf("the rewrite dropped %q, which is outside the markers", kept)
		}
	}

	// Rewriting twice changes nothing: the guardrail compares a regenerated
	// document against the page, so a rewrite that was not a fixed point would
	// fail the build on a page nobody had touched.
	twice, err := supportref.Rewrite(rewritten)
	if err != nil {
		t.Fatalf("Rewrite twice: %v", err)
	}

	if twice != rewritten {
		t.Error("rewriting an already-generated document changed it again")
	}
}

func TestRewrite_ReportsADocumentWithNoRegion(t *testing.T) {
	t.Parallel()

	_, rewriteErr := supportref.Rewrite("a page with no markers\n")
	if rewriteErr == nil {
		t.Error("Rewrite accepted a document with no generated region")
	}

	_, regionErr := supportref.Region(supportref.BeginMarker + "\nonly the opening\n")
	if regionErr == nil {
		t.Error("Region accepted a document with no closing marker")
	}
}
