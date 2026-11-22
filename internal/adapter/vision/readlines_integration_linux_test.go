//go:build integration && linux

package vision

import (
	"context"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

// TestReadYomiLines_ReadsChineseAndJapaneseWithoutALocale reads
// cjk-16pt-2x.png with no locale naming Chinese or Japanese. At least
// minCJKAccuracy of the characters must read right. On each line read exactly
// right, every character must come back as a piece of its own whose box holds
// its ink, within maxMeanInkOffset points of it on average. A misread line is
// left out of that, since a wrong character count moves the cells
// placeWideRuns cuts for the characters around it. yomi misreads 4 of the 44
// characters today (们 as ¶」, 散 as 歇, 测 as its traditional form), all on
// the first two lines.
func TestReadYomiLines_ReadsChineseAndJapaneseWithoutALocale(t *testing.T) {
	const minCJKAccuracy = 0.9

	img := loadLinuxFixture(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)
	region := image.Rect(0, 0, img.Rect.Dx()/2, img.Rect.Dy()/2)

	lines, err := readYomiLines(context.Background(), img, region)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	if len(lines) != len(cjkInk) {
		t.Fatalf("read %d lines, want %d", len(lines), len(cjkInk))
	}

	offsets, placed, wrong, total := 0.0, 0, 0, 0

	for index, want := range cjkInk {
		var got []ports.TextPiece
		for _, piece := range lines[index] {
			if strings.TrimSpace(piece.Text) != "" {
				got = append(got, piece)
			}
		}

		var read, expected []rune
		for _, piece := range got {
			read = append(read, []rune(piece.Text)...)
		}

		for _, ink := range want {
			expected = append(expected, []rune(ink.char)...)
		}

		wrong += editDistance(read, expected)
		total += len(expected)

		if string(read) != string(expected) || len(got) != len(want) {
			continue
		}

		for offset, piece := range got {
			ink := want[offset]
			left, right := float64(piece.Bounds.Min.X), float64(piece.Bounds.Max.X)
			offsets += math.Abs((left+right)/2 - ink.x)
			placed++

			if ink.x < left || ink.x > right {
				t.Errorf("line %d: %q spans %.0f to %.0f, which misses its ink at %.1f",
					index, piece.Text, left, right, ink.x)
			}
		}
	}

	if accuracy := 1 - float64(wrong)/float64(total); accuracy < minCJKAccuracy {
		t.Errorf("read %.0f%% of the characters right, want at least %.0f%%",
			100*accuracy, 100*minCJKAccuracy)
	}

	if placed == 0 {
		t.Fatal("no line read exactly right, so no placement to check")
	}

	if offsets/float64(placed) > maxMeanInkOffset {
		t.Errorf("boxes are %.2f points from their ink on average, want at most %.2f",
			offsets/float64(placed), maxMeanInkOffset)
	}
}

// editDistance is the Levenshtein distance between read and want.
func editDistance(read, want []rune) int {
	prev := make([]int, len(want)+1)
	for col := range prev {
		prev[col] = col
	}

	for row := range read {
		cur := make([]int, len(want)+1)
		cur[0] = row + 1

		for col := range want {
			cost := 1
			if read[row] == want[col] {
				cost = 0
			}

			cur[col+1] = min(prev[col+1]+1, cur[col]+1, prev[col]+cost)
		}

		prev = cur
	}

	return prev[len(want)]
}

// TestReadYomiLines_ReadsRussianBesideEnglishWithoutALocale reads
// mixed-line-16pt-2x.png with no locale naming Russian, and expects its
// Russian line in Russian and its English line in English.
func TestReadYomiLines_ReadsRussianBesideEnglishWithoutALocale(t *testing.T) {
	img := loadLinuxFixture(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "mixed-line-16pt-2x.png"),
	)
	region := image.Rect(0, 0, img.Rect.Dx()/2, img.Rect.Dy()/2)

	lines, err := readYomiLines(context.Background(), img, region)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	texts := make([]string, 0, len(lines))

	for _, line := range lines {
		var text strings.Builder
		for _, piece := range line {
			text.WriteString(piece.Text)
		}

		texts = append(texts, text.String())
	}

	if !slices.Contains(texts, "Русскийтекстдляпроверки") {
		t.Fatalf("lines read %q, want the Russian line read in Russian", texts)
	}

	if !slices.ContainsFunc(
		texts,
		func(text string) bool { return strings.HasPrefix(text, "Variousversions") },
	) {
		t.Fatalf("lines read %q, want the English line read in English", texts)
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
