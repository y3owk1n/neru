//go:build integration && linux

package linux

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// These are the only tests that run tesseract. Everything else about the OCR
// backend — where the language data lives, which failures a caller should stop
// retrying, how a word's box lands on the screen — is decided in Go and tested
// without an engine; whether the bridge actually reads text cannot be.
//
// They need no display server, unlike the capture tests beside them: the
// pixels come from a buffer this file draws. What they need is the language
// data, which docs/LINUX_SETUP.md makes a required install.
//
// Run with: go test -tags=integration ./internal/adapter/platform/linux/

// requireEngine skips when this machine has no language data, naming what it
// is missing the same way the adapter does.
func requireEngine(t *testing.T) {
	t.Helper()

	err := OCRHealth()
	if err == nil {
		return
	}

	if derrors.IsNotSupported(err) {
		t.Skipf("no usable OCR engine: %v", err)
	}

	t.Fatalf("OCRHealth failed for a reason other than a missing engine: %v", err)
}

// TestRecognizeText_ReadsRenderedText is the end-to-end claim the vision hint
// strategy rests on: given pixels, the bridge returns the text in them and
// where it is.
//
// The glyphs are drawn here rather than loaded from a font, so the test needs
// no font package and no rasterizer and cannot fail because a distribution
// shipped a different DejaVu.
func TestRecognizeText_ReadsRenderedText(t *testing.T) {
	requireEngine(t)

	const word = "HELLO"

	img := renderBlockText(word, image.Pt(60, 40), 8)

	words, _, err := RecognizeText(img, OCRParams{WordLevel: true, TimeoutMS: 5000})
	if err != nil {
		t.Fatalf("RecognizeText: %v", err)
	}

	if len(words) == 0 {
		t.Fatal("RecognizeText found no words in an image that is nothing but a word")
	}

	var found bool

	for _, got := range words {
		if !nearlyEqual(strings.ToUpper(strings.TrimSpace(got.Text)), word) {
			continue
		}

		found = true

		if !got.Bounds.In(img.Rect) {
			t.Errorf("word box %v is outside the %v image", got.Bounds, img.Rect)
		}

		if got.Bounds.Dx() < scaledWidth(word, 8)/2 {
			t.Errorf("word box %v is far narrower than the text drawn into the frame", got.Bounds)
		}

		if got.Confidence <= 0 || got.Confidence > 1 {
			t.Errorf("confidence %v is outside 0..1", got.Confidence)
		}
	}

	if !found {
		// Naming the text here is not a privacy question: it is a literal a few
		// lines up, drawn by this test, rather than anything off a screen.
		for _, got := range words {
			t.Logf("recognized %q at %v (confidence %.2f)", got.Text, got.Bounds, got.Confidence)
		}

		t.Errorf("recognized %d runs, none of them close to %q", len(words), word)
	}
}

// nearlyEqual accepts one substituted character.
//
// The word is drawn in the crude 5x7 font below rather than a real typeface, so
// a glyph or two sits close to another letter's shape — tesseract reads this
// font's O as a U, and which characters it confuses would change with the
// engine version and the language pack anyway. Demanding an exact string would
// make the test a report on tesseract's release notes; one substitution still
// only passes if recognition genuinely worked.
func nearlyEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}

	differences := 0

	for i := range want {
		if got[i] != want[i] {
			differences++
		}
	}

	return differences <= 1
}

// TestRecognizeText_BlankFrameIsNotAnError pins the difference between "there
// is no text on screen" and "recognition failed". A caller that could not tell
// them apart would report an error every time a user pointed hints at an image
// viewer.
func TestRecognizeText_BlankFrameIsNotAnError(t *testing.T) {
	requireEngine(t)

	img := renderBlockText("", image.Pt(400, 200), 8)

	words, _, err := RecognizeText(img, OCRParams{TimeoutMS: 5000})
	if err != nil {
		t.Fatalf("RecognizeText on a blank frame: %v", err)
	}

	if len(words) != 0 {
		t.Errorf("found %d runs of text in a blank frame", len(words))
	}
}

// TestRecognizeText_RefusesADegenerateImage keeps a caller from reading "no
// words" out of an image the engine never looked at.
func TestRecognizeText_RefusesADegenerateImage(t *testing.T) {
	requireEngine(t)

	words, _, err := RecognizeText(image.NewRGBA(image.Rectangle{}), OCRParams{})
	if err == nil {
		t.Fatalf("RecognizeText accepted an empty image and returned %d words", len(words))
	}
}

// TestRecognizeText_SurvivesRepeatedUse exercises the cached engine, which is
// the one piece of this subsystem that outlives a call.
//
// The handle is created once and reused, so every recognition after the first
// must find it warm and free of the previous frame rather than in whatever
// state that call left it. The frames differ in size on purpose: a reuse bug
// that only shows when the image geometry changes would pass a loop over one
// buffer. The health probe runs between them because the adapter runs it before
// every detection, and it takes the same lock and the same handle.
func TestRecognizeText_SurvivesRepeatedUse(t *testing.T) {
	requireEngine(t)

	frames := []image.Point{{X: 60, Y: 40}, {X: 120, Y: 80}, {X: 20, Y: 10}, {X: 300, Y: 200}}

	for round := range 2 {
		for frame, origin := range frames {
			healthErr := OCRHealth()
			if healthErr != nil {
				t.Fatalf("round %d frame %d: OCRHealth: %v", round, frame, healthErr)
			}

			img := renderBlockText("HELLO", origin, 8)

			words, _, err := RecognizeText(img, OCRParams{WordLevel: true, TimeoutMS: 5000})
			if err != nil {
				t.Fatalf("round %d frame %d (%v): %v", round, frame, img.Rect, err)
			}

			if len(words) == 0 {
				t.Fatalf("round %d frame %d (%v) found nothing", round, frame, img.Rect)
			}
		}
	}
}

// TestRecognizeText_DoesNotDegradeAcrossFrames is the latency guardrail on the
// cached engine, and it exists because the tidy-looking version of this code is
// a sevenfold regression.
//
// The engine is reused across activations, so anything it accumulates is paid
// for by every later frame. Clearing the adaptive classifier between frames —
// which reads like the correct hygiene, since it holds shapes learned from the
// frame just read — also resets the document dictionary, and reinitializing
// that took recognition of this frame from 0.5s to 3.5s. The opposite mistake,
// letting real state grow unboundedly, would show here as the same shape.
//
// The bound is deliberately loose. This asserts against a regression of the
// size that makes the hint strategy unusable, not against runner noise, and a
// dense frame is used because a frame with one word in it exercises none of
// what accumulates.
func TestRecognizeText_DoesNotDegradeAcrossFrames(t *testing.T) {
	requireEngine(t)

	// Window-sized and full of text, which is the shape a real activation hands
	// over on a session where the focused-window bounds cannot be resolved.
	img := renderDenseText(1904, 994)

	var first time.Duration

	for round := range 3 {
		started := time.Now()

		words, stats, err := RecognizeText(img, OCRParams{TimeoutMS: 60000})
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}

		if len(words) == 0 {
			t.Fatalf("round %d found no text in a frame full of it", round)
		}

		elapsed := time.Since(started)
		t.Logf(
			"round %d: %d runs in %s (engine reported %s)",
			round,
			len(words),
			elapsed,
			stats.Recognition,
		)

		if round == 0 {
			first = elapsed

			continue
		}

		if elapsed > 4*first {
			t.Errorf("round %d took %s against %s for the first frame; "+
				"the engine is accumulating something across recognitions",
				round, elapsed, first)
		}
	}
}

// renderDenseText fills a frame with text the way a real screen is dense,
// rather than the single word the other tests draw. What accumulates in the
// engine accumulates per recognized word, so a sparse frame exercises none of
// it.
func renderDenseText(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	const scale = 3

	cellWidth := scaledWidth("HELLO", scale) + blockGlyphWidth*scale
	cellHeight := blockGlyphHeight * scale * 3

	for y := cellHeight; y+cellHeight < height; y += cellHeight {
		for x := cellWidth; x+cellWidth < width; x += cellWidth {
			drawBlockText(img, "HELLO", image.Pt(x, y), scale)
		}
	}

	return img
}

// scaledWidth is how wide renderBlockText draws text, ignoring the margin.
func scaledWidth(text string, scale int) int {
	return len(text) * (blockGlyphWidth + blockGlyphSpacing) * scale
}

// blockGlyphs is a 5x7 bitmap font covering exactly the letters these tests
// draw. It exists so the tests carry their own text rendering: adding a font
// rasterizer as a dependency to draw five letters would be a heavier price than
// the test is worth.
const (
	blockGlyphWidth   = 5
	blockGlyphHeight  = 7
	blockGlyphSpacing = 1
)

// Each glyph is one string of blockGlyphHeight newline-separated rows, so the
// shape reads as the letter it draws.
var blockGlyphs = map[rune]string{
	'H': "#...#\n#...#\n#...#\n#####\n#...#\n#...#\n#...#",
	'E': "#####\n#....\n#....\n####.\n#....\n#....\n#####",
	'L': "#....\n#....\n#....\n#....\n#....\n#....\n#####",
	'O': ".###.\n#...#\n#...#\n#...#\n#...#\n#...#\n.###.",
}

// renderBlockText draws text in black on white at the given scale, inset by
// origin, on a canvas large enough to hold it with margin. An empty string
// gives a blank white frame.
func renderBlockText(text string, origin image.Point, scale int) *image.RGBA {
	width := origin.X*2 + max(1, len(text))*(blockGlyphWidth+blockGlyphSpacing)*scale
	height := origin.Y*2 + blockGlyphHeight*scale

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	drawBlockText(img, text, origin, scale)

	return img
}

// drawBlockText draws text onto an existing canvas at origin, so a caller can
// place several runs on one frame.
func drawBlockText(img *image.RGBA, text string, origin image.Point, scale int) {
	for index, letter := range text {
		glyph, ok := blockGlyphs[letter]
		if !ok {
			continue
		}

		left := origin.X + index*(blockGlyphWidth+blockGlyphSpacing)*scale

		for row, bits := range strings.Split(glyph, "\n") {
			for col, bit := range bits {
				if bit != '#' {
					continue
				}

				fillBlack(img,
					image.Rect(
						left+col*scale,
						origin.Y+row*scale,
						left+(col+1)*scale,
						origin.Y+(row+1)*scale,
					))
			}
		}
	}
}

func fillBlack(img *image.RGBA, rect image.Rectangle) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			offset := img.PixOffset(x, y)
			img.Pix[offset] = 0x00
			img.Pix[offset+1] = 0x00
			img.Pix[offset+2] = 0x00
			img.Pix[offset+3] = 0xFF
		}
	}
}
