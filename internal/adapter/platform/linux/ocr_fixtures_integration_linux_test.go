//go:build integration && linux

package linux

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ocrFixturePhrases are lines every fixture frame in ../testdata/ocr holds,
// rendered in the macOS system font at UI sizes on a white frame the size of
// a 1080p or 4K screen. Each must come back word for word from the pass hints
// mode runs. A third line, a path with a tilde, is left unchecked, since every
// engine reads its punctuation a little differently.
var ocrFixturePhrases = []string{"denotes breaking CLI changes.", "Settings Downloads Help"}

// TestRecognizeText_ReadsSmallUIText pins that tesseract reads small UI text
// word for word from a whole screen-sized frame. copy_text relies on it here,
// since Linux has no second, more careful read.
func TestRecognizeText_ReadsSmallUIText(t *testing.T) {
	requireEngine(t)

	paths, err := filepath.Glob(filepath.Join("..", "testdata", "ocr", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no OCR fixtures found: %v", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			words, _, err := RecognizeText(loadOCRFixture(t, path), OCRParams{TimeoutMS: 10000})
			if err != nil {
				t.Fatalf("RecognizeText: %v", err)
			}

			texts := make([]string, 0, len(words))
			for _, word := range words {
				texts = append(texts, word.Text)
			}

			read := strings.Join(strings.Fields(strings.Join(texts, " ")), " ")

			for _, want := range ocrFixturePhrases {
				if !strings.Contains(read, want) {
					t.Errorf("read %q, want it to contain %q", read, want)
				}
			}
		})
	}
}

// loadOCRFixture decodes a fixture frame into the RGBA a capture produces.
func loadOCRFixture(t *testing.T, path string) *image.RGBA {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	frame := image.NewRGBA(decoded.Bounds())
	draw.Draw(frame, frame.Rect, decoded, decoded.Bounds().Min, draw.Src)

	return frame
}
