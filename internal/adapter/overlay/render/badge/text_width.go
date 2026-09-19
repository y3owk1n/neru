package badge

import (
	"math"
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// asciiRunes is how many runes a width table holds. The labels, badges and
// indicators drawn here are keys and digits, so a table indexed by the rune
// answers nearly every one of them without hashing a string.
const asciiRunes = 128

// widthKey is one realized font, which is what a rune's width depends on.
type widthKey struct {
	family string
	size   float64
	bold   bool
}

// widthTable is the measured advance of each ASCII rune at one font, filled in
// as runes are first drawn.
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
// Widths are summed rune by rune from a table per font, so a draw that sizes
// hundreds of hint labels asks the platform once per distinct character rather
// than once per label, and a repeat draw makes no call to it. Summing ignores
// kerning, which can only make a box slightly wider.
//
// Where the platform cannot measure, the answer is EstimateTextWidth's, so a
// box is never sized from nothing.
func TextWidth(text, family string, fontSize float64, bold bool) int {
	if text == "" || fontSize <= 0 {
		return EstimateTextWidth(text, fontSize)
	}

	table := widthTableFor(widthKey{family: family, size: fontSize, bold: bold})

	// A label whose characters are all in the table is summed under one read
	// lock. That is every draw after the first, and the path hundreds of hint
	// labels take on a keypress.
	if total, known := table.sumKnown(text); known {
		return int(math.Ceil(total))
	}

	var total float64

	for _, character := range text {
		width, measured := table.widthOf(character, family, fontSize, bold)
		if !measured {
			return EstimateTextWidth(text, fontSize)
		}

		total += width
	}

	return int(math.Ceil(total))
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

// widthOf answers one rune's width, measuring it on first use. A rune outside
// ASCII is measured every time it is asked for, which the platform measurer
// remembers per string.
func (t *widthTable) widthOf(
	character rune,
	family string,
	size float64,
	bold bool,
) (float64, bool) {
	inTable := character < asciiRunes

	if inTable {
		t.mu.RLock()
		width, known := t.widths[character], t.known[character]
		t.mu.RUnlock()

		if known {
			return width, true
		}
	}

	metrics, err := ports.MeasureText(string(character), family, size, bold)
	if err != nil {
		// Not remembered, so a text layer that was briefly unusable is asked again.
		return 0, false
	}

	if inTable {
		t.mu.Lock()
		t.widths[character], t.known[character] = metrics.Width, true
		t.mu.Unlock()
	}

	return metrics.Width, true
}
