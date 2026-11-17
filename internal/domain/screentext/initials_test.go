package screentext_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/screentext"
)

const saveAs = "Save As"

// TestWordStarts covers where a word starts in a spelling: after a space or
// symbol, on a camelCase hump or a syllable Fold spells with a capital, on the
// last capital of a run a small letter follows, and where letters and digits
// meet.
func TestWordStarts(t *testing.T) {
	tests := []struct {
		folded string
		want   string
	}{
		{folded: saveAs, want: "^    ^ "},
		{folded: "openRecentFile", want: "^   ^     ^   "},
		{folded: "ShenZhen", want: "^   ^   "},
		{folded: "XMLParser", want: "^  ^     "},
		{folded: "File2go", want: "^   ^^ "},
		{folded: "a-b", want: "^ ^"},
	}

	for _, test := range tests {
		starts := screentext.WordStarts(test.folded)

		got := make([]byte, len(starts))
		for index, start := range starts {
			got[index] = ' '
			if start {
				got[index] = '^'
			}
		}

		if string(got) != test.want {
			t.Errorf("WordStarts(%q) = %q, want %q", test.folded, got, test.want)
		}
	}
}

// TestInitials covers letters that land on word starts in order, each right
// after the letter before it or on the next word start on its line, and every
// byte the last can land on. The first letter starts a word, no letter skips a
// word, and none crosses a line.
func TestInitials(t *testing.T) {
	tests := []struct {
		folded string
		query  string
		at     int
		want   []int
	}{
		{folded: saveAs, query: "sas", at: 0, want: []int{6}},
		{folded: saveAs, query: "sa", at: 0, want: []int{1, 5}},
		{folded: saveAs, query: "sava", at: 0, want: []int{5}},
		{folded: "ShenZhen", query: "sz", at: 0, want: []int{4}},
		{folded: "Open Recent File", query: "of", at: 0},
		{folded: saveAs, query: "as", at: 1},
		{folded: "Save\nAs", query: "sa", at: 0, want: []int{1}},
		{folded: "So\nAs", query: "sa", at: 0},
	}

	for _, test := range tests {
		spelling := []byte(test.folded)
		for index, char := range spelling {
			if char >= 'A' && char <= 'Z' {
				spelling[index] = char + 'a' - 'A'
			}
		}

		got := screentext.Initials(
			string(spelling),
			screentext.WordStarts(test.folded),
			test.query,
			test.at,
		)
		if !slices.Equal(got, test.want) {
			t.Errorf(
				"Initials(%q, %q, %d) = %v, want %v",
				test.folded,
				test.query,
				test.at,
				got,
				test.want,
			)
		}
	}
}
