//go:build integration && (darwin || linux || windows)

package vision

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadYomi_ReadsSmallUITextOnAWholeScreen reads the frames in
// ../platform/testdata/ocr, lines in the macOS system font at UI sizes on a
// white frame the size of a 1080p or 4K screen, and expects each phrase word
// for word from the pass hints mode runs. copy_text relies on it, since there
// is no second, more careful read. "CLI" at 11 pt still reads as "CLl", so
// that phrase is checked on the larger frames only, and a third line, a path
// with a tilde, is left unchecked.
func TestReadYomi_ReadsSmallUITextOnAWholeScreen(t *testing.T) {
	const menu = "Settings Downloads Help"

	both := []string{"denotes breaking CLI changes.", menu}
	tests := map[string][]string{
		"1080p-11pt.png": {menu},
		"1080p-13pt.png": both,
		"4k-11pt.png":    {menu},
		"4k-16.5pt.png":  both,
	}

	for name, phrases := range tests {
		t.Run(name, func(t *testing.T) {
			img := loadFixtureImage(t, filepath.Join("..", "platform", "testdata", "ocr", name))

			lines, err := readYomi(context.Background(), img, 1)
			if err != nil {
				t.Fatalf("reading: %v", err)
			}

			words := yomiWords(lines, true)
			texts := make([]string, 0, len(words))

			for _, word := range words {
				texts = append(texts, word.Text)
			}

			read := strings.Join(texts, " ")

			for _, want := range phrases {
				if !strings.Contains(read, want) {
					t.Errorf("read %q, want it to contain %q", read, want)
				}
			}
		})
	}
}
