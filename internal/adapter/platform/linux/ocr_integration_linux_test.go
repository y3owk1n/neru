//go:build integration && linux

package linux

import (
	"image"
	"strings"
	"testing"

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

	words, err := RecognizeText(img, OCRParams{WordLevel: true, TimeoutMS: 5000})
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

	words, err := RecognizeText(img, OCRParams{TimeoutMS: 5000})
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

	words, err := RecognizeText(image.NewRGBA(image.Rectangle{}), OCRParams{})
	if err == nil {
		t.Fatalf("RecognizeText accepted an empty image and returned %d words", len(words))
	}
}

// TestRecognizeText_SurvivesRepeatedUse exercises the cached engine: the handle
// is created once and reused, so the second recognition must find it warm and
// free of the first frame rather than in whatever state that call left it.
func TestRecognizeText_SurvivesRepeatedUse(t *testing.T) {
	requireEngine(t)

	img := renderBlockText("HELLO", image.Pt(60, 40), 8)

	for attempt := range 3 {
		words, err := RecognizeText(img, OCRParams{WordLevel: true, TimeoutMS: 5000})
		if err != nil {
			t.Fatalf("RecognizeText call %d: %v", attempt+1, err)
		}

		if len(words) == 0 {
			t.Fatalf("RecognizeText call %d found nothing", attempt+1)
		}
	}
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

	return img
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
