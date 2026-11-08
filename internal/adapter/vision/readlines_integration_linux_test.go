//go:build integration && linux

package vision

import (
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	platformlinux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestReadLinePieces_ReadsChineseInTheLocalesLanguage pins the second read an
// accurate read makes on Linux. eng alone reads the Chinese line of
// cjk-16pt-2x.png as Latin noise. With a Chinese locale, the chi_sim+eng read
// replaces them, and each character comes back as a piece of its own whose box
// holds its ink, within maxMeanInkOffset points of it on average.
func TestReadLinePieces_ReadsChineseInTheLocalesLanguage(t *testing.T) {
	for _, name := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		t.Setenv(name, "")
	}

	t.Setenv("LANG", "zh_CN.UTF-8")

	language, err := platformlinux.WideLanguage()
	if err != nil || language == "" {
		t.Skipf("skipping: no Chinese tesseract data (%v)", err)
	}

	img := loadLinuxFixture(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)
	region := image.Rect(0, 0, img.Rect.Dx()/2, img.Rect.Dy()/2)

	detected, _, err := readLinePieces(img, img, region, platformlinux.OCRParams{})
	if err != nil {
		t.Fatalf("reading in eng: %v", err)
	}

	wide, _, err := readLinePieces(img, img, region, platformlinux.OCRParams{Language: language})
	if err != nil {
		t.Fatalf("reading in %s: %v", language, err)
	}

	lines := mergeWideLines(detected, wide)
	slices.SortFunc(lines, func(a, b []ports.TextPiece) int {
		return lineBounds(a).Min.Y - lineBounds(b).Min.Y
	})

	offsets, count := 0.0, 0

	// The second line's brackets read as Latin letters on tesseract's fast
	// models, and the third line is Japanese, which chi_sim does not read.
	for index, want := range cjkInk[:1] {
		var got []ports.TextPiece
		for _, piece := range lines[index] {
			if strings.TrimSpace(piece.Text) != "" {
				got = append(got, piece)
			}
		}

		if len(got) != len(want) {
			t.Errorf("line %d read as %d pieces %v, want one per character of %v",
				index, len(got), got, want)

			continue
		}

		for offset, piece := range got {
			ink := want[offset]
			if piece.Text != ink.char {
				t.Errorf("line %d piece %d is %q, want %q", index, offset, piece.Text, ink.char)

				continue
			}

			left, right := float64(piece.Bounds.Min.X), float64(piece.Bounds.Max.X)
			offsets += math.Abs((left+right)/2 - ink.x)
			count++

			if ink.x < left || ink.x > right {
				t.Errorf("line %d: %q spans %.0f to %.0f, which misses its ink at %.1f",
					index, piece.Text, left, right, ink.x)
			}
		}
	}

	if count > 0 {
		t.Logf("boxes are %.2f points from their ink on average", offsets/float64(count))
	}

	if count > 0 && offsets/float64(count) > maxMeanInkOffset {
		t.Errorf("boxes are %.2f points from their ink on average, want at most %.2f",
			offsets/float64(count), maxMeanInkOffset)
	}
}

// loadLinuxFixture decodes a PNG fixture into the RGBA frame a capture gives.
func loadLinuxFixture(t *testing.T, path string) *image.RGBA {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}

	img := image.NewRGBA(decoded.Bounds())
	draw.Draw(img, img.Rect, decoded, decoded.Bounds().Min, draw.Src)

	return img
}
