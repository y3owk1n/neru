package screentext

import (
	"image"
	"strings"
	"unicode"

	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Characters splits s into the characters an app moves over one at a time,
// its grapheme clusters, so a Thai vowel sign stays with its consonant.
func Characters(s string) []string {
	var chars []string

	for char := graphemes.FromString(s); char.Next(); {
		chars = append(chars, char.Value())
	}

	return chars
}

// cellsOf is how many cells char takes on screen: two for a wide character
// such as 中 or 한, and one for the rest.
func cellsOf(char string) int {
	return max(displaywidth.String(char), 1)
}

// rightToLeftScripts are the scripts written right to left that the screen
// is read in.
var rightToLeftScripts = []*unicode.RangeTable{
	unicode.Arabic, unicode.Hebrew, unicode.Syriac, unicode.Thaana, unicode.Nko,
}

// rightToLeft reports whether s is written right to left, by the script of
// its first letter.
func rightToLeft(s string) bool {
	for _, char := range s {
		if unicode.IsLetter(char) {
			return unicode.In(char, rightToLeftScripts...)
		}
	}

	return false
}

// shares splits bounds among chars by the cells each takes, so a wide
// character gets twice a narrow one's width. A word written right to left
// starts at the right.
func shares(bounds image.Rectangle, chars []string) []image.Rectangle {
	total := 0
	for _, char := range chars {
		total += cellsOf(char)
	}

	boxes := make([]image.Rectangle, 0, len(chars))
	reverse := rightToLeft(strings.Join(chars, ""))
	at := 0

	for _, char := range chars {
		left := bounds.Dx() * at / total
		at += cellsOf(char)
		right := max(bounds.Dx()*at/total, left+1)

		if reverse {
			left, right = bounds.Dx()-right, bounds.Dx()-left
		}

		boxes = append(
			boxes,
			image.Rect(bounds.Min.X+left, bounds.Min.Y, bounds.Min.X+right, bounds.Max.Y),
		)
	}

	return boxes
}
