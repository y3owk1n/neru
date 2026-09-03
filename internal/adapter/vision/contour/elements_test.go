package contour_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
)

func TestElements_OffsetsByOriginAndDropsOutsideRegion(t *testing.T) {
	t.Parallel()

	origin := image.Pt(1920, 0)
	region := image.Rect(2000, 100, 2800, 700)
	rects := []image.Rectangle{
		image.Rect(100, 150, 200, 190), // inside once offset
		image.Rect(10, 10, 50, 30),     // outside region
	}

	got := contour.Elements(origin, region, rects)
	if len(got) != 1 {
		t.Fatalf("len(Elements) = %d, want 1", len(got))
	}

	want := image.Rect(2020, 150, 2120, 190)
	if got[0].Bounds() != want {
		t.Errorf("Bounds() = %v, want %v", got[0].Bounds(), want)
	}

	if !got[0].IsClickable() || !got[0].IsVisionOnly() {
		t.Error("element must be clickable and vision-only")
	}
}
