package badge

import (
	"math"
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// asciiRunes is how many runes a width table holds. The labels, badges and
	// indicators drawn here are keys and digits, so a table indexed by the rune
	// answers nearly every one of them without hashing a string.
	asciiRunes = 128

	// firstPrintableASCII and lastPrintableASCII bound what WarmTextWidths
	// measures ahead of a draw: everything a keyboard types into a label, a
	// search query or an indicator.
	firstPrintableASCII = ' '
	lastPrintableASCII  = '~'

	// widthReferenceSize is the one size every character is measured at. Text
	// scales close to linearly with its font size, so a width is stored per
	// unit of font size and a table depends on the family and the weight
	// alone, not on the configured size or the display's scale. That is what
	// lets a table be filled before anything is drawn, where the scale is not
	// known, and what keeps the number of tables small. It is large so that
	// hinting, which rounds small sizes to whole pixels, does not skew it.
	widthReferenceSize = 64

	// maxWidthTables bounds the tables kept. A table is a family and a weight,
	// so a configuration uses a handful; somebody trying fonts one after
	// another would otherwise keep every one for the daemon's lifetime.
	maxWidthTables = 32
)

// widthKey is one face: what a rune's width per unit of size depends on.
type widthKey struct {
	family string
	bold   bool
}

// widthTable is the measured advance of each ASCII rune in one face, per unit
// of font size.
type widthTable struct {
	mu     sync.RWMutex
	widths [asciiRunes]float64
	known  [asciiRunes]bool
}

var (
	widthTablesMu sync.RWMutex
	widthTables   = map[widthKey]*widthTable{}
)

// TextWidth returns the width of text at a font, measured by the platform's
// text layer (ports.MeasureText), for sizing the box a label is drawn in.
//
// The estimate it replaces gave every rune 0.7 of the font size, which a W
// outgrows and an I never fills. A box sized that way clips wide labels on a
// backend that clips text to its rectangle, and sits loose around narrow ones.
//
// Widths are summed rune by rune from a table per face. A backend fills the
// table with WarmTextWidths when a configuration is applied, so a draw reads
// it and measures nothing. Summing ignores kerning, which can only make a box
// slightly wider.
//
// A character outside the table, a modifier symbol or a monitor's name, is
// measured the first time it is drawn and remembered by the platform
// measurer. Where the platform cannot measure, the answer is
// EstimateTextWidth's, so a box is never sized from nothing.
func TextWidth(text, family string, fontSize float64, bold bool) int {
	if text == "" || fontSize <= 0 {
		return EstimateTextWidth(text, fontSize)
	}

	table := widthTableFor(widthKey{family: family, bold: bold})

	// A label whose characters are all in the table is summed under one read
	// lock. That is the path hundreds of hint labels take on a keypress.
	if perSize, known := table.sumKnown(text); known {
		return int(math.Ceil(perSize * fontSize))
	}

	var perSize float64

	for _, character := range text {
		width, measured := table.widthOf(character, family, bold)
		if !measured {
			return EstimateTextWidth(text, fontSize)
		}

		perSize += width
	}

	return int(math.Ceil(perSize * fontSize))
}

// WarmTextWidths measures every printable ASCII character, and every character
// of extra, in one face, so that a later TextWidth in that face measures
// nothing. It asks the platform's text layer once per character, which is why
// it belongs where a configuration is applied and never in a draw.
func WarmTextWidths(family string, bold bool, extra string) {
	table := widthTableFor(widthKey{family: family, bold: bold})

	for character := firstPrintableASCII; character <= lastPrintableASCII; character++ {
		if _, measured := table.widthOf(character, family, bold); !measured {
			// The platform cannot measure, so there is nothing to warm.
			return
		}
	}

	for _, character := range extra {
		table.widthOf(character, family, bold)
	}
}

func widthTableFor(key widthKey) *widthTable {
	widthTablesMu.RLock()

	table, found := widthTables[key]

	widthTablesMu.RUnlock()

	if found {
		return table
	}

	widthTablesMu.Lock()
	defer widthTablesMu.Unlock()

	if table, found = widthTables[key]; found {
		return table
	}

	if len(widthTables) >= maxWidthTables {
		clear(widthTables)
	}

	table = &widthTable{}
	widthTables[key] = table

	return table
}

// sumKnown sums a text's widths when the table already holds every character.
func (t *widthTable) sumKnown(text string) (float64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64

	for _, character := range text {
		if character >= asciiRunes || !t.known[character] {
			return 0, false
		}

		total += t.widths[character]
	}

	return total, true
}

// widthOf answers one rune's width per unit of font size, measuring it on
// first use. A rune outside ASCII is asked of the platform measurer every
// time, which remembers it per string.
func (t *widthTable) widthOf(character rune, family string, bold bool) (float64, bool) {
	inTable := character < asciiRunes

	if inTable {
		t.mu.RLock()
		width, known := t.widths[character], t.known[character]
		t.mu.RUnlock()

		if known {
			return width, true
		}
	}

	metrics, err := ports.MeasureText(string(character), family, widthReferenceSize, bold)
	if err != nil {
		// Not remembered, so a text layer that was briefly unusable is asked again.
		return 0, false
	}

	perSize := metrics.Width / widthReferenceSize

	if inTable {
		t.mu.Lock()
		t.widths[character], t.known[character] = perSize, true
		t.mu.Unlock()
	}

	return perSize, true
}
