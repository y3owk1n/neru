//go:build integration && darwin

package vision

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadImageLines_StopsOnceCanceled pins that a read whose cancel flag is
// set returns nothing, as a scroll that moved the screen on cancels the read
// of the screen it left, and that the next read still reads.
func TestReadImageLines_StopsOnceCanceled(t *testing.T) {
	frame := loadFixtureFrame(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)

	if readLinesFromImageCanceled(frame, 2) {
		t.Fatal("a canceled read returned lines, want none")
	}

	if len(readLinesFromImage(frame, 2)) == 0 {
		t.Fatal("the read after a canceled one read nothing")
	}
}

// TestReadLines_PutsEachWideCharacterOverItsInk pins ReadLines on Chinese
// and Japanese. With no language set the recognizer reads English
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
