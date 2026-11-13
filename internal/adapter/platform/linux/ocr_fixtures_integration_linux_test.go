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

	"github.com/y3owk1n/neru/internal/derrors"
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

// TestRecognizeLines_ReadsEachCharacterInItsLineAndWord pins the read vim
// mode lays the screen out from. Each fixture phrase comes back a character at
// a time, numbered with one line, a word's characters sharing a word number
// and the next word having the next one. The boxes are not pinned, since the
// LSTM engine places a character only roughly, a "t" sometimes covering the
// "e" before it.
func TestRecognizeLines_ReadsEachCharacterInItsLineAndWord(t *testing.T) {
	requireEngine(t)

	paths, err := filepath.Glob(filepath.Join("..", "testdata", "ocr", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no OCR fixtures found: %v", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			symbols, _, err := RecognizeLines(loadOCRFixture(t, path), OCRParams{TimeoutMS: 10000})
			if err != nil {
				t.Fatalf("RecognizeLines: %v", err)
			}

			lines := map[int]string{}
			last := map[int]OCRSymbol{}

			for _, symbol := range symbols {
				if previous, ok := last[symbol.Line]; ok {
					if symbol.Word != previous.Word && symbol.Word != previous.Word+1 {
						t.Fatalf(
							"line %d jumps from word %d to %d",
							symbol.Line,
							previous.Word,
							symbol.Word,
						)
					}

					if symbol.Word != previous.Word {
						lines[symbol.Line] += " "
					}
				}

				lines[symbol.Line] += symbol.Text
				last[symbol.Line] = symbol
			}

			for _, want := range ocrFixturePhrases {
				found := false
				for _, line := range lines {
					found = found || strings.Contains(line, want)
				}

				if !found {
					t.Errorf("no line read has %q: %v", want, lines)
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

// TestRecognizeLines_StopsWhenDoneCloses pins what canceling a line read
// means. A read whose Done closes stops and returns no lines and
// CodeContextCanceled, never the part read before it stopped, so vim mode can
// drop a read for a screen a scroll has since moved away.
func TestRecognizeLines_StopsWhenDoneCloses(t *testing.T) {
	requireEngine(t)

	paths, err := filepath.Glob(filepath.Join("..", "testdata", "ocr", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no OCR fixtures found: %v", err)
	}

	done := make(chan struct{})
	close(done)

	symbols, _, err := RecognizeLines(
		loadOCRFixture(t, paths[0]),
		OCRParams{TimeoutMS: 10000, Done: done},
	)
	if !derrors.IsCode(err, derrors.CodeContextCanceled) {
		t.Fatalf("RecognizeLines after Done closed = %v, want CodeContextCanceled", err)
	}

	if symbols != nil {
		t.Fatalf("a canceled read returned %d characters, want none", len(symbols))
	}

	symbols, _, err = RecognizeLines(loadOCRFixture(t, paths[0]), OCRParams{TimeoutMS: 10000})
	if err != nil || len(symbols) == 0 {
		t.Fatalf("the read after a canceled one = %d characters, %v, want text", len(symbols), err)
	}
}
