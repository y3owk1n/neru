//go:build integration && darwin

package vision

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadText_ReadsEachFixtureLineExactly pins the second look copy_text
// takes at a hint found on screen. Each line of each fixture frame in
// ../platform/testdata/ocr, cropped as a hint's box would be, has to come back
// word for word, which the whole-frame pass does not manage. Each crop is
// read at readTextScale pixels per point, the scale ReadText captures at. The
// path line is checked only up to its flag, since the recognizer reads "--"
// and "~/" inconsistently at any size.
func TestReadText_ReadsEachFixtureLineExactly(t *testing.T) {
	want := []string{
		"denotes breaking CLI changes.",
		"run neru",
		"Settings Downloads Help",
	}

	paths, err := filepath.Glob(filepath.Join("..", "platform", "testdata", "ocr", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no OCR fixtures found: %v", err)
	}

	for _, path := range paths {
		name := filepath.Base(path)

		// The 4K frames hold two pixels per point, the 1080p ones one.
		pixelsPerPoint := 1
		if strings.HasPrefix(name, "4k") {
			pixelsPerPoint = 2
		}

		captureScale := float64(readTextScale) / float64(pixelsPerPoint)

		t.Run(name, func(t *testing.T) {
			frame := loadFixtureFrame(t, path)

			lines := textLines(frame, readTextPadding*pixelsPerPoint)
			if len(lines) != len(want) {
				t.Fatalf("found %d text lines in the frame, want %d", len(lines), len(want))
			}

			for index, line := range lines {
				crop := image.NewRGBA(image.Rect(0, 0, line.Dx(), line.Dy()))
				draw.Draw(crop, crop.Rect, frame, line.Min, draw.Src)

				got := readTextFromImage(crop, captureScale)
				if !strings.HasPrefix(got, want[index]) {
					t.Errorf(
						"line %d read as %q, want it to start with %q",
						index,
						got,
						want[index],
					)
				}
			}
		})
	}
}

// textLines finds each band of rows holding dark pixels and returns its box,
// widened by padding pixels as ReadText pads a hint's box.
func textLines(frame *image.RGBA, padding int) []image.Rectangle {
	dark := func(col, row int) bool {
		pixel := frame.RGBAAt(col, row)

		return int(pixel.R)+int(pixel.G)+int(pixel.B) < 3*160
	}

	var lines []image.Rectangle

	top := -1

	for row := frame.Rect.Min.Y; row <= frame.Rect.Max.Y; row++ {
		minX, maxX := frame.Rect.Max.X, frame.Rect.Min.X

		if row < frame.Rect.Max.Y {
			for col := frame.Rect.Min.X; col < frame.Rect.Max.X; col++ {
				if dark(col, row) {
					minX, maxX = min(minX, col), max(maxX, col)
				}
			}
		}

		switch {
		case minX <= maxX && top < 0:
			top = row
		case minX > maxX && top >= 0:
			lines = append(lines, lineBox(frame, top, row, padding, dark))
			top = -1
		}
	}

	return lines
}

// lineBox is the box around the dark pixels in rows top to bottom, widened
// by padding pixels.
func lineBox(
	frame *image.RGBA,
	top, bottom, padding int,
	dark func(col, row int) bool,
) image.Rectangle {
	minX, maxX := frame.Rect.Max.X, frame.Rect.Min.X

	for row := top; row < bottom; row++ {
		for col := frame.Rect.Min.X; col < frame.Rect.Max.X; col++ {
			if dark(col, row) {
				minX, maxX = min(minX, col), max(maxX, col)
			}
		}
	}

	return image.Rect(minX, top, maxX+1, bottom).Inset(-padding).Intersect(frame.Rect)
}

// loadFixtureFrame decodes a fixture frame into the RGBA a capture produces.
func loadFixtureFrame(t *testing.T, path string) *image.RGBA {
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
