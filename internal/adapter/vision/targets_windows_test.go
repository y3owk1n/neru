package vision

import (
	"image"
	"testing"
)

func TestScaleRects_RoundsOutward(t *testing.T) {
	t.Parallel()

	got := scaleRects([]image.Rectangle{image.Rect(3, 5, 11, 7)}, 1.5)
	if want := image.Rect(4, 7, 17, 11); got[0] != want {
		t.Errorf("scaleRects(1.5) = %v, want %v", got[0], want)
	}

	if in := []image.Rectangle{image.Rect(1, 2, 3, 4)}; &scaleRects(in, 1)[0] != &in[0] {
		t.Error("scaleRects(1) should return its input")
	}
}
