//go:build integration && darwin

package vision

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// inkCenter is where a character's ink is centered on a fixture line, in
// points, measured by drawing it alone in red in its place.
type inkCenter struct {
	char string
	x    float64
}

// cjkInk is each character of each line of cjk-16pt-2x.png with its ink
// center, spaces left out.
//
//nolint:gosmopolitan
var cjkInk = [][]inkCenter{
	{
		{"今", 19.8},
		{"天", 35.5},
		{"的", 51.5},
		{"天", 67.2},
		{"气", 83.2},
		{"很", 99.2},
		{"好", 115.2},
		{"，", 130.8},
		{"我", 146.8},
		{"们", 162.5},
		{"去", 178.5},
		{"公", 194.5},
		{"园", 210.2},
		{"散", 226.2},
		{"步", 242.0},
		{"。", 258.2},
	},
	{
		{"「", 17.0},
		{"中", 28.0},
		{"文", 44.0},
		{"」", 55.0},
		{"测", 73.0},
		{"试", 89.2},
		{"（", 107.0},
		{"括", 118.2},
		{"号", 134.0},
		{"）", 145.0},
		{"与", 162.0},
		{"E", 177.8},
		{"n", 186.8},
		{"g", 195.5},
		{"l", 202.2},
		{"i", 206.0},
		{"s", 212.0},
		{"h", 220.5},
		{"混", 234.8},
		{"合", 250.5},
	},
	{
		{"日", 20.0},
		{"本", 35.2},
		{"語", 51.8},
		{"の", 67.0},
		{"テ", 82.2},
		{"キ", 97.5},
		{"ス", 112.5},
		{"ト", 129.0},
	},
	{
		{"한", 19.0},
		{"국", 32.5},
		{"어", 45.8},
		{"텍", 64.2},
		{"스", 78.5},
		{"트", 92.0},
		{"를", 106.0},
		{"읽", 123.8},
		{"습", 138.0},
		{"니", 151.5},
		{"다", 166.2},
	},
}

// TestReadLines_PutsEachWideCharacterOverItsInk pins ReadLines on Chinese,
// Japanese and Korean. With no language set the recognizer reads English
// alone. The gaps inside a character such as 测 split it in two, and
// punctuation such as 」 is set narrower than a character, so an even split
// drifts. Each character has to come back as a piece of its own whose box
// holds the character's ink, and the boxes have to be centered on the ink
// within maxMeanInkOffset points on average. The even split this replaced
// was 2.4 points off.
func TestReadLines_PutsEachWideCharacterOverItsInk(t *testing.T) {
	frame := loadFixtureFrame(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)

	lines := map[int][]readPiece{}
	for _, piece := range readLinesFromImage(frame, 2) {
		if strings.TrimSpace(piece.text) != "" {
			lines[piece.line] = append(lines[piece.line], piece)
		}
	}

	if len(lines) != len(cjkInk) {
		t.Fatalf("read %d lines, want %d", len(lines), len(cjkInk))
	}

	offsets, count := 0.0, 0

	for index, want := range cjkInk {
		got := lines[index]
		if len(got) != len(want) {
			t.Errorf(
				"line %d read as %d pieces %v, want one per character of %v",
				index,
				len(got),
				got,
				want,
			)

			continue
		}

		for offset, piece := range got {
			ink := want[offset]
			if piece.text != ink.char {
				t.Errorf("line %d piece %d is %q, want %q", index, offset, piece.text, ink.char)

				continue
			}

			offsets += math.Abs(piece.x + piece.width/2 - ink.x)
			count++

			if ink.x < piece.x || ink.x > piece.x+piece.width {
				t.Errorf("line %d: %q spans %.1f to %.1f, which misses its ink at %.1f",
					index, piece.text, piece.x, piece.x+piece.width, ink.x)
			}
		}
	}

	if count > 0 && offsets/float64(count) > maxMeanInkOffset {
		t.Errorf("boxes are %.2f points from their ink on average, want at most %.2f",
			offsets/float64(count), maxMeanInkOffset)
	}
}

// maxMeanInkOffset is how far, in points, a box's center may be from its
// character's ink on average.
const maxMeanInkOffset = 1.25
