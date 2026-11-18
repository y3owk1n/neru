//go:build integration && windows

package vision

import (
	"context"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.uber.org/zap"

	platformwindows "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestLinesFromWords_ReadsALineACharacterAtATime pins Windows vim mode's
// reading. Windows.Media.Ocr gives whole words, and each word comes back as
// one piece per character, in reading order, with cells that never overlap
// and all fall inside the word's own box.
func TestLinesFromWords_ReadsALineACharacterAtATime(t *testing.T) {
	err := platformwindows.OCRHealth()
	if err != nil {
		t.Skipf("skipping: no OCR engine (%v)", err)
	}

	img := loadWindowsFixture(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "mixed-line-16pt-2x.png"),
	)
	region := image.Rect(0, 0, img.Rect.Dx()/2, img.Rect.Dy()/2)

	words, _, err := platformwindows.RecognizeText(
		context.Background(), img, platformwindows.OCRParams{WordLevel: true, TimeoutMS: 10000},
	)
	if err != nil {
		t.Fatalf("RecognizeText: %v", err)
	}

	lines := linesFromWords(words, img, region)
	if len(lines) == 0 {
		t.Fatal("read no lines")
	}

	var text strings.Builder

	for index, piece := range lines[0] {
		if len([]rune(piece.Text)) != 1 {
			t.Fatalf("piece %d is %q, want one character", index, piece.Text)
		}

		if index > 0 && lines[0][index-1].Word == piece.Word &&
			piece.Bounds.Min.X < lines[0][index-1].Bounds.Max.X {
			t.Fatalf("%q at %v overlaps %q at %v",
				piece.Text, piece.Bounds, lines[0][index-1].Text, lines[0][index-1].Bounds)
		}

		text.WriteString(piece.Text)
	}

	if !strings.Contains(text.String(), "Variousversions") {
		t.Fatalf("line 0 reads %q, want it to start with Various versions", text.String())
	}
}

// TestReadFrameLines_ReadsChineseInTheAccountsLanguage pins the wide read an
// accurate read makes on Windows. With a language above Chinese in the
// account's list, the profile engine drops the Chinese line of
// cjk-16pt-2x.png, and the accurate read, which also reads with the engine
// for the account's Chinese, Japanese or Korean language, gives it back.
func TestReadFrameLines_ReadsChineseInTheAccountsLanguage(t *testing.T) {
	ready, err := platformwindows.WideOCRReady()
	if err != nil && !derrors.IsNotSupported(err) {
		t.Fatalf("WideOCRReady: %v", err)
	}

	if !ready {
		t.Skipf(
			"skipping: no Chinese, Japanese or Korean OCR engine below the profile one (%v)",
			err,
		)
	}

	img := loadWindowsFixture(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)
	region := image.Rect(0, 0, img.Rect.Dx()/2, img.Rect.Dy()/2)
	adapter := &Adapter{logger: zap.NewNop()}

	fast, err := adapter.readFrameLines(context.Background(), region, img, ports.TextFast)
	if err != nil {
		t.Fatalf("fast read: %v", err)
	}

	if slices.ContainsFunc(fast, lineInOtherScript) {
		t.Fatal("the profile engine already reads the Chinese, so the fixture pins nothing here")
	}

	accurate, err := adapter.readFrameLines(context.Background(), region, img, ports.TextAccurate)
	if err != nil {
		t.Fatalf("accurate read: %v", err)
	}

	slices.SortFunc(accurate, func(a, b []ports.TextPiece) int {
		return lineBounds(a).Min.Y - lineBounds(b).Min.Y
	})

	if len(accurate) == 0 || !lineInOtherScript(accurate[0]) {
		t.Fatalf("the accurate read's top line is not in Chinese: %v", accurate)
	}
}

// loadWindowsFixture decodes a PNG fixture into the RGBA frame a capture gives.
func loadWindowsFixture(t *testing.T, path string) *image.RGBA {
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
