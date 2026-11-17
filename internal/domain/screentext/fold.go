package screentext

import (
	"strings"
	"unicode/utf8"

	anyascii "github.com/anyascii/go"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Fold spells text in ASCII, so the keys of a US layout find text in any
// script: 深圳 folds to ShenZhen, Борис to Boris and René to Rene. ASCII stays
// as it is, and every other character becomes its romanization, which can be
// several letters or none, so the result is all ASCII. For each byte of the
// result, the spans hold the start and end in text of the character it came
// from, a grapheme cluster as Characters splits them, so a Thai vowel sign
// folds with its consonant.
func Fold(text string) (string, [][2]int) {
	var out strings.Builder

	spans := make([][2]int, 0, len(text))

	for char := graphemes.FromString(text); char.Next(); {
		before := out.Len()

		for _, r := range char.Value() {
			if r < utf8.RuneSelf {
				out.WriteRune(r)
			} else {
				out.WriteString(anyascii.TransliterateRune(r))
			}
		}

		span := [2]int{char.Start(), char.End()}
		for range out.Len() - before {
			spans = append(spans, span)
		}
	}

	return out.String(), spans
}
