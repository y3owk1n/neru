package frame_test

import (
	"image"
	"image/color"
	"sync/atomic"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision/frame"
)

func TestDownsample_AveragesEachBlock(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{100, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 200, 0, 255})
	img.SetRGBA(1, 1, color.RGBA{0, 0, 40, 255})

	for x := 2; x < 4; x++ {
		img.SetRGBA(x, 0, color.RGBA{255, 255, 255, 255})
		img.SetRGBA(x, 1, color.RGBA{255, 255, 255, 255})
	}

	out := frame.Downsample(img, 2)

	if got := out.Bounds(); got != image.Rect(0, 0, 2, 1) {
		t.Fatalf("Bounds() = %v, want 2x1", got)
	}

	if got, want := out.RGBAAt(0, 0), (color.RGBA{25, 50, 10, 255}); got != want {
		t.Errorf("left block = %v, want the average %v", got, want)
	}

	if got, want := out.RGBAAt(1, 0), (color.RGBA{255, 255, 255, 255}); got != want {
		t.Errorf("right block = %v, want %v", got, want)
	}
}

func TestDownsample_OddEdgeAveragesWhatItHolds(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.SetRGBA(2, 0, color.RGBA{90, 90, 90, 255})

	out := frame.Downsample(img, 2)

	if got := out.Bounds(); got != image.Rect(0, 0, 2, 1) {
		t.Fatalf("Bounds() = %v, want 2x1", got)
	}

	if got, want := out.RGBAAt(1, 0), (color.RGBA{90, 90, 90, 255}); got != want {
		t.Errorf("edge block = %v, want %v from its one pixel", got, want)
	}
}

func TestDownsample_FactorOneIsTheSameFrame(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	if frame.Downsample(img, 1) != img {
		t.Error("Downsample(img, 1) should return img itself")
	}
}

func TestRows_CoversEveryRowOnce(t *testing.T) {
	t.Parallel()

	for _, height := range []int{0, 1, 7, 1081} {
		seen := make([]atomic.Int32, height)

		frame.Rows(height, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				seen[y].Add(1)
			}
		})

		for y := range seen {
			if n := seen[y].Load(); n != 1 {
				t.Errorf("height %d: row %d visited %d times, want 1", height, y, n)
			}
		}
	}
}

func TestResample_SwapsBGRAAndAverages(t *testing.T) {
	t.Parallel()

	// Two BGRA pixels side by side, with a stride wider than the row.
	pix := []byte{
		10, 20, 30, 0, 50, 60, 70, 0, 99, 99,
	}

	out := frame.Resample(pix, len(pix), 2, 1, frame.BGRA, 2)

	if got, want := out.RGBAAt(0, 0), (color.RGBA{50, 40, 30, 255}); got != want {
		t.Errorf("Resample(BGRA, 2) = %v, want red and blue swapped and averaged %v", got, want)
	}

	copied := frame.Resample(pix, len(pix), 2, 1, frame.BGRA, 1)
	if got, want := copied.RGBAAt(1, 0), (color.RGBA{70, 60, 50, 255}); got != want {
		t.Errorf("Resample(BGRA, 1) = %v, want %v", got, want)
	}
}
