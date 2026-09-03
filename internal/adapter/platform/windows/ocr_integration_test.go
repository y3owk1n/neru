//go:build integration && windows

package windows

import (
	"context"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// These are the only tests that run Windows.Media.Ocr. Everything else about
// the backend — how a word's box lands on the screen, which failures a caller
// should stop retrying — is decided in Go and tested without an engine;
// whether the bridge actually reads text cannot be.
//
// The end-to-end test paints a word on the overlay, captures it back off the
// desktop and asks the engine what it says, which is the whole vision
// strategy in one pass. It needs an interactive desktop, like the capture
// tests beside it, and an OCR language pack, which every stock Windows image
// installs for its display language.
//
// Run with: go test -tags=integration ./internal/adapter/platform/windows/

// requireEngine skips when this machine has no OCR language pack, naming what
// it is missing the same way the adapter does.
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

// TestRecognizeText_ReadsAWordOffTheScreen is the acceptance check for the
// vision strategy on Windows: a known word rendered into a window is captured
// and found, at the rectangle it was drawn in.
func TestRecognizeText_ReadsAWordOffTheScreen(t *testing.T) {
	requireEngine(t)

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	const word = "HELLO"

	// A white card with the word in black, sized so the text is a comfortable
	// size for the engine and the card has margin around it.
	card := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+440, screen.Min.Y+200)
	textBox := card.Inset(40)

	overlay := paintedOverlay(t, card, 0xFFFFFFFF)
	overlay.DrawTextCentered(word, textBox, "", 48, 0xFF000000)

	flushErr := overlay.Flush()
	if flushErr != nil {
		t.Fatalf("Flush: %v", flushErr)
	}

	time.Sleep(100 * time.Millisecond)

	img, err := CaptureScreenRegion(context.Background(), card)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%v): %v", card, err)
	}

	words, stats, err := RecognizeText(
		context.Background(),
		img,
		OCRParams{WordLevel: true, TimeoutMS: 5000},
	)
	if err != nil {
		t.Fatalf("RecognizeText: %v", err)
	}

	t.Logf("recognized %d runs in %s", len(words), stats.Recognition)

	// Where the word should be, in the captured frame's own space.
	expected := textBox.Sub(card.Min)

	for _, got := range words {
		if !strings.EqualFold(got.Text, word) {
			continue
		}

		if !got.Bounds.In(img.Rect) {
			t.Errorf("word box %v is outside the %v frame", got.Bounds, img.Rect)
		}

		if !got.Bounds.Overlaps(expected) {
			t.Errorf("word box %v does not overlap the %v it was drawn in", got.Bounds, expected)
		}

		if got.Bounds.Dx() < expected.Dx()/4 {
			t.Errorf("word box %v is far narrower than the text drawn into the frame", got.Bounds)
		}

		if got.Confidence <= 0 || got.Confidence > 1 {
			t.Errorf("confidence %v is outside 0..1", got.Confidence)
		}

		return
	}

	// The frame is a region of a live desktop, so what the engine read there
	// is screen content: only the boxes are logged, never the text.
	for _, got := range words {
		t.Logf("recognized a run of %d characters at %v", len(got.Text), got.Bounds)
	}

	t.Errorf("recognized %d runs, none of them %q", len(words), word)
}

// TestRecognizeText_BlankFrameIsNotAnError pins the difference between "there
// is no text on screen" and "recognition failed". A caller that could not tell
// them apart would report an error every time a user pointed hints at an image
// viewer. The frame is a buffer, so this needs no desktop.
func TestRecognizeText_BlankFrameIsNotAnError(t *testing.T) {
	requireEngine(t)

	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	words, _, err := RecognizeText(context.Background(), img, OCRParams{TimeoutMS: 5000})
	if err != nil {
		t.Fatalf("RecognizeText on a blank frame: %v", err)
	}

	if len(words) != 0 {
		t.Errorf("found %d runs of text in a blank frame", len(words))
	}
}

// TestRecognizeText_AcceptsAFrameWiderThanTheEngineLimit pins the resample:
// a frame past MaxImageDimension, which any 4K monitor is, is read rather
// than refused, and comes back in its own coordinate space.
func TestRecognizeText_AcceptsAFrameWiderThanTheEngineLimit(t *testing.T) {
	requireEngine(t)

	img := image.NewRGBA(image.Rect(0, 0, 3840, 200))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	words, _, err := RecognizeText(context.Background(), img, OCRParams{TimeoutMS: 10000})
	if err != nil {
		t.Fatalf("RecognizeText on a 3840-wide frame: %v", err)
	}

	for _, got := range words {
		if !got.Bounds.In(img.Rect) {
			t.Errorf("word box %v is outside the %v frame it was read from", got.Bounds, img.Rect)
		}
	}
}

// TestRecognizeText_RefusesADegenerateImage keeps a caller from reading "no
// words" out of an image the engine never looked at.
func TestRecognizeText_RefusesADegenerateImage(t *testing.T) {
	words, _, err := RecognizeText(
		context.Background(),
		image.NewRGBA(image.Rectangle{}),
		OCRParams{},
	)
	if err == nil {
		t.Fatalf("RecognizeText accepted an empty image and returned %d words", len(words))
	}
}
