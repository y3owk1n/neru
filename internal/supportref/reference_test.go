package supportref_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/parity"
	"github.com/y3owk1n/neru/internal/supportref"
)

func TestTable_NamesEveryNarrowWord(t *testing.T) {
	t.Parallel()

	table := supportref.Table()

	limited := supportref.Declaration().Limited()
	if len(limited) == 0 {
		t.Fatal("nothing is declared as narrower than every platform; the table would be empty")
	}

	for _, word := range limited {
		// Named in full, so a word copied from a load-time warning finds its row.
		if !strings.Contains(table, "`"+word.Written()+"`") {
			t.Errorf("the table does not name %q", word.Written())
		}
	}
}

// TestTable_RendersOneRowPerReason is what keeps the table short. Words that
// share a platform column and a reason share a row, so the reason is read once.
func TestTable_RendersOneRowPerReason(t *testing.T) {
	t.Parallel()

	table := supportref.Table()
	groups := supportref.Groups()

	// One row per group, plus the header and the separator.
	rows := strings.Count(strings.TrimSpace(table), "\n") + 1
	if rows != len(groups)+2 {
		t.Errorf("the table has %d lines for %d reasons, want %d", rows, len(groups), len(groups)+2)
	}

	seen := map[string]bool{}

	for _, group := range groups {
		key := fmt.Sprint(group[0].Platforms) + "|" + group[0].Note
		if seen[key] {
			t.Errorf(
				"two rows share the column %v and the reason %q",
				group[0].Platforms,
				group[0].Note,
			)
		}

		seen[key] = true

		for _, word := range group {
			if word.Note != group[0].Note || !slices.Equal(word.Platforms, group[0].Platforms) {
				t.Errorf(
					"%q is grouped under a reason or column that is not its own",
					word.Written(),
				)
			}
		}
	}
}

// TestTable_LeavesOutTheWordsThatWorkEverywhere is the whole reason the table is
// readable. The declarations name several hundred words, and a table of all of
// them would bury the dozen a reader is actually looking for.
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
