//go:build linux || windows

package vision

import (
	"image"
	"image/color"
	"testing"
)

// TestBandOf_MatchesACaptureOfTheBandAlone pins the comparison the watch
// makes. The band of the last read's whole capture has to equal a capture of
// that band by itself when nothing moved, at 1x and on a HiDPI display, or a
// region that never changed would never be seen as unchanged.
func TestBandOf_MatchesACaptureOfTheBandAlone(t *testing.T) {
	region := image.Rect(100, 50, 500, 450)
	band := probeBand(region)

	for _, scale := range []int{1, 2} {
		screen := func(r image.Rectangle) *image.RGBA {
			img := image.NewRGBA(image.Rect(0, 0, r.Dx()*scale, r.Dy()*scale))
			for y := range img.Rect.Dy() {
				for x := range img.Rect.Dx() {
					gx, gy := r.Min.X*scale+x, r.Min.Y*scale+y
					img.SetRGBA(
						x,
						y,
						color.RGBA{R: uint8(gx), G: uint8(gy), B: uint8(gx ^ gy), A: 0xFF},
					)
				}
			}

			return img
		}

		if !sameFrame(bandOf(screen(region), region, band), screen(band)) {
			t.Fatalf(
				"at %dx the band of the whole capture differs from a capture of the band",
				scale,
			)
		}
	}

	if band.Dy() >= region.Dy() || !band.In(region) {
		t.Fatalf("band %v is not a strip inside region %v", band, region)
	}
}
